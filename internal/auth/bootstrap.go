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

// BootstrapFirstAdmin promotes u to sysadmin and creates a default "Personal"
// library (with u as its admin) the first time a matching user signs in. It
// is a no-op if any libraries already exist, so retrying is safe.
//
// The whole thing runs in one transaction: if any step fails, u's role is
// not promoted and no partial library is left behind.
func BootstrapFirstAdmin(ctx context.Context, db *gorm.DB, u *models.User, bootstrapEmails []string) error {
	if !emailInList(u.Email, bootstrapEmails) {
		return nil
	}

	var libCount int64
	if err := db.WithContext(ctx).Model(&models.Library{}).Count(&libCount).Error; err != nil {
		return fmt.Errorf("count libraries: %w", err)
	}
	if libCount > 0 {
		return nil
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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

	var count int64
	if err := db.WithContext(ctx).Model(&models.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	_, err := createLocalAdmin(ctx, db, email, password)
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

	var count int64
	if err := db.WithContext(ctx).Model(&models.User{}).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil, ErrAlreadyBootstrapped
	}

	return createLocalAdmin(ctx, db, email, password)
}

// createLocalAdmin is the shared transaction for seed/HTTP bootstrap.
// Callers are responsible for the "no users exist" precondition.
func createLocalAdmin(ctx context.Context, db *gorm.DB, email, password string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	hashStr := string(hash)

	u := models.User{
		Subject:      localSubject(email),
		Email:        email,
		DisplayName:  email,
		Role:         "user", // promoted by BootstrapFirstAdmin below
		PasswordHash: &hashStr,
	}
	if err := db.WithContext(ctx).Create(&u).Error; err != nil {
		return nil, fmt.Errorf("create admin user: %w", err)
	}

	if err := BootstrapFirstAdmin(ctx, db, &u, []string{email}); err != nil {
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
	if displayName == "" {
		displayName = email
	}
	subject := localSubject(email)

	var existing models.User
	err := db.WithContext(ctx).Where("subject = ? OR email = ?", subject, email).First(&existing).Error
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
		Subject:      subject,
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
// email or subject is already present.
var ErrUserExists = errors.New("user already exists")

func emailInList(email string, list []string) bool {
	for _, e := range list {
		if strings.EqualFold(strings.TrimSpace(e), email) {
			return true
		}
	}
	return false
}
