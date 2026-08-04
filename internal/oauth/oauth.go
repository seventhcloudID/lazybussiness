package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	PublicBase string
	StateSecret []byte

	ThreadsAppID     string
	ThreadsAppSecret string
	ThreadsScopes    string

	InstagramAppID     string
	InstagramAppSecret string
	InstagramScopes    string

	HTTP *http.Client
}

func FromEnv() *Config {
	pub := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/")
	sec := strings.TrimSpace(os.Getenv("AUTH_SECRET"))
	if sec == "" {
		sec = strings.TrimSpace(os.Getenv("OAUTH_STATE_SECRET"))
	}
	if sec == "" {
		sec = "malesngonten-oauth-dev"
	}
	thID := firstEnv("THREADS_APP_ID", "META_APP_ID", "FACEBOOK_APP_ID")
	thSec := firstEnv("THREADS_APP_SECRET", "META_APP_SECRET", "FACEBOOK_APP_SECRET")
	igID := firstEnv("INSTAGRAM_APP_ID", "META_APP_ID", "FACEBOOK_APP_ID")
	igSec := firstEnv("INSTAGRAM_APP_SECRET", "META_APP_SECRET", "FACEBOOK_APP_SECRET", "INSTAGRAM_APP_SECRET")
	// Prefer dedicated IG secret if set alone
	if v := strings.TrimSpace(os.Getenv("INSTAGRAM_APP_SECRET")); v != "" {
		igSec = v
	}
	return &Config{
		PublicBase:         pub,
		StateSecret:        []byte(sec),
		ThreadsAppID:       thID,
		ThreadsAppSecret:   thSec,
		ThreadsScopes:      envOr("THREADS_OAUTH_SCOPES", "threads_basic,threads_content_publish,threads_manage_replies,threads_read_replies,threads_manage_insights"),
		InstagramAppID:     igID,
		InstagramAppSecret: igSec,
		InstagramScopes:    envOr("INSTAGRAM_OAUTH_SCOPES", "instagram_business_basic,instagram_business_content_publish,instagram_business_manage_comments,instagram_business_manage_insights"),
		HTTP:               &http.Client{Timeout: 45 * time.Second},
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func (c *Config) ThreadsReady() bool {
	return c != nil && c.PublicBase != "" && c.ThreadsAppID != "" && c.ThreadsAppSecret != ""
}

func (c *Config) InstagramReady() bool {
	return c != nil && c.PublicBase != "" && c.InstagramAppID != "" && c.InstagramAppSecret != ""
}

func (c *Config) ThreadsRedirectURI() string {
	return c.PublicBase + "/auth/threads/callback"
}

func (c *Config) InstagramRedirectURI() string {
	return c.PublicBase + "/auth/instagram/callback"
}

func (c *Config) Status() map[string]any {
	return map[string]any{
		"public_base":        c.PublicBase,
		"threads_ready":      c.ThreadsReady(),
		"instagram_ready":    c.InstagramReady(),
		"threads_redirect":   c.ThreadsRedirectURI(),
		"instagram_redirect": c.InstagramRedirectURI(),
		"deauthorize_url":    c.DeauthorizeURI(),
		"data_deletion_url":  c.DataDeletionURI(),
	}
}

func (c *Config) DeauthorizeURI() string {
	if c == nil || c.PublicBase == "" {
		return ""
	}
	return c.PublicBase + "/auth/meta/deauthorize"
}

func (c *Config) DataDeletionURI() string {
	if c == nil || c.PublicBase == "" {
		return ""
	}
	return c.PublicBase + "/auth/meta/data-deletion"
}

func (c *Config) DataDeletionStatusURI(code string) string {
	return c.PublicBase + "/auth/meta/data-deletion-status?code=" + url.QueryEscape(code)
}

// ParseSignedRequest memverifikasi signed_request Meta (deauth / data deletion).
func (c *Config) ParseSignedRequest(signed string) (userID string, err error) {
	signed = strings.TrimSpace(signed)
	parts := strings.Split(signed, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("signed_request tidak valid")
	}
	sigRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		sigRaw, err = base64.URLEncoding.DecodeString(parts[0])
		if err != nil {
			return "", fmt.Errorf("signature decode gagal")
		}
	}
	payload := parts[1]
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		payloadJSON, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return "", fmt.Errorf("payload decode gagal")
		}
	}
	var data struct {
		Algorithm string `json:"algorithm"`
		UserID    string `json:"user_id"`
	}
	if err := json.Unmarshal(payloadJSON, &data); err != nil {
		return "", fmt.Errorf("payload JSON tidak valid")
	}
	if data.UserID == "" {
		return "", fmt.Errorf("user_id kosong")
	}
	if alg := strings.ToUpper(strings.TrimSpace(data.Algorithm)); alg != "" && alg != "HMAC-SHA256" {
		return "", fmt.Errorf("algorithm tidak didukung")
	}
	secrets := c.appSecrets()
	if len(secrets) == 0 {
		return "", fmt.Errorf("app secret belum di-set")
	}
	for _, sec := range secrets {
		mac := hmac.New(sha256.New, []byte(sec))
		_, _ = mac.Write([]byte(payload))
		if hmac.Equal(sigRaw, mac.Sum(nil)) {
			return data.UserID, nil
		}
	}
	return "", fmt.Errorf("signature tidak cocok")
}

func (c *Config) appSecrets() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range []string{c.ThreadsAppSecret, c.InstagramAppSecret} {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func ConfirmationCode(userID string) string {
	userID = strings.TrimSpace(userID)
	sum := sha256.Sum256([]byte("del:" + userID + ":" + fmt.Sprint(time.Now().Unix()/86400)))
	return hex.EncodeToString(sum[:8])
}

type statePayload struct {
	AccountID string `json:"a"`
	Provider  string `json:"p"`
	Exp       int64  `json:"e"`
	Nonce     string `json:"n"`
}

func (c *Config) SignState(accountID, provider string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	provider = strings.TrimSpace(provider)
	if accountID == "" || provider == "" {
		return "", fmt.Errorf("account_id dan provider wajib")
	}
	nonce := make([]byte, 8)
	_, _ = rand.Read(nonce)
	p := statePayload{
		AccountID: accountID,
		Provider:  provider,
		Exp:       time.Now().Add(15 * time.Minute).Unix(),
		Nonce:     hex.EncodeToString(nonce),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, c.StateSecret)
	_, _ = mac.Write([]byte(body))
	sig := hex.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

func (c *Config) ParseState(state, wantProvider string) (accountID string, err error) {
	state = strings.TrimSpace(state)
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("state tidak valid")
	}
	body, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, c.StateSecret)
	_, _ = mac.Write([]byte(body))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(sig)) {
		return "", fmt.Errorf("state signature salah")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", fmt.Errorf("state decode gagal")
	}
	var p statePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", fmt.Errorf("state payload rusak")
	}
	if p.Exp < time.Now().Unix() {
		return "", fmt.Errorf("state kedaluwarsa — mulai connect lagi")
	}
	if wantProvider != "" && p.Provider != wantProvider {
		return "", fmt.Errorf("state provider tidak cocok")
	}
	if strings.TrimSpace(p.AccountID) == "" {
		return "", fmt.Errorf("account_id kosong di state")
	}
	return p.AccountID, nil
}

func (c *Config) ThreadsAuthorizeURL(accountID string) (string, error) {
	if !c.ThreadsReady() {
		return "", fmt.Errorf("set PUBLIC_BASE_URL + THREADS_APP_ID + THREADS_APP_SECRET di .env")
	}
	state, err := c.SignState(accountID, "threads")
	if err != nil {
		return "", err
	}
	q := url.Values{
		"client_id":     {c.ThreadsAppID},
		"redirect_uri":  {c.ThreadsRedirectURI()},
		"scope":         {c.ThreadsScopes},
		"response_type": {"code"},
		"state":         {state},
	}
	return "https://threads.net/oauth/authorize?" + q.Encode(), nil
}

func (c *Config) InstagramAuthorizeURL(accountID string) (string, error) {
	if !c.InstagramReady() {
		return "", fmt.Errorf("set PUBLIC_BASE_URL + INSTAGRAM_APP_ID + INSTAGRAM_APP_SECRET di .env")
	}
	state, err := c.SignState(accountID, "instagram")
	if err != nil {
		return "", err
	}
	q := url.Values{
		"client_id":     {c.InstagramAppID},
		"redirect_uri":  {c.InstagramRedirectURI()},
		"scope":         {c.InstagramScopes},
		"response_type": {"code"},
		"state":         {state},
	}
	return "https://www.instagram.com/oauth/authorize?" + q.Encode(), nil
}

func CleanCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.TrimSuffix(code, "#_")
	code = strings.Split(code, "#")[0]
	return strings.TrimSpace(code)
}

type TokenResult struct {
	AccessToken string
	UserID      string
	ExpiresIn   int64
	Raw         json.RawMessage
}

func (c *Config) ExchangeThreadsCode(code string) (*TokenResult, error) {
	code = CleanCode(code)
	if code == "" {
		return nil, fmt.Errorf("code kosong")
	}
	form := url.Values{
		"client_id":     {c.ThreadsAppID},
		"client_secret": {c.ThreadsAppSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.ThreadsRedirectURI()},
		"code":          {code},
	}
	req, err := http.NewRequest(http.MethodPost, "https://graph.threads.net/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("threads token exchange: %s", truncate(string(raw), 280))
	}
	var short struct {
		AccessToken string `json:"access_token"`
		UserID      any    `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &short); err != nil || short.AccessToken == "" {
		return nil, fmt.Errorf("threads short token tidak valid")
	}
	longTok, exp, longRaw, err := c.exchangeThreadsLongLived(short.AccessToken)
	if err != nil {
		// Short-lived masih bisa dipakai sementara
		return &TokenResult{AccessToken: short.AccessToken, UserID: fmt.Sprint(short.UserID), Raw: raw}, nil
	}
	return &TokenResult{AccessToken: longTok, UserID: fmt.Sprint(short.UserID), ExpiresIn: exp, Raw: longRaw}, nil
}

func (c *Config) exchangeThreadsLongLived(short string) (token string, exp int64, raw json.RawMessage, err error) {
	u := "https://graph.threads.net/access_token?" + url.Values{
		"grant_type":    {"th_exchange_token"},
		"client_secret": {c.ThreadsAppSecret},
		"access_token":  {short},
	}.Encode()
	res, err := c.HTTP.Get(u)
	if err != nil {
		return "", 0, nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return "", 0, nil, fmt.Errorf("threads long-lived: %s", truncate(string(body), 280))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", 0, nil, fmt.Errorf("threads long-lived response invalid")
	}
	return out.AccessToken, out.ExpiresIn, json.RawMessage(body), nil
}

func (c *Config) ExchangeInstagramCode(code string) (*TokenResult, error) {
	code = CleanCode(code)
	if code == "" {
		return nil, fmt.Errorf("code kosong")
	}
	form := url.Values{
		"client_id":     {c.InstagramAppID},
		"client_secret": {c.InstagramAppSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.InstagramRedirectURI()},
		"code":          {code},
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.instagram.com/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("instagram token exchange: %s", truncate(string(raw), 280))
	}
	var short struct {
		AccessToken string `json:"access_token"`
		UserID      any    `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &short); err != nil || short.AccessToken == "" {
		return nil, fmt.Errorf("instagram short token tidak valid")
	}
	longTok, exp, longRaw, err := c.exchangeInstagramLongLived(short.AccessToken)
	if err != nil {
		return &TokenResult{AccessToken: short.AccessToken, UserID: fmt.Sprint(short.UserID), Raw: raw}, nil
	}
	return &TokenResult{AccessToken: longTok, UserID: fmt.Sprint(short.UserID), ExpiresIn: exp, Raw: longRaw}, nil
}

func (c *Config) exchangeInstagramLongLived(short string) (token string, exp int64, raw json.RawMessage, err error) {
	form := url.Values{
		"grant_type":    {"ig_exchange_token"},
		"client_secret": {c.InstagramAppSecret},
		"access_token":  {short},
	}
	// GET dulu (dokumentasi Meta), fallback POST jika Meta tolak method.
	u := "https://graph.instagram.com/access_token?" + form.Encode()
	res, err := c.HTTP.Get(u)
	if err != nil {
		return "", 0, nil, err
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode >= 400 {
		req, rerr := http.NewRequest(http.MethodPost, "https://graph.instagram.com/access_token", strings.NewReader(form.Encode()))
		if rerr != nil {
			return "", 0, nil, fmt.Errorf("instagram long-lived: %s", truncate(string(body), 280))
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res2, rerr := c.HTTP.Do(req)
		if rerr != nil {
			return "", 0, nil, fmt.Errorf("instagram long-lived: %s", truncate(string(body), 280))
		}
		body, _ = io.ReadAll(res2.Body)
		res2.Body.Close()
		if res2.StatusCode >= 400 {
			return "", 0, nil, fmt.Errorf("instagram long-lived: %s", truncate(string(body), 280))
		}
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", 0, nil, fmt.Errorf("instagram long-lived response invalid")
	}
	return out.AccessToken, out.ExpiresIn, json.RawMessage(body), nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
