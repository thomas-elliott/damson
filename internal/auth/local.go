package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// Token TTL. Short enough that a stolen token has a narrow window, long
// enough that typical user sessions don't need to refresh constantly.
const LocalTokenTTL = time.Hour

// defaultJWTIssuer is the fallback iss/aud value when NewLocalAuthenticator
// is called without an explicit issuer. Per-deployment callers should pass a
// stable identifier (e.g. DAMSON_BASE_URL) so a token signed for one Damson
// can't be replayed against another that happens to share a key.
const defaultJWTIssuer = "damson-local"

// LocalAuthenticator issues and verifies server-signed JWTs for users whose
// credentials live in the database. All users carry a PasswordHash and
// optionally a TokensValidAfter watermark that revokes any token issued
// before that instant (used on password change or admin deactivation).
type LocalAuthenticator struct {
	db         *gorm.DB
	signingKey []byte
	issuer     string           // iss + aud claim value
	now        func() time.Time // injectable for tests
}

func NewLocalAuthenticator(db *gorm.DB, signingKey []byte) (*LocalAuthenticator, error) {
	return NewLocalAuthenticatorWithIssuer(db, signingKey, defaultJWTIssuer)
}

// NewLocalAuthenticatorWithIssuer is the explicit-issuer variant. The empty
// string falls back to defaultJWTIssuer so a missing-config call doesn't
// silently produce unverifiable tokens.
func NewLocalAuthenticatorWithIssuer(db *gorm.DB, signingKey []byte, issuer string) (*LocalAuthenticator, error) {
	if len(signingKey) < 32 {
		return nil, errors.New("DAMSON_JWT_SIGNING_KEY must be at least 32 bytes")
	}
	if issuer == "" {
		issuer = defaultJWTIssuer
	}
	return &LocalAuthenticator{
		db:         db,
		signingKey: signingKey,
		issuer:     issuer,
		now:        time.Now,
	}, nil
}

func (a *LocalAuthenticator) Authenticate(ctx context.Context, rawToken string) (*models.User, error) {
	claims := &jwt.RegisteredClaims{}
	tok, err := jwt.ParseWithClaims(rawToken, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.signingKey, nil
	}, jwt.WithIssuer(a.issuer), jwt.WithAudience(a.issuer), jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		return nil, ErrUnauthenticated
	}

	userID, err := parseUserID(claims.Subject)
	if err != nil {
		return nil, ErrUnauthenticated
	}

	var u models.User
	if err := a.db.WithContext(ctx).First(&u, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, err
	}

	// Honour the revocation watermark: tokens issued before it are invalid.
	if u.TokensValidAfter != nil && claims.IssuedAt != nil &&
		claims.IssuedAt.Time.Before(*u.TokensValidAfter) {
		return nil, ErrUnauthenticated
	}

	return &u, nil
}

// Login verifies the email+password combination and, on success, returns a
// freshly-signed JWT and its expiry. Generic "invalid credentials" is
// returned for any failure so enumeration attacks can't distinguish wrong
// password from missing user.
func (a *LocalAuthenticator) Login(ctx context.Context, email, password string) (token string, expiresAt time.Time, _ error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return "", time.Time{}, ErrUnauthenticated
	}

	var u models.User
	err := a.db.WithContext(ctx).
		Where("lower(email) = lower(?) AND password_hash IS NOT NULL", email).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Still hash something to keep timing roughly constant.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$00000000000000000000000000000000000000000000000000000"), []byte(password))
		return "", time.Time{}, ErrUnauthenticated
	}
	if err != nil {
		return "", time.Time{}, err
	}
	if u.PasswordHash == nil {
		return "", time.Time{}, ErrUnauthenticated
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password)); err != nil {
		return "", time.Time{}, ErrUnauthenticated
	}

	return a.issueToken(&u)
}

// Refresh issues a new short-lived JWT for an already-authenticated user.
// Callers are expected to have validated the bearer via Authenticate (i.e.
// the request reached an authed handler) — this is what lets the SPA
// extend a session without re-prompting for a password while still
// honouring the tokens_valid_after revocation watermark.
//
// The new token's iat is `now`, so a freshly-issued refresh stays valid
// even if tokens_valid_after has just been bumped (admin password reset,
// membership change, etc.). The old token remains usable until it expires
// naturally; for v1 we don't track per-token revocation.
func (a *LocalAuthenticator) Refresh(u *models.User) (string, time.Time, error) {
	return a.issueToken(u)
}

func (a *LocalAuthenticator) issueToken(u *models.User) (string, time.Time, error) {
	now := a.now()
	exp := now.Add(LocalTokenTTL)
	claims := jwt.RegisteredClaims{
		Issuer:    a.issuer,
		Audience:  jwt.ClaimStrings{a.issuer},
		Subject:   fmt.Sprintf("%d", u.ID),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(a.signingKey)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func parseUserID(s string) (uint, error) {
	var id uint
	_, err := fmt.Sscanf(s, "%d", &id)
	if err != nil || id == 0 {
		return 0, errors.New("invalid subject")
	}
	return id, nil
}
