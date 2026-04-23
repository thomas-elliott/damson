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
	IssuerURL       string
	ClientID        string
	Audience        string
	BootstrapAdmins []string // emails promoted to sysadmin on first sign-in
}

// OIDCAuthenticator validates IdP-issued bearer tokens and upserts users
// from claims. The first user matching BootstrapAdmins triggers the one-time
// bootstrap (see bootstrap.go).
type OIDCAuthenticator struct {
	verifier        *oidc.IDTokenVerifier
	db              *gorm.DB
	bootstrapAdmins []string
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
		verifier:        provider.Verifier(&oidc.Config{ClientID: audience}),
		db:              db,
		bootstrapAdmins: cfg.BootstrapAdmins,
	}, nil
}

func (a *OIDCAuthenticator) Authenticate(ctx context.Context, rawToken string) (*models.User, error) {
	tok, err := a.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, ErrUnauthenticated
	}

	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := tok.Claims(&claims); err != nil || claims.Sub == "" {
		return nil, ErrUnauthenticated
	}

	return a.upsertUser(ctx, claims.Sub, claims.Email, claims.Name)
}

// upsertUser finds a user by OIDC subject or creates one, keeping email/name
// in sync with the identity provider. If this is a freshly-created user and
// their email matches the bootstrap list, the first-admin bootstrap runs
// inside the same transaction (see bootstrap.go).
func (a *OIDCAuthenticator) upsertUser(ctx context.Context, subject, email, displayName string) (*models.User, error) {
	var u models.User
	err := a.db.WithContext(ctx).Where("subject = ?", subject).First(&u).Error
	if err == nil {
		if u.Email != email || (displayName != "" && u.DisplayName != displayName) {
			u.Email = email
			if displayName != "" {
				u.DisplayName = displayName
			}
			if err := a.db.WithContext(ctx).Save(&u).Error; err != nil {
				return nil, err
			}
		}
		return &u, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if displayName == "" {
		displayName = email
	}
	subj := subject
	u = models.User{
		Subject:     subj,
		Email:       email,
		DisplayName: displayName,
		Role:        "user",
	}
	if err := a.db.WithContext(ctx).Create(&u).Error; err != nil {
		return nil, err
	}

	if err := BootstrapFirstAdmin(ctx, a.db, &u, a.bootstrapAdmins); err != nil {
		return nil, err
	}
	return &u, nil
}
