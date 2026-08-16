package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/rules"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/filebrowser/filebrowser/v2/users"
)

// forgeSecurityToken builds a valid HS256 auth token for the given user ID,
// mirroring what withUser expects.
func forgeSecurityToken(t *testing.T, key []byte, id uint, perm users.Permissions) string {
	t.Helper()
	claims := &authToken{
		User: userInfo{ID: id, Username: "u", Perm: perm},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

// newSecurityStorage builds a storage with one user rooted at a ScopedFs over
// the given scope.
func newSecurityStorage(t *testing.T, key []byte, scope string, perms users.Permissions, commands []string, userRules []rules.Rule) *storage.Storage {
	t.Helper()
	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatalf("failed to get storage: %v", err)
	}
	if err := st.Users.Save(&users.User{
		Username: "u",
		Password: "pw",
		Perm:     perms,
		Commands: commands,
		Rules:    userRules,
	}); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}
	if err := st.Settings.Save(&settings.Settings{Key: key}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}
	st.Users = &customFSUser{
		Store: st.Users,
		fs:    files.NewScopedFs(afero.NewOsFs(), scope),
	}
	return st
}

// TestCommandWorkingDirConfinedToScope verifies that the websocket command
// runner refuses to execute with a working directory outside the user's scope:
// through an escaping symlink, or in a directory denied by an exact-match
// rule. A normal in-scope directory must keep working.
func TestCommandWorkingDirConfinedToScope(t *testing.T) {
	root := t.TempDir()
	userScope := filepath.Join(root, "user")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{userScope, outside, filepath.Join(userScope, "ok")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	key := []byte("test-signing-key")
	perms := users.Permissions{Execute: true, Create: true}
	userRules := []rules.Rule{
		{Regex: true, Allow: false, Path: "^/secret$", Regexp: &rules.Regexp{Raw: "^/secret$"}},
	}
	st := newSecurityStorage(t, key, userScope, perms, []string{"go"}, userRules)

	server := &settings.Server{EnableExec: true}
	ts := httptest.NewServer(handle(commandsHandler, "/api/command", st, server))
	defer ts.Close()

	dial := func(t *testing.T, path string) (*websocket.Conn, error) {
		t.Helper()
		url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/command" + path
		conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{
			"X-Auth": []string{forgeSecurityToken(t, key, 1, perms)},
		})
		return conn, err
	}

	sendAndRead := func(t *testing.T, conn *websocket.Conn) (string, error) {
		t.Helper()
		if err := conn.WriteMessage(websocket.TextMessage, []byte("go version")); err != nil {
			return "", err
		}
		_, msg, err := conn.ReadMessage()
		return string(msg), err
	}

	t.Run("symlink escaping scope is rejected", func(t *testing.T) {
		if err := os.Symlink(outside, filepath.Join(userScope, "escape")); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
		conn, err := dial(t, "/escape")
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		defer conn.Close()

		msg, err := sendAndRead(t, conn)
		if err == nil && strings.Contains(msg, "go version") {
			t.Fatal("VULNERABLE: command executed with a working directory outside the user's scope")
		}
	})

	// Note on lexical traversal ("..", "..\" in the URL): FullPath computes
	// Join(userScope, Join("/", p)); Join("/", p) resolves every ".." against
	// the virtual root, so the result can never carry a leading ".." and the
	// final directory always stays lexically inside the scope. The genuinely
	// exploitable vector is a symlink whose target lives outside the scope —
	// the OS chdir follows it although the lexical path stays inside — which
	// is what the scoped-Stat gate above blocks.

	t.Run("directory denied by exact rule is rejected", func(t *testing.T) {
		if err := os.MkdirAll(filepath.Join(userScope, "secret"), 0o755); err != nil {
			t.Fatal(err)
		}
		conn, err := dial(t, "/secret")
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		defer conn.Close()

		msg, err := sendAndRead(t, conn)
		if err != nil {
			t.Fatalf("expected rejection message, got connection error: %v", err)
		}
		if msg != string(cmdNotAllowed) {
			t.Fatalf("expected command-not-allowed message, got %q", msg)
		}
	})

	t.Run("in-scope directory still works", func(t *testing.T) {
		conn, err := dial(t, "/ok")
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}
		defer conn.Close()

		msg, err := sendAndRead(t, conn)
		if err != nil {
			t.Fatalf("expected command output, got error: %v", err)
		}
		if !strings.Contains(msg, "go version") {
			t.Fatalf("unexpected command output: %q", msg)
		}
	})
}

// TestTusPostMkdirRespectsDenyRules verifies that the directory auto-creation
// in the tus POST handler honors deny rules that match the parent directory
// exactly (regex): the file path itself does not match such a rule, so without
// an explicit check the blocked directory would silently be created.
func TestTusPostMkdirRespectsDenyRules(t *testing.T) {
	scope := t.TempDir()
	key := []byte("test-signing-key")
	perms := users.Permissions{Create: true, Modify: true}
	userRules := []rules.Rule{
		{Regex: true, Allow: false, Path: "^/blocked$", Regexp: &rules.Regexp{Raw: "^/blocked$"}},
	}
	st := newSecurityStorage(t, key, scope, perms, nil, userRules)

	// Production requests always carry a leading slash (gorilla mux routes
	// from the root); relative paths would silently fail every rule match.
	req, err := http.NewRequest(http.MethodPost, "/blocked/new.txt", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Auth", forgeSecurityToken(t, key, 1, perms))
	req.Header.Set("Upload-Length", "10")

	recorder := httptest.NewRecorder()
	handler := handle(tusPostHandler(newMemoryUploadCache()), "", st, &settings.Server{})
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", recorder.Code)
	}
	if _, statErr := os.Stat(filepath.Join(scope, "blocked")); statErr == nil {
		t.Error("VULNERABLE: blocked directory was created despite the deny rule")
	}
}
