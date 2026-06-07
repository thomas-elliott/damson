package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

type OIDCConfig struct {
	IssuerURL string
	ClientID  string
	Audience  string
}

// OIDCAuthenticator validates IdP-issued bearer tokens and resolves them to a
// damson user via the user_identities table. The very first user to sign in on
// an empty system triggers the one-time bootstrap (see bootstrap.go).
type OIDCAuthenticator struct {
	verifier *oidc.IDTokenVerifier
	db       *gorm.DB
}

func NewOIDCAuthenticator(ctx context.Context, db *gorm.DB, cfg OIDCConfig) (*OIDCAuthenticator, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	audience := cfg.Audience
	if audience == "" {
		audience = cfg.ClientID
	}
	return &OIDCAuthenticator{
		verifier: provider.Verifier(&oidc.Config{ClientID: audience}),
		db:       db,
	}, nil
}

func (a *OIDCAuthenticator) Authenticate(ctx context.Context, rawToken string) (*models.User, error) {
	tok, err := a.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	// Issuer and Subject are validated struct fields on the verified token;
	// email/name come from the raw claims.
	if tok.Issuer == "" || tok.Subject == "" {
		return nil, ErrUnauthenticated
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := tok.Claims(&claims); err != nil {
		return nil, ErrUnauthenticated
	}

	return a.resolveUser(ctx, tok.Issuer, tok.Subject, claims.Email, claims.EmailVerified, claims.Name)
}

// resolveUser maps a verified IdP identity to a damson user, in priority order:
//
//  1. an existing (issuer, subject) identity → its user;
//  2. else, when the IdP asserts a *verified* email, an existing user with that
//     email → link a new identity to them (this is what lets someone who first
//     signed up via one IdP log in via another, same email, same account);
//  3. else, a brand-new user seeded from the claims, with this identity.
//
// Email/name are only ever used to seed a new profile or as the join key — once
// a user exists the app owns their email/display name (edited via PATCH
// /api/me), so later logins never overwrite them. The whole thing runs in one
// transaction; a freshly-created first user triggers bootstrap inside it.
func (a *OIDCAuthenticator) resolveUser(ctx context.Context, issuer, subject, email string, emailVerified bool, displayName string) (*models.User, error) {
	var u models.User
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Known identity.
		var ident models.UserIdentity
		err := tx.Where("issuer = ? AND subject = ?", issuer, subject).First(&ident).Error
		if err == nil {
			return tx.First(&u, ident.UserID).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 2. Adopt an existing account by verified email, linking this identity.
		if emailVerified && email != "" {
			err := tx.Where("lower(email) = lower(?)", email).First(&u).Error
			if err == nil {
				return tx.Create(&models.UserIdentity{
					UserID: u.ID, Issuer: issuer, Subject: subject,
				}).Error
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		// 3. New user. Leave email blank when the IdP doesn't send a verified
		// one — the SPA detects the blank email (needs_profile) and prompts for
		// it. Only seed email/display name from a verified address.
		seedEmail := ""
		if emailVerified {
			seedEmail = email
		}
		if displayName == "" {
			displayName = seedEmail
		}
		u = models.User{Email: seedEmail, DisplayName: displayName, Role: "user"}
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.UserIdentity{
			UserID: u.ID, Issuer: issuer, Subject: subject,
		}).Error; err != nil {
			return err
		}
		return BootstrapFirstAdmin(ctx, tx, &u)
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}
