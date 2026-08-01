package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

const (
	// TokenPrefix marks a bearer as a personal access token rather than a
	// JWT or an IdP token.
	TokenPrefix = "damson_pat_"

	ScopeRead  = "read"
	ScopeWrite = "write"

	tokenSecretBytes  = 32
	tokenDisplayChars = 8

	lastUsedResolution = time.Minute
)

// ErrTokenScope signals that a valid token lacks the scope for the attempted
// operation. Distinct from ErrUnauthenticated: the caller is known, so this
// maps to 403 rather than 401.
var ErrTokenScope = errors.New("token scope insufficient")

// Principal is the result of authenticating a personal access token: the
// owning user with CurrentLibraryID already pinned to the token's library,
// plus the token row itself.
type Principal struct {
	User  *models.User
	Token *models.APIToken
}

// IsTokenBearer reports whether raw looks like a personal access token. Used
// by the middleware to pick a branch before doing any work.
func IsTokenBearer(raw string) bool {
	return strings.HasPrefix(raw, TokenPrefix)
}

// GenerateToken mints a new secret and returns it alongside the values that
// get persisted. The secret is returned exactly once; only hash and prefix
// are ever stored.
func GenerateToken() (secret, hash, prefix string, err error) {
	buf := make([]byte, tokenSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("generate token: %w", err)
	}
	secret = TokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	hash = HashToken(secret)
	body := strings.TrimPrefix(secret, TokenPrefix)
	if len(body) > tokenDisplayChars {
		body = body[:tokenDisplayChars]
	}
	return secret, hash, TokenPrefix + body, nil
}

// HashToken returns the lookup digest for a raw secret. SHA-256 rather than
// bcrypt: the secret is 32 random bytes, so a work factor adds nothing, and a
// fast digest is what allows the indexed lookup on every request.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// NormaliseScope maps user input onto a valid scope, defaulting to write.
func NormaliseScope(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", ScopeWrite:
		return ScopeWrite, nil
	case ScopeRead:
		return ScopeRead, nil
	default:
		return "", fmt.Errorf("scope must be %q or %q", ScopeRead, ScopeWrite)
	}
}

// TokenAuthenticator resolves personal access tokens to their owning user.
// It sits alongside the mode-specific Authenticator rather than replacing it.
type TokenAuthenticator struct {
	db  *gorm.DB
	now func() time.Time
}

func NewTokenAuthenticator(db *gorm.DB) *TokenAuthenticator {
	return &TokenAuthenticator{db: db, now: time.Now}
}

// Authenticate verifies raw and returns the principal it represents. Every
// failure — unknown, revoked, expired, orphaned user — surfaces as
// ErrUnauthenticated.
func (t *TokenAuthenticator) Authenticate(ctx context.Context, raw string) (*Principal, error) {
	if !IsTokenBearer(raw) {
		return nil, ErrUnauthenticated
	}

	var tok models.APIToken
	err := t.db.WithContext(ctx).Where("token_hash = ?", HashToken(raw)).Take(&tok).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}

	now := t.now().UTC()
	if tok.RevokedAt != nil {
		return nil, ErrUnauthenticated
	}
	if tok.ExpiresAt != nil && !tok.ExpiresAt.After(now) {
		return nil, ErrUnauthenticated
	}

	var user models.User
	err = t.db.WithContext(ctx).Where("id = ?", tok.UserID).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}

	var count int64
	if err := t.db.WithContext(ctx).
		Model(&models.LibraryMembership{}).
		Where("library_id = ? AND user_id = ?", tok.LibraryID, tok.UserID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, ErrUnauthenticated
	}

	libraryID := tok.LibraryID
	user.CurrentLibraryID = &libraryID

	t.touch(ctx, &tok, now)

	return &Principal{User: &user, Token: &tok}, nil
}

// touch records last_used_at, at most once per lastUsedResolution. Errors are
// swallowed by design.
func (t *TokenAuthenticator) touch(ctx context.Context, tok *models.APIToken, now time.Time) {
	if tok.LastUsedAt != nil && now.Sub(*tok.LastUsedAt) < lastUsedResolution {
		return
	}
	_ = t.db.WithContext(ctx).
		Model(&models.APIToken{}).
		Where("id = ?", tok.ID).
		Update("last_used_at", now).Error
	tok.LastUsedAt = &now
}

// CheckScope reports whether a token may perform an HTTP method. Read tokens
// are confined to GET.
func CheckScope(scope, method string) error {
	if scope == ScopeRead && method != "GET" && method != "HEAD" {
		return ErrTokenScope
	}
	return nil
}

// TokenPathDenied reports whether a route is off-limits to token auth: token
// management and sysadmin surface.
func TokenPathDenied(routePath string) bool {
	return strings.Contains(routePath, "/me/tokens") ||
		strings.Contains(routePath, "/admin/")
}
