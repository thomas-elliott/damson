package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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

// SeedLocalAdmin creates the initial local-auth admin user if no users exist
// yet. Runs at startup in local auth mode. The email/password come from
// DAMSON_LOCAL_ADMIN_EMAIL / DAMSON_LOCAL_ADMIN_PASSWORD.
//
// The resulting user is immediately put through BootstrapFirstAdmin to set
// up the default library. Re-running with different env values after a user
// has been seeded does NOT rotate the password — that's an admin action.
func SeedLocalAdmin(ctx context.Context, db *gorm.DB, email, password string) error {
	if email == "" || password == "" {
		return errors.New("DAMSON_LOCAL_ADMIN_EMAIL and DAMSON_LOCAL_ADMIN_PASSWORD are required in local auth mode")
	}

	var count int64
	if err := db.WithContext(ctx).Model(&models.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
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
		return fmt.Errorf("create admin user: %w", err)
	}

	return BootstrapFirstAdmin(ctx, db, &u, []string{email})
}

func emailInList(email string, list []string) bool {
	for _, e := range list {
		if strings.EqualFold(strings.TrimSpace(e), email) {
			return true
		}
	}
	return false
}
