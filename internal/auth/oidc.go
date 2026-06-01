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

// OIDCAuthenticator validates IdP-issued bearer tokens and upserts users by
// their OIDC subject. The very first user to sign in on an empty system
// triggers the one-time bootstrap (see bootstrap.go).
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

// upsertUser finds a user by OIDC subject or creates one. The subject is the
// only thing we trust from the IdP; email and name are optional claims used
// merely to seed the profile at creation. Once a user exists the app owns
// their email/display name (edited via PATCH /api/me), so we never overwrite
// them from later logins. A freshly-created first user triggers the one-time
// bootstrap inside the same call (see bootstrap.go).
func (a *OIDCAuthenticator) upsertUser(ctx context.Context, subject, email, displayName string) (*models.User, error) {
	var u models.User
	err := a.db.WithContext(ctx).Where("subject = ?", subject).First(&u).Error
	if err == nil {
		return &u, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Seed the profile from claims when present. Leave email empty when the
	// IdP doesn't send one — the SPA detects the blank email (needs_profile)
	// and prompts the user to fill it in. Display name defaults to email only
	// when we actually have an email to fall back to.
	if displayName == "" {
		displayName = email
	}
	u = models.User{
		Subject:     subject,
		Email:       email,
		DisplayName: displayName,
		Role:        "user",
	}
	if err := a.db.WithContext(ctx).Create(&u).Error; err != nil {
		return nil, err
	}

	if err := BootstrapFirstAdmin(ctx, a.db, &u); err != nil {
		return nil, err
	}
	return &u, nil
}
