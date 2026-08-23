package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	CookieName = "th_session"
	maxAge     = 7 * 24 * time.Hour
)

type Gate struct {
	// Legacy single-user fallback when users.json empty
	user   string
	pass   string
	secret []byte
	secure bool
	users  *UserStore
	keys   *ConnectKeyStore
}

func NewFromEnv() *Gate {
	user := strings.TrimSpace(os.Getenv("AUTH_USER"))
	pass := strings.TrimSpace(os.Getenv("AUTH_PASSWORD"))
	sec := strings.TrimSpace(os.Getenv("AUTH_SECRET"))
	if sec == "" {
		sec = user + ":" + pass + ":threads-dashboard"
	}
	g := &Gate{
		user:   user,
		pass:   pass,
		secret: []byte(sec),
		secure: strings.EqualFold(os.Getenv("AUTH_SECURE"), "1") ||
			strings.EqualFold(os.Getenv("AUTH_SECURE"), "true"),
	}
	return g
}

func (g *Gate) SetUsers(s *UserStore) {
	if g != nil {
		g.users = s
	}
}

func (g *Gate) Users() *UserStore {
	if g == nil {
		return nil
	}
	return g.users
}

func (g *Gate) SetConnectKeys(s *ConnectKeyStore) {
	if g != nil {
		g.keys = s
	}
}

func (g *Gate) ConnectKeys() *ConnectKeyStore {
	if g == nil {
		return nil
	}
	return g.keys
}

func (g *Gate) Enabled() bool {
	if g == nil {
		return false
	}
	if g.users != nil && g.users.Count() > 0 {
		return true
	}
	return g.user != "" && g.pass != ""
}

// Authenticate checks users.json first, then legacy AUTH_USER/PASSWORD.
func (g *Gate) Authenticate(username, password string) (*User, error) {
	if g.users != nil && g.users.Count() > 0 {
		return g.users.Authenticate(username, password)
	}
	// Legacy env login → treat as admin
	if g.user != "" && g.pass != "" &&
		subtleConstEq(username, g.user) && subtleConstEq(password, g.pass) {
		return &User{
			ID:       "admin",
			Username: g.user,
			Role:     RoleAdmin,
			Active:   true,
		}, nil
	}
	return nil, errBadCreds
}

var errBadCreds = errAuth("username/password salah")

type authError string

func (e authError) Error() string { return string(e) }
func errAuth(s string) error      { return authError(s) }

func (g *Gate) Check(user, pass string) bool {
	u, err := g.Authenticate(user, pass)
	return err == nil && u != nil
}

func subtleConstEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (g *Gate) IssueCookie(w http.ResponseWriter, user string) {
	g.IssueSession(w, &User{Username: user, Role: RoleAdmin})
}

func (g *Gate) IssueSession(w http.ResponseWriter, u *User) {
	if u == nil {
		return
	}
	exp := time.Now().Add(maxAge).Unix()
	role := u.Role
	if role == "" {
		role = RoleAdmin
	}
	payload := strings.Join([]string{u.Username, role, u.TenantID, itoa(exp)}, "|")
	sig := g.sign(payload)
	val := base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sig))
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   g.secure,
		MaxAge:   int(maxAge.Seconds()),
	})
}

func (g *Gate) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   g.secure,
		MaxAge:   -1,
	})
}

func (g *Gate) Valid(r *http.Request) bool {
	return g.SessionFromRequest(r) != nil
}

func (g *Gate) sign(payload string) string {
	m := hmac.New(sha256.New, g.secret)
	_, _ = m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

func (g *Gate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.Enabled() || isPublic(r) {
			next.ServeHTTP(w, r)
			return
		}
		if g.Valid(r) {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"login required","code":"unauthorized"}`))
			return
		}
		nextURL := path
		if r.URL.RawQuery != "" {
			nextURL += "?" + r.URL.RawQuery
		}
		login := "/app/login"
		if strings.HasPrefix(path, "/core") {
			login = "/core/login.html"
		}
		http.Redirect(w, r, login+"?next="+url.QueryEscape(nextURL), http.StatusFound)
		return
	})
}

func isPublic(r *http.Request) bool {
	p := r.URL.Path
	switch {
	case p == "/login.html" || p == "/app/login" || p == "/app/login.html" || p == "/core/login.html":
		return true
	case p == "/" || p == "/index.html":
		return true
	case p == "/privacy" || p == "/privacy.html" || p == "/privacy-policy" || p == "/privacy-policy.html":
		return true
	case p == "/openapi.yaml" || p == "/openapi.json" || p == "/openapi":
		return true
	case p == "/docs" || p == "/docs.html" || p == "/api-docs":
		return true
	case p == "/health":
		return true
	case p == "/auth/threads/callback" || p == "/auth/instagram/callback":
		return true
	case strings.HasPrefix(p, "/auth/repliz/"):
		return true
	case p == "/auth/meta/deauthorize" || p == "/auth/meta/data-deletion" || p == "/auth/meta/data-deletion-status":
		return true
	case p == "/api/auth/login" || p == "/api/auth/logout" || p == "/api/auth/me":
		return true
	case strings.HasPrefix(p, "/media/lazy/"):
		return true
	case strings.HasPrefix(p, "/media/thumbs/"):
		return true
	case strings.HasPrefix(p, "/css/") || strings.HasPrefix(p, "/js/"):
		return true
	case strings.HasPrefix(p, "/core/js/") || strings.HasPrefix(p, "/core/css/"):
		return true
	default:
		return false
	}
}

// RequireAdminHTML blocks non-admin browsers from /core (except public login/js).
func (g *Gate) RequireAdminHTML(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if !strings.HasPrefix(p, "/core") || isPublic(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !g.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		sess := g.SessionFromRequest(r)
		if sess != nil && sess.IsAdmin() {
			next.ServeHTTP(w, r)
			return
		}
		if sess == nil {
			http.Redirect(w, r, "/core/login.html?next="+url.QueryEscape(p), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/app/ringkasan", http.StatusFound)
	})
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func atoi(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// RandomSecret helper for docs/scripts.
func RandomSecret() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
