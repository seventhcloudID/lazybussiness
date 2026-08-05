package auth

import (
	"crypto/hmac"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

func bearerAPIKey(r *http.Request, keys *ConnectKeyStore) (name string, ok bool) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	raw := strings.TrimSpace(parts[1])
	if raw == "" {
		return "", false
	}
	if keys != nil {
		return keys.Validate(raw)
	}
	return (&ConnectKeyStore{}).Validate(raw)
}

// Session is parsed from the auth cookie.
type Session struct {
	Username string
	Role     string
	TenantID string
	Expires  int64
}

func (s *Session) IsAdmin() bool {
	return s != nil && s.Role == RoleAdmin
}

func (s *Session) IsTenant() bool {
	return s != nil && s.Role == RoleTenant
}

// SessionFromRequest returns the session if cookie or Bearer API key is valid.
func (g *Gate) SessionFromRequest(r *http.Request) *Session {
	if g == nil {
		return nil
	}
	// Auth disabled (dev): synthetic admin
	if !g.Enabled() {
		return &Session{Username: "dev", Role: RoleAdmin, Expires: time.Now().Add(maxAge).Unix()}
	}
	if name, ok := bearerAPIKey(r, g.keys); ok {
		return &Session{
			Username: "api:" + name,
			Role:     RoleAdmin,
			Expires:  time.Now().Add(maxAge).Unix(),
		}
	}
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	parts := strings.Split(string(raw), "|")
	// New: username|role|tenantID|exp|sig
	// Legacy: username|exp|sig
	var user, role, tenantID, expStr, sig string
	switch len(parts) {
	case 3:
		user, expStr, sig = parts[0], parts[1], parts[2]
		role = RoleAdmin
	case 5:
		user, role, tenantID, expStr, sig = parts[0], parts[1], parts[2], parts[3], parts[4]
	default:
		return nil
	}
	exp := atoi(expStr)
	if exp <= 0 || time.Now().Unix() > exp {
		return nil
	}
	payload := strings.Join([]string{user, role, tenantID, expStr}, "|")
	legacyPayload := user + "|" + expStr
	ok := hmac.Equal([]byte(sig), []byte(g.sign(payload))) ||
		(len(parts) == 3 && hmac.Equal([]byte(sig), []byte(g.sign(legacyPayload))))
	if !ok {
		return nil
	}
	if role == "" {
		role = RoleAdmin
	}
	return &Session{Username: user, Role: role, TenantID: tenantID, Expires: exp}
}
