package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// bootstrapLockKey serialises first-run bootstrap across requests and pods.
// Distinct from db.AdvisoryLockKey, which serialises migrations.
const bootstrapLockKey int64 = 4242424243

// lockBootstrap holds bootstrapLockKey until tx ends. Counting users after it
// is what makes "am I first?" safe: under read committed, the count then sees
// every bootstrap that committed while this one waited.
func lockBootstrap(tx *gorm.DB) error {
	if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", bootstrapLockKey).Error; err != nil {
		return fmt.Errorf("bootstrap lock: %w", err)
	}
	return nil
}

// BootstrapFirstAdmin promotes u to sysadmin and creates a default "Personal"
// library (with u as its admin) when u is the very first user on an empty
// system. Identity is whoever signs in first — we no longer consult an env
// var of privileged emails. It is a no-op once any other user exists or any
// library has been created, so retrying is safe.
//
// The whole thing runs in one transaction under bootstrapLockKey: if any step
// fails, u's role is not promoted and no partial library is left behind, and
// two first sign-ups racing each other cannot both be promoted.
func BootstrapFirstAdmin(ctx context.Context, db *gorm.DB, u *models.User) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockBootstrap(tx); err != nil {
			return err
		}
		// Only the first account on an empty system bootstraps. u has already
		// been created by the caller, so it counts itself — exactly one user
		// means u is the first. The library check is belt-and-braces against a
		// half-finished previous run.
		var userCount int64
		if err := tx.Model(&models.User{}).Count(&userCount).Error; err != nil {
			return fmt.Errorf("count users: %w", err)
		}
		var libCount int64
		if err := tx.Model(&models.Library{}).Count(&libCount).Error; err != nil {
			return fmt.Errorf("count libraries: %w", err)
		}
		if userCount > 1 || libCount > 0 {
			return nil
		}

		u.Role = "sysadmin"
		if err := tx.Save(u).Error; err != nil {
			return fmt.Errorf("promote to sysadmin: %w", err)
		}

		lib := models.Library{
			Name:        "Personal",
			Description: "Default library created at first-run bootstrap",
		}
		if err := tx.Create(&lib).Error; err != nil {
			return fmt.Errorf("create library: %w", err)
		}

		mem := models.LibraryMembership{
			LibraryID: lib.ID,
			UserID:    u.ID,
			Role:      "admin",
		}
		if err := tx.Create(&mem).Error; err != nil {
			return fmt.Errorf("create membership: %w", err)
		}

		u.CurrentLibraryID = &lib.ID
		if err := tx.Save(u).Error; err != nil {
			return fmt.Errorf("set current library: %w", err)
		}

		slog.Info("bootstrapped first admin",
			"user_id", u.ID,
			"email", u.Email,
			"library_id", lib.ID)
		return nil
	})
}

// ErrAlreadyBootstrapped is returned by CreateFirstLocalAdmin when a user
// already exists. Callers map it to 409 Conflict at the HTTP boundary so
// that the "first user" endpoint is safe to leave registered permanently
// — it simply stops succeeding once bootstrap is done.
var ErrAlreadyBootstrapped = errors.New("already bootstrapped")

// MinPasswordLength is the minimum length we accept for a local-auth
// password. Local auth is a development/testing-grade credential — see
// docs/archive/technical-design.md §8.1; OIDC remains the recommended path for any
// deployment where account security matters. Eight characters reflects that
// floor without making the bootstrap flow unusable.
const MinPasswordLength = 8

// ErrPasswordTooShort is returned by the password-write paths when the
// supplied password is below MinPasswordLength.
var ErrPasswordTooShort = errors.New("password is too short")

// validatePassword enforces the project-wide password floor. Callers should
// surface ErrPasswordTooShort as a 400 with err.Error() per the convention.
func validatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// SeedLocalAdmin creates the initial local-auth admin user if no users exist
// yet. Runs at startup in local auth mode when DAMSON_LOCAL_ADMIN_EMAIL /
// DAMSON_LOCAL_ADMIN_PASSWORD are both set. Missing credentials is not an
// error — the server boots anyway and the first browser hit lands on the
// in-app bootstrap page via CreateFirstLocalAdmin.
//
// The resulting user is immediately put through BootstrapFirstAdmin to set
// up the default library. Re-running with different env values after a user
// has been seeded does NOT rotate the password — that's an admin action.
func SeedLocalAdmin(ctx context.Context, db *gorm.DB, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	if err := validatePassword(password); err != nil {
		return fmt.Errorf("seed local admin: %w", err)
	}

	_, err := createLocalAdmin(ctx, db, email, password)
	if errors.Is(err, ErrAlreadyBootstrapped) {
		return nil
	}
	return err
}

// CreateFirstLocalAdmin creates the very first user of a local-auth
// deployment when no users exist yet. Returns ErrAlreadyBootstrapped if
// any user record is present. Intended for HTTP-driven bootstrap when
// ops haven't pre-seeded via env vars.
func CreateFirstLocalAdmin(ctx context.Context, db *gorm.DB, email, password string) (*models.User, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	return createLocalAdmin(ctx, db, email, password)
}

// createLocalAdmin is the shared transaction for seed/HTTP bootstrap. It
// returns ErrAlreadyBootstrapped, leaving no user behind, when any user
// exists. The check and the insert share one transaction under
// bootstrapLockKey, so of two concurrent callers exactly one creates a user.
func createLocalAdmin(ctx context.Context, db *gorm.DB, email, password string) (*models.User, error) {
	// A cheap check first, so a server that is already set up answers
	// without paying for a password hash. The locked check below decides.
	var existing int64
	if err := db.WithContext(ctx).Model(&models.User{}).Count(&existing).Error; err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if existing > 0 {
		return nil, ErrAlreadyBootstrapped
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	hashStr := string(hash)

	u := models.User{
		Email:        email,
		DisplayName:  email,
		Role:         "user", // promoted by BootstrapFirstAdmin below
		PasswordHash: &hashStr,
	}
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockBootstrap(tx); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.User{}).Count(&count).Error; err != nil {
			return fmt.Errorf("count users: %w", err)
		}
		if count > 0 {
			return ErrAlreadyBootstrapped
		}
		if err := tx.Create(&u).Error; err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
		return BootstrapFirstAdmin(ctx, tx, &u)
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateLocalUser creates a local-auth user with a pre-set password.
// Returns ErrUserExists when the email is taken. Intended for admin-driven
// member invites (see internal/handlers/api/members.go) — distinct from
// CreateFirstLocalAdmin in that it does NOT bootstrap a library or promote
// the user to sysadmin.
//
// Display name defaults to email when empty.
func CreateLocalUser(ctx context.Context, db *gorm.DB, email, displayName, password string) (*models.User, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = email
	}

	var existing models.User
	err := db.WithContext(ctx).Where("lower(email) = lower(?)", email).First(&existing).Error
	if err == nil {
		return &existing, ErrUserExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	hashStr := string(hash)
	u := models.User{
		Email:        email,
		DisplayName:  displayName,
		Role:         "user",
		PasswordHash: &hashStr,
	}
	if err := db.WithContext(ctx).Create(&u).Error; err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

// SetLocalUserPassword rotates an existing user's password and bumps
// tokens_valid_after so existing JWTs are revoked. Used by admins from
// the members editor when a user needs a credential reset.
func SetLocalUserPassword(ctx context.Context, db *gorm.DB, userID uint, password string) error {
	if password == "" {
		return errors.New("password is required")
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	hashStr := string(hash)
	now := time.Now().UTC()
	return db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"password_hash":      hashStr,
			"tokens_valid_after": &now,
		}).Error
}

// ErrUserExists is returned by CreateLocalUser when a user with the same
// email is already present.
var ErrUserExists = errors.New("user already exists")
