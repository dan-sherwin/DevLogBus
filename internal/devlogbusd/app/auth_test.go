package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPAuthOpenByDefault(t *testing.T) {
	auth := testAuthManager(t)
	handler := auth.withHTTPAuth(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/records", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	status := auth.statusForRequest(req)
	if status.LoginRequired || status.LoginEnabled || status.UserCount != 0 {
		t.Fatalf("status = %#v, want open default", status)
	}
}

func TestHTTPAuthCannotEnableLoginWithoutUsers(t *testing.T) {
	auth := testAuthManager(t)

	if err := auth.setLoginEnabled(true); err == nil {
		t.Fatalf("setLoginEnabled returned nil, want error")
	}
	if auth.loginRequired() {
		t.Fatalf("loginRequired = true, want false")
	}
}

func TestHTTPAuthRequiresSessionWhenEnabled(t *testing.T) {
	auth := testAuthManager(t)
	if _, err := auth.addUser(authCreateUserRequest{
		Username:    "dan",
		DisplayName: "Dan Sherwin",
		Password:    "correct-horse-battery-staple",
	}); err != nil {
		t.Fatalf("addUser: %v", err)
	}
	if err := auth.setLoginEnabled(true); err != nil {
		t.Fatalf("setLoginEnabled: %v", err)
	}

	handler := auth.withHTTPAuth(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rr := httptest.NewRecorder()
	handler(rr, httptest.NewRequest(http.MethodGet, "/api/records", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}

	loginCookie := loginForTest(t, auth, "dan", "correct-horse-battery-staple")
	req := httptest.NewRequest(http.MethodGet, "/api/records", nil)
	req.AddCookie(loginCookie)
	rr = httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("authenticated status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHTTPAuthRejectsBadPassword(t *testing.T) {
	auth := testAuthManager(t)
	if _, err := auth.addUser(authCreateUserRequest{
		Username:    "dan",
		DisplayName: "Dan Sherwin",
		Password:    "correct-horse-battery-staple",
	}); err != nil {
		t.Fatalf("addUser: %v", err)
	}

	body := bytes.NewBufferString(`{"username":"dan","password":"wrong-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	rr := httptest.NewRecorder()

	auth.handleHTTPAuthLogin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func testAuthManager(t *testing.T) *authManager {
	t.Helper()
	auth := newAuthManager()
	auth.now = func() time.Time { return time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC) }
	auth.persistLoginEnabled = func(bool) error { return nil }
	auth.persistUsers = func([]authUser) error { return nil }
	return auth
}

func loginForTest(t *testing.T, auth *authManager, username string, password string) *http.Cookie {
	t.Helper()
	payload, err := json.Marshal(authLoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("marshal login: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(payload))
	rr := httptest.NewRecorder()

	auth.handleHTTPAuthLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == authCookieName {
			return cookie
		}
	}
	t.Fatalf("login response did not set %s", authCookieName)
	return nil
}
