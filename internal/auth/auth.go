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
	user   string
	pass   string
	secret []byte
	secure bool
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

func (g *Gate) Enabled() bool {
	return g != nil && g.user != "" && g.pass != ""
}

func (g *Gate) Check(user, pass string) bool {
	if !g.Enabled() {
		return true
	}
	return subtleConstEq(user, g.user) && subtleConstEq(pass, g.pass)
}

func subtleConstEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (g *Gate) IssueCookie(w http.ResponseWriter, user string) {
	exp := time.Now().Add(maxAge).Unix()
	payload := user + "|" + itoa(exp)
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
	if !g.Enabled() {
		return true
	}
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return false
	}
	user, expStr, sig := parts[0], parts[1], parts[2]
	if user != g.user {
		return false
	}
	exp := atoi(expStr)
	if exp <= 0 || time.Now().Unix() > exp {
		return false
	}
	payload := user + "|" + expStr
	return hmac.Equal([]byte(sig), []byte(g.sign(payload)))
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
		http.Redirect(w, r, "/login.html?next="+url.QueryEscape(nextURL), http.StatusFound)
	})
}

func isPublic(r *http.Request) bool {
	p := r.URL.Path
	switch {
	case p == "/login.html":
		return true
	case p == "/health":
		return true
	case p == "/api/auth/login" || p == "/api/auth/logout" || p == "/api/auth/me":
		return true
	case strings.HasPrefix(p, "/media/lazy/"):
		return true // Meta must fetch carousel images without cookie
	case strings.HasPrefix(p, "/media/thumbs/"):
		return true // Meta must fetch Threads thumbnail images without cookie
	case strings.HasPrefix(p, "/css/") || strings.HasPrefix(p, "/js/"):
		return true
	default:
		return false
	}
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
