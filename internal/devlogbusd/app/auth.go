package app

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dan-sherwin/go-app-settings"
	settingsdb "github.com/dan-sherwin/go-app-settings/db"
	settingsmodels "github.com/dan-sherwin/go-app-settings/db/models"
)

const (
	authLoginEnabledSetting = "auth_login_enabled"
	authUsersSetting        = "auth_users"
	authCookieName          = "devlogbus_session"
	authSessionDuration     = 12 * time.Hour
	passwordIterations      = 210_000
	passwordSaltBytes       = 16
	passwordKeyBytes        = 32
)

var (
	defaultAuthManager = newAuthManager()
	errInvalidLogin    = errors.New("invalid username or password")
)

type authUser struct {
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

type authUserResponse struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

type authStatusResponse struct {
	LoginEnabled  bool              `json:"loginEnabled"`
	LoginRequired bool              `json:"loginRequired"`
	UserCount     int               `json:"userCount"`
	CurrentUser   *authUserResponse `json:"currentUser,omitempty"`
}

type authUsersResponse struct {
	Users []authUserResponse `json:"users"`
}

type authSettingsRequest struct {
	LoginEnabled bool `json:"loginEnabled"`
}

type authLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authCreateUserRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

type authSession struct {
	Username  string
	ExpiresAt time.Time
}

type authManager struct {
	mu                  sync.Mutex
	loginEnabled        bool
	users               []authUser
	sessions            map[string]authSession
	now                 func() time.Time
	persistLoginEnabled func(bool) error
	persistUsers        func([]authUser) error
}

func init() {
	defaultAuthManager.persistLoginEnabled = persistAuthLoginEnabled
	defaultAuthManager.persistUsers = persistAuthUsers

	app_settings.RegisterSetting(&app_settings.Setting{
		Name:        authLoginEnabledSetting,
		Description: "Require a DevLogBus UI login when users exist",
		GetFunc: func() string {
			return strconv.FormatBool(defaultAuthManager.loginEnabledConfigured())
		},
		SetFunc: func(s string) error {
			enabled, err := strconv.ParseBool(strings.TrimSpace(s))
			if err != nil {
				return err
			}
			defaultAuthManager.setLoginEnabledFromSettings(enabled)
			return nil
		},
	})
	app_settings.RegisterSetting(&app_settings.Setting{
		Name:              authUsersSetting,
		Description:       "DevLogBus UI users",
		Hidden:            true,
		ValueToStringFunc: authJSONValueToString,
		GetFunc: func() string {
			return defaultAuthManager.usersJSON()
		},
		SetFunc: func(s string) error {
			return defaultAuthManager.setUsersFromSettings(s)
		},
	})
}

func newAuthManager() *authManager {
	return &authManager{
		sessions: map[string]authSession{},
		now:      time.Now,
	}
}

func (m *authManager) loginEnabledConfigured() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loginEnabled
}

func (m *authManager) setLoginEnabledFromSettings(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loginEnabled = enabled
}

func (m *authManager) setLoginEnabled(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if enabled && len(m.users) == 0 {
		return fmt.Errorf("add at least one user before enabling login")
	}
	if m.persistLoginEnabled != nil {
		if err := m.persistLoginEnabled(enabled); err != nil {
			return err
		}
	}
	m.loginEnabled = enabled
	return nil
}

func (m *authManager) loginRequiredLocked() bool {
	return m.loginEnabled && len(m.users) > 0
}

func (m *authManager) loginRequired() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loginRequiredLocked()
}

func (m *authManager) statusForRequest(r *http.Request) authStatusResponse {
	currentUser, ok := m.userFromRequest(r)

	m.mu.Lock()
	defer m.mu.Unlock()
	status := authStatusResponse{
		LoginEnabled:  m.loginEnabled,
		LoginRequired: m.loginRequiredLocked(),
		UserCount:     len(m.users),
	}
	if ok {
		status.CurrentUser = &currentUser
	}
	return status
}

func (m *authManager) userResponses() []authUserResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	return userResponses(m.users)
}

func userResponses(users []authUser) []authUserResponse {
	responses := make([]authUserResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, authUserResponse{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			CreatedAt:   user.CreatedAt,
		})
	}
	return responses
}

func (m *authManager) addUser(req authCreateUserRequest) ([]authUserResponse, error) {
	username, err := normalizeAuthUsername(req.Username)
	if err != nil {
		return nil, err
	}
	displayName, err := normalizeDisplayName(req.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.userIndexLocked(username) >= 0 {
		return nil, fmt.Errorf("user %s already exists", username)
	}
	nextUsers := append(slices.Clone(m.users), authUser{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		CreatedAt:    m.now().UTC(),
	})
	sortAuthUsers(nextUsers)
	if m.persistUsers != nil {
		if err := m.persistUsers(nextUsers); err != nil {
			return nil, err
		}
	}
	m.users = nextUsers
	return userResponses(m.users), nil
}

func (m *authManager) deleteUser(username string) ([]authUserResponse, error) {
	username, err := normalizeAuthUsername(username)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	index := m.userIndexLocked(username)
	if index < 0 {
		return nil, fmt.Errorf("user %s not found", username)
	}
	nextUsers := slices.Clone(m.users)
	nextUsers = append(nextUsers[:index], nextUsers[index+1:]...)
	if m.persistUsers != nil {
		if err := m.persistUsers(nextUsers); err != nil {
			return nil, err
		}
	}
	m.users = nextUsers
	for token, session := range m.sessions {
		if session.Username == username {
			delete(m.sessions, token)
		}
	}
	return userResponses(m.users), nil
}

func (m *authManager) authenticate(username string, password string) (string, authUserResponse, time.Time, error) {
	username, err := normalizeAuthUsername(username)
	if err != nil {
		return "", authUserResponse{}, time.Time{}, errInvalidLogin
	}

	m.mu.Lock()
	index := m.userIndexLocked(username)
	if index < 0 {
		m.mu.Unlock()
		return "", authUserResponse{}, time.Time{}, errInvalidLogin
	}
	user := m.users[index]
	m.mu.Unlock()

	ok, err := verifyPasswordHash(user.PasswordHash, password)
	if err != nil || !ok {
		return "", authUserResponse{}, time.Time{}, errInvalidLogin
	}
	token, err := randomToken(32)
	if err != nil {
		return "", authUserResponse{}, time.Time{}, err
	}
	expiresAt := m.now().UTC().Add(authSessionDuration)

	m.mu.Lock()
	m.sessions[token] = authSession{
		Username:  user.Username,
		ExpiresAt: expiresAt,
	}
	m.mu.Unlock()

	return token, authUserResponse{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
	}, expiresAt, nil
}

func (m *authManager) logout(r *http.Request) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, cookie.Value)
}

func (m *authManager) userFromRequest(r *http.Request) (authUserResponse, bool) {
	if r == nil {
		return authUserResponse{}, false
	}
	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie.Value == "" {
		return authUserResponse{}, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[cookie.Value]
	if !ok {
		return authUserResponse{}, false
	}
	if !m.now().Before(session.ExpiresAt) {
		delete(m.sessions, cookie.Value)
		return authUserResponse{}, false
	}
	index := m.userIndexLocked(session.Username)
	if index < 0 {
		delete(m.sessions, cookie.Value)
		return authUserResponse{}, false
	}
	user := m.users[index]
	return authUserResponse{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
	}, true
}

func (m *authManager) userIndexLocked(username string) int {
	for i, user := range m.users {
		if user.Username == username {
			return i
		}
	}
	return -1
}

func (m *authManager) usersJSON() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := json.Marshal(m.users)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func (m *authManager) setUsersFromSettings(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "[]"
	}
	var users []authUser
	if err := json.Unmarshal([]byte(value), &users); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for i := range users {
		username, err := normalizeAuthUsername(users[i].Username)
		if err != nil {
			return err
		}
		if _, ok := seen[username]; ok {
			return fmt.Errorf("duplicate auth user %s", username)
		}
		seen[username] = struct{}{}
		displayName, err := normalizeDisplayName(users[i].DisplayName)
		if err != nil {
			return err
		}
		if users[i].PasswordHash == "" {
			return fmt.Errorf("auth user %s is missing a password hash", username)
		}
		users[i].Username = username
		users[i].DisplayName = displayName
	}
	sortAuthUsers(users)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.users = users
	return nil
}

func sortAuthUsers(users []authUser) {
	slices.SortFunc(users, func(a, b authUser) int {
		return strings.Compare(a.Username, b.Username)
	})
}

func normalizeAuthUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(username) < 3 {
		return "", fmt.Errorf("username must be at least 3 characters")
	}
	if len(username) > 64 {
		return "", fmt.Errorf("username must be 64 characters or fewer")
	}
	for _, r := range username {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == '@' {
			continue
		}
		return "", fmt.Errorf("username can only use letters, numbers, '.', '_', '-', or '@'")
	}
	return username, nil
}

func normalizeDisplayName(displayName string) (string, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return "", fmt.Errorf("display name is required")
	}
	if len(displayName) > 80 {
		return "", fmt.Errorf("display name must be 80 characters or fewer")
	}
	return displayName, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 1024 {
		return fmt.Errorf("password must be 1024 characters or fewer")
	}
	return nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordKeyBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"pbkdf2-sha256$%d$%s$%s",
		passwordIterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func verifyPasswordHash(storedHash string, password string) (bool, error) {
	parts := strings.Split(storedHash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false, fmt.Errorf("unsupported password hash")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false, fmt.Errorf("invalid password hash iterations")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func randomToken(byteCount int) (string, error) {
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func authJSONValueToString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

func persistAuthLoginEnabled(enabled bool) error {
	return saveAuthSetting(authLoginEnabledSetting, strconv.FormatBool(enabled))
}

func persistAuthUsers(users []authUser) error {
	data, err := json.Marshal(users)
	if err != nil {
		return err
	}
	return saveAuthSetting(authUsersSetting, string(data))
}

func saveAuthSetting(key string, value string) error {
	if settingsdb.AppSetting == nil {
		return fmt.Errorf("settings database is not initialized")
	}
	return settingsdb.AppSetting.Save(&settingsmodels.AppSetting{
		Key:   key,
		Value: value,
	})
}

func (m *authManager) withHTTPAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.loginRequired() {
			next(w, r)
			return
		}
		if _, ok := m.userFromRequest(r); !ok {
			writeAPIError(w, http.StatusUnauthorized, "login required")
			return
		}
		next(w, r)
	}
}

func (m *authManager) handleHTTPAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, m.statusForRequest(r))
}

func (m *authManager) handleHTTPAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req authLoginRequest
	if err := decodeHTTPJSON(w, r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, user, expiresAt, err := m.authenticate(req.Username, req.Password)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(authSessionDuration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	status := m.statusForRequest(r)
	status.CurrentUser = &user
	writeJSON(w, http.StatusOK, status)
}

func (m *authManager) handleHTTPAuthLogout(w http.ResponseWriter, r *http.Request) {
	m.logout(r)
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, m.statusForRequest(r))
}

func (m *authManager) handleHTTPAuthSettings(w http.ResponseWriter, r *http.Request) {
	var req authSettingsRequest
	if err := decodeHTTPJSON(w, r, &req); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := m.setLoginEnabled(req.LoginEnabled); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m.statusForRequest(r))
}

func (m *authManager) handleHTTPAuthUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, authUsersResponse{Users: m.userResponses()})
	case http.MethodPost:
		var req authCreateUserRequest
		if err := decodeHTTPJSON(w, r, &req); err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		users, err := m.addUser(req)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "already exists") {
				status = http.StatusConflict
			}
			writeAPIError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, authUsersResponse{Users: users})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *authManager) handleHTTPAuthUser(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/auth/users/")
	if username == "" || strings.Contains(username, "/") {
		writeAPIError(w, http.StatusNotFound, "user not found")
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	users, err := m.deleteUser(username)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authUsersResponse{Users: users})
}

func decodeHTTPJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode request: multiple JSON values")
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
