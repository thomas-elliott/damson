package auth

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"gorm.io/gorm"

	"github.com/thomas-elliott/damson/internal/models"
)

// raceFirstRun starts n first-run sign-ups at once on an empty system and
// returns each one's error.
func raceFirstRun(n int, signUp func(i int) error) []error {
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = signUp(i)
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

// requireOneBootstrap checks the outcome every first-run race must end in:
// exactly one sysadmin and one Personal library, the sysadmin its admin.
func requireOneBootstrap(t *testing.T, db *gorm.DB) {
	t.Helper()
	var admins []models.User
	if err := db.Where("role = ?", "sysadmin").Find(&admins).Error; err != nil {
		t.Fatalf("load sysadmins: %v", err)
	}
	var libs []models.Library
	if err := db.Find(&libs).Error; err != nil {
		t.Fatalf("load libraries: %v", err)
	}
	if len(admins) != 1 || len(libs) != 1 {
		t.Fatalf("got %d sysadmins and %d libraries, want 1 and 1", len(admins), len(libs))
	}
	var mem models.LibraryMembership
	if err := db.Where("library_id = ? AND user_id = ? AND role = ?", libs[0].ID, admins[0].ID, "admin").First(&mem).Error; err != nil {
		t.Fatalf("the sysadmin does not administer the Personal library: %v", err)
	}
}

func TestCreateFirstLocalAdmin_ConcurrentSignUpsBootstrapOnce(t *testing.T) {
	db := newBootstrapTestDB(t)

	errs := raceFirstRun(4, func(i int) error {
		_, err := CreateFirstLocalAdmin(t.Context(), db, fmt.Sprintf("admin%d@example.com", i), "password1")
		return err
	})

	var won, lost int
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, ErrAlreadyBootstrapped):
			lost++
		default:
			t.Fatalf("sign-up failed: %v", err)
		}
	}
	if won != 1 || lost != len(errs)-1 {
		t.Fatalf("%d sign-ups won and %d got ErrAlreadyBootstrapped, want 1 and %d", won, lost, len(errs)-1)
	}
	var users int64
	if err := db.Model(&models.User{}).Count(&users).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if users != 1 {
		t.Fatalf("%d users exist; a losing sign-up left its row behind", users)
	}
	requireOneBootstrap(t, db)
}

func TestResolveUser_ConcurrentFirstLoginsBootstrapOnce(t *testing.T) {
	db := newBootstrapTestDB(t)
	a := &OIDCAuthenticator{db: db}

	errs := raceFirstRun(4, func(i int) error {
		_, err := a.resolveUser(t.Context(), "https://idp.example", fmt.Sprintf("sub-%d", i),
			fmt.Sprintf("user%d@example.com", i), true, "")
		return err
	})
	for _, err := range errs {
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}
	}
	// Every login still becomes a user; only one of them is the sysadmin.
	var users int64
	if err := db.Model(&models.User{}).Count(&users).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if users != int64(len(errs)) {
		t.Fatalf("%d users, want %d", users, len(errs))
	}
	requireOneBootstrap(t, db)
}
