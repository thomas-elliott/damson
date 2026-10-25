package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/models"
)

type tokenPayload struct {
	ID          uint       `json:"id"`
	Label       string     `json:"label"`
	Prefix      string     `json:"prefix"`
	Scope       string     `json:"scope"`
	LibraryID   uint       `json:"library_id"`
	LibraryName string     `json:"library_name"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
	Token       string     `json:"token"`
}

type tokenListPayload struct {
	Tokens []tokenPayload `json:"tokens"`
}

func TestPostAPIToken_ReturnsTheSecretOnceAndStoresOnlyItsDigest(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "owner@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{
		"label": "laptop sync",
		"scope": "write",
	})
	requireStatus(t, w, http.StatusCreated)
	created := jsonResponse[tokenPayload](t, w)

	if created.Token == "" {
		t.Fatal("create must return the secret")
	}
	if !strings.HasPrefix(created.Token, auth.TokenPrefix) {
		t.Fatalf("secret %q lacks the damson_pat_ prefix", created.Token)
	}
	if created.Scope != auth.ScopeWrite {
		t.Fatalf("scope: got %q", created.Scope)
	}
	if created.LibraryID != lib.ID || created.LibraryName != "Main" {
		t.Fatalf("library: got %d/%q want %d/Main", created.LibraryID, created.LibraryName, lib.ID)
	}
	if !strings.HasPrefix(created.Token, created.Prefix) || created.Prefix == created.Token {
		t.Fatalf("display prefix %q is not a strict prefix of the secret", created.Prefix)
	}

	// Only the digest is persisted; the plaintext must appear nowhere.
	var row models.APIToken
	if err := db.Where("id = ?", created.ID).Take(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.TokenHash != auth.HashToken(created.Token) {
		t.Fatal("stored digest does not match the issued secret")
	}
	if strings.Contains(row.TokenHash, strings.TrimPrefix(created.Token, auth.TokenPrefix)) {
		t.Fatal("stored digest embeds the secret")
	}

	// And the secret is never recoverable through the list endpoint.
	lw := performGET(r, "/api/me/tokens")
	requireStatus(t, lw, http.StatusOK)
	if strings.Contains(lw.Body.String(), created.Token) {
		t.Fatal("list response leaks the secret")
	}
	list := jsonResponse[tokenListPayload](t, lw)
	if len(list.Tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(list.Tokens))
	}
	if list.Tokens[0].Token != "" {
		t.Fatal("list entries must carry no secret")
	}
	if list.Tokens[0].LibraryName != "Main" {
		t.Fatalf("library name not joined: %q", list.Tokens[0].LibraryName)
	}
}

func TestPostAPIToken_Validation(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "valid@example.com")
	seedLibraryForUser(t, db, u, "Main")
	r := setupRouter(h, u)

	t.Run("label required", func(t *testing.T) {
		w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "   "})
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("label bounded", func(t *testing.T) {
		w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{
			"label": strings.Repeat("x", maxTokenLabelLen+1),
		})
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("unknown scope rejected", func(t *testing.T) {
		w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{
			"label": "bad scope", "scope": "admin",
		})
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("expiry must be in the future", func(t *testing.T) {
		w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{
			"label":      "stale",
			"expires_at": time.Now().UTC().Add(-time.Hour),
		})
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("scope defaults to write", func(t *testing.T) {
		w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "defaulted"})
		requireStatus(t, w, http.StatusCreated)
		if got := jsonResponse[tokenPayload](t, w).Scope; got != auth.ScopeWrite {
			t.Fatalf("scope: got %q want %q", got, auth.ScopeWrite)
		}
	})
}

func TestPostAPIToken_RequiresACurrentLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "nolib@example.com")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "orphan"})
	requireStatus(t, w, http.StatusForbidden)

	var count int64
	db.Model(&models.APIToken{}).Count(&count)
	if count != 0 {
		t.Fatalf("no token should have been minted, found %d", count)
	}
}

// A stale current_library_id — the user was removed from the library — must
// not mint a token that would re-grant access to it.
func TestPostAPIToken_RejectsAStaleCurrentLibrary(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "stale@example.com")
	lib := seedLibraryForUser(t, db, u, "Main")
	if err := db.Where("library_id = ? AND user_id = ?", lib.ID, u.ID).
		Delete(&models.LibraryMembership{}).Error; err != nil {
		t.Fatalf("drop membership: %v", err)
	}
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "ghost"})
	requireStatus(t, w, http.StatusForbidden)
}

func TestPostAPIToken_CapsActiveTokens(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "many@example.com")
	seedLibraryForUser(t, db, u, "Main")
	r := setupRouter(h, u)

	var lastID uint
	for i := 0; i < maxActiveTokensPerUser; i++ {
		w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "t"})
		requireStatus(t, w, http.StatusCreated)
		lastID = jsonResponse[tokenPayload](t, w).ID
	}
	w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "one too many"})
	requireStatus(t, w, http.StatusConflict)

	// Revoking frees a slot — the cap counts active tokens, not rows.
	requireStatus(t, performRequest(r, http.MethodDelete, tokenPath(lastID), nil), http.StatusNoContent)
	requireStatus(t, performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "after revoke"}), http.StatusCreated)
}

func TestDeleteAPIToken_RevokesAndIsIdempotent(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "revoke@example.com")
	seedLibraryForUser(t, db, u, "Main")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "doomed"})
	requireStatus(t, w, http.StatusCreated)
	created := jsonResponse[tokenPayload](t, w)

	requireStatus(t, performRequest(r, http.MethodDelete, tokenPath(created.ID), nil), http.StatusNoContent)
	requireStatus(t, performRequest(r, http.MethodDelete, tokenPath(created.ID), nil), http.StatusNoContent)

	// The row survives revocation so the audit trail keeps a target.
	var row models.APIToken
	if err := db.Where("id = ?", created.ID).Take(&row).Error; err != nil {
		t.Fatalf("row should still exist: %v", err)
	}
	if row.RevokedAt == nil {
		t.Fatal("revoked_at not set")
	}

	// And the secret no longer authenticates.
	if _, err := auth.NewTokenAuthenticator(db).Authenticate(t.Context(), created.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("revoked secret still authenticates: %v", err)
	}

	requireStatus(t, performRequest(r, http.MethodDelete, tokenPath(created.ID+9999), nil), http.StatusNotFound)
}

// Tokens are per-user even inside a shared library: another member must not
// see or revoke them.
func TestAPITokens_AreIsolatedBetweenUsers(t *testing.T) {
	h, db := newTestHandler(t)
	alice := seedUser(t, db, "alice@example.com")
	lib := seedLibraryForUser(t, db, alice, "Shared")
	bob := seedUser(t, db, "bob@example.com")
	seedMembership(t, db, lib.ID, bob.ID, "admin")
	bob.CurrentLibraryID = &lib.ID
	if err := db.Save(bob).Error; err != nil {
		t.Fatalf("set bob's library: %v", err)
	}

	aliceR := setupRouter(h, alice)
	w := performRequest(aliceR, http.MethodPost, "/api/me/tokens", map[string]any{"label": "alice only"})
	requireStatus(t, w, http.StatusCreated)
	aliceToken := jsonResponse[tokenPayload](t, w)

	bobR := setupRouter(h, bob)
	list := jsonResponse[tokenListPayload](t, performGET(bobR, "/api/me/tokens"))
	if len(list.Tokens) != 0 {
		t.Fatalf("bob sees %d of alice's tokens", len(list.Tokens))
	}

	requireStatus(t, performRequest(bobR, http.MethodDelete, tokenPath(aliceToken.ID), nil), http.StatusNotFound)

	var row models.APIToken
	if err := db.Where("id = ?", aliceToken.ID).Take(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.RevokedAt != nil {
		t.Fatal("bob revoked alice's token")
	}
}

func TestAPITokens_Audited(t *testing.T) {
	h, db := newTestHandler(t)
	u := seedUser(t, db, "audit@example.com")
	seedLibraryForUser(t, db, u, "Main")
	r := setupRouter(h, u)

	w := performRequest(r, http.MethodPost, "/api/me/tokens", map[string]any{"label": "audited"})
	requireStatus(t, w, http.StatusCreated)
	created := jsonResponse[tokenPayload](t, w)
	requireStatus(t, performRequest(r, http.MethodDelete, tokenPath(created.ID), nil), http.StatusNoContent)

	for _, action := range []string{"token.create", "token.revoke"} {
		var entry models.AuditLog
		err := db.Where("action_type = ? AND target_type = ? AND target_id = ?", action, "api_token", created.ID).
			Take(&entry).Error
		if err != nil {
			t.Fatalf("%s audit row missing: %v", action, err)
		}
		if strings.Contains(string(entry.DiffJSON), strings.TrimPrefix(created.Token, auth.TokenPrefix)) {
			t.Fatalf("%s audit diff leaks the secret", action)
		}
	}
}

func tokenPath(id uint) string {
	return "/api/me/tokens/" + strconv.FormatUint(uint64(id), 10)
}
