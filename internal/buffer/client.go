package buffer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const endpoint = "https://api.buffer.com"

// Client pushes to Buffer (TikTok photos + X/Twitter threads, Notify Me).
// Satu instance per akun — key disimpan di keyPath (bukan global).
type Client struct {
	apiKey           string
	keyPath          string
	orgID            string
	tiktokChannelID  string
	twitterChannelID string
	hc               *http.Client

	mu             sync.Mutex
	resolvedTikTok string
	resolvedX      string
}

type Channel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Service     string `json:"service"`
	IsLocked    bool   `json:"isLocked"`
}

type CreateResult struct {
	PostID         string `json:"post_id"`
	Status         string `json:"status"`
	SchedulingType string `json:"scheduling_type"`
	ChannelID      string `json:"channel_id"`
	Service        string `json:"service"`
}

// CreatePhotoResult kept for callers; same shape as CreateResult.
type CreatePhotoResult = CreateResult

// NewAt builds a Buffer client for one account. Key dari file keyPath saja.
// Channel/org di-detect dari key akun ini — jangan pakai BUFFER_*_CHANNEL_ID global
// supaya multi-akun tidak salah kirim ke channel akun lain.
func NewAt(keyPath string) *Client {
	if v := strings.TrimSpace(os.Getenv("BUFFER_ENABLED")); v == "0" || strings.EqualFold(v, "false") {
		return &Client{keyPath: keyPath, hc: &http.Client{Timeout: 90 * time.Second}}
	}
	return &Client{
		apiKey:  loadStoredAPIKeyAt(keyPath),
		keyPath: keyPath,
		hc:      &http.Client{Timeout: 90 * time.Second},
	}
}

// SeedFromEnv writes BUFFER_API_KEY into keyPath if file kosong (sekali, per akun yang dipanggil).
func SeedFromEnv(keyPath string) {
	if loadStoredAPIKeyAt(keyPath) != "" {
		return
	}
	key := normalizeKey(os.Getenv("BUFFER_API_KEY"))
	if key == "" {
		return
	}
	_ = saveStoredAPIKeyAt(keyPath, key)
}

// SetAPIKey menyimpan key ke file akun ini dan reset cache channel.
func (c *Client) SetAPIKey(key string) error {
	if c == nil {
		return fmt.Errorf("buffer client nil")
	}
	key = normalizeKey(key)
	if key == "" {
		return fmt.Errorf("api key wajib")
	}
	if err := saveStoredAPIKeyAt(c.keyPath, key); err != nil {
		return err
	}
	c.mu.Lock()
	c.apiKey = key
	c.orgID = ""
	c.tiktokChannelID = ""
	c.twitterChannelID = ""
	c.resolvedTikTok = ""
	c.resolvedX = ""
	c.mu.Unlock()
	return nil
}

// ClearAPIKey menghapus key UI akun ini.
func (c *Client) ClearAPIKey() error {
	if c == nil {
		return fmt.Errorf("buffer client nil")
	}
	if err := saveStoredAPIKeyAt(c.keyPath, ""); err != nil {
		return err
	}
	c.mu.Lock()
	c.apiKey = ""
	c.orgID = ""
	c.tiktokChannelID = ""
	c.twitterChannelID = ""
	c.resolvedTikTok = ""
	c.resolvedX = ""
	c.mu.Unlock()
	return nil
}

// KeySource: "file" or "".
func (c *Client) KeySource() string {
	if c == nil || !c.Enabled() {
		return ""
	}
	return "file"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// KeyHint returns last 4 chars for debug (never the full secret).
func (c *Client) KeyHint() string {
	if !c.Enabled() {
		return ""
	}
	k := c.apiKey
	if len(k) <= 4 {
		return "****"
	}
	return "…" + k[len(k)-4:]
}

func (c *Client) Enabled() bool {
	return c != nil && strings.TrimSpace(c.apiKey) != ""
}

func (c *Client) Status() map[string]any {
	if c == nil || !c.Enabled() {
		return map[string]any{
			"ok":      false,
			"enabled": false,
			"note":    "Tempel Buffer API key untuk akun/workspace ini (tidak dipakai akun lain).",
		}
	}
	out := map[string]any{
		"ok":      true,
		"enabled": true,
	}
	out["key_hint"] = c.KeyHint()
	out["key_source"] = c.KeySource()
	if org, err := c.OrganizationID(); err == nil {
		out["org_id"] = org
	} else {
		out["org_error"] = err.Error()
		out["token_ok"] = false
		return out
	}

	chs, err := c.ListChannels()
	if err != nil {
		out["channels_error"] = err.Error()
		out["token_ok"] = false
		return out
	}

	channels := make([]map[string]any, 0, len(chs))
	for _, ch := range chs {
		channels = append(channels, map[string]any{
			"id":           ch.ID,
			"name":         ch.Name,
			"display_name": ch.DisplayName,
			"service":      ch.Service,
			"locked":       ch.IsLocked,
		})
	}
	out["channels"] = channels
	out["channel_count"] = len(channels)

	tokenOK := true
	if ch, err := c.TikTokChannelID(); err == nil {
		out["tiktok_channel_id"] = ch
		out["tiktok_ok"] = true
		for _, x := range chs {
			if x.ID == ch {
				out["tiktok_name"] = firstNonEmpty(x.DisplayName, x.Name)
				out["tiktok_service"] = x.Service
				break
			}
		}
	} else {
		out["tiktok_ok"] = false
		out["tiktok_error"] = err.Error()
		if strings.Contains(strings.ToLower(err.Error()), "access token") {
			tokenOK = false
		}
	}
	if ch, err := c.TwitterChannelID(); err == nil {
		out["twitter_channel_id"] = ch
		out["twitter_ok"] = true
		for _, x := range chs {
			if x.ID == ch {
				out["twitter_name"] = firstNonEmpty(x.DisplayName, x.Name)
				out["twitter_service"] = x.Service
				break
			}
		}
	} else {
		out["twitter_ok"] = false
		out["twitter_error"] = err.Error()
		if strings.Contains(strings.ToLower(err.Error()), "access token") {
			tokenOK = false
		}
	}
	out["token_ok"] = tokenOK
	return out
}

func (c *Client) gql(query string, variables map[string]any, out any) error {
	if !c.Enabled() {
		return fmt.Errorf("Buffer belum dikonfigurasi")
	}
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("buffer response: %s", truncate(string(raw), 200))
	}
	if len(envelope.Errors) > 0 {
		msg := envelope.Errors[0].Message
		low := strings.ToLower(msg)
		if strings.Contains(low, "not authorized") ||
			strings.Contains(low, "unauthorized") ||
			strings.Contains(low, "access token") ||
			(strings.Contains(low, "invalid") && strings.Contains(low, "token")) {
			return fmt.Errorf("buffer: access token tidak valid — buat API key baru di publish.buffer.com/settings/api, lalu simpan di Workspace → Kelola untuk akun ini")
		}
		return fmt.Errorf("buffer: %s", msg)
	}
	if out == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("buffer: data kosong")
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *Client) OrganizationID() (string, error) {
	if id := strings.TrimSpace(c.orgID); id != "" {
		return id, nil
	}
	var data struct {
		Account struct {
			Organizations []struct {
				ID string `json:"id"`
			} `json:"organizations"`
		} `json:"account"`
	}
	err := c.gql(`query { account { organizations { id } } }`, nil, &data)
	if err != nil {
		return "", err
	}
	if len(data.Account.Organizations) == 0 {
		return "", fmt.Errorf("buffer: tidak ada organization")
	}
	c.orgID = data.Account.Organizations[0].ID
	return c.orgID, nil
}

func (c *Client) ListChannels() ([]Channel, error) {
	org, err := c.OrganizationID()
	if err != nil {
		return nil, err
	}
	var data struct {
		Channels []Channel `json:"channels"`
	}
	err = c.gql(
		`query($org: OrganizationId!) { channels(input: { organizationId: $org }) { id name displayName service isLocked } }`,
		map[string]any{"org": org},
		&data,
	)
	return data.Channels, err
}

func (c *Client) channelByServices(explicit string, cache *string, services ...string) (string, error) {
	c.mu.Lock()
	if id := strings.TrimSpace(explicit); id != "" {
		c.mu.Unlock()
		return id, nil
	}
	if cache != nil {
		if id := strings.TrimSpace(*cache); id != "" {
			c.mu.Unlock()
			return id, nil
		}
	}
	c.mu.Unlock()

	chs, err := c.ListChannels()
	if err != nil {
		return "", err
	}
	want := map[string]bool{}
	for _, s := range services {
		want[strings.ToLower(s)] = true
	}
	for _, ch := range chs {
		if want[strings.ToLower(ch.Service)] && !ch.IsLocked {
			c.mu.Lock()
			if cache != nil {
				*cache = ch.ID
			}
			c.mu.Unlock()
			return ch.ID, nil
		}
	}
	return "", fmt.Errorf("buffer: channel %s belum connect (atau locked)", strings.Join(services, "/"))
}

func (c *Client) TikTokChannelID() (string, error) {
	return c.channelByServices(c.tiktokChannelID, &c.resolvedTikTok, "tiktok")
}

func (c *Client) TwitterChannelID() (string, error) {
	return c.channelByServices(c.twitterChannelID, &c.resolvedX, "twitter", "x")
}

func (c *Client) createPost(input map[string]any, service, chID string) (*CreateResult, error) {
	var data struct {
		CreatePost json.RawMessage `json:"createPost"`
	}
	err := c.gql(`
mutation($input: CreatePostInput!) {
  createPost(input: $input) {
    ... on PostActionSuccess {
      post { id text status schedulingType }
    }
    ... on MutationError { message }
  }
}`, map[string]any{"input": input}, &data)
	if err != nil {
		return nil, err
	}
	var success struct {
		Post *struct {
			ID             string `json:"id"`
			Status         string `json:"status"`
			SchedulingType string `json:"schedulingType"`
		} `json:"post"`
	}
	if err := json.Unmarshal(data.CreatePost, &success); err == nil && success.Post != nil && success.Post.ID != "" {
		return &CreateResult{
			PostID:         success.Post.ID,
			Status:         success.Post.Status,
			SchedulingType: success.Post.SchedulingType,
			ChannelID:      chID,
			Service:        service,
		}, nil
	}
	var fail struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data.CreatePost, &fail); err == nil && fail.Message != "" {
		return nil, fmt.Errorf("buffer: %s", fail.Message)
	}
	return nil, fmt.Errorf("buffer: createPost gagal: %s", truncate(string(data.CreatePost), 240))
}

// QueueTikTokPhotos adds a photo carousel to Buffer with Notify Me (manual finish on phone).
func (c *Client) QueueTikTokPhotos(caption, title string, imageURLs []string) (*CreateResult, error) {
	urls := cleanURLs(imageURLs, 10)
	if len(urls) < 1 {
		return nil, fmt.Errorf("buffer: butuh minimal 1 gambar publik")
	}
	chID, err := c.TikTokChannelID()
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(caption)
	if text == "" {
		text = strings.TrimSpace(title)
	}
	if text == "" {
		text = " "
	}
	assets := make([]map[string]any, 0, len(urls))
	for _, u := range urls {
		assets = append(assets, map[string]any{
			"image": map[string]any{"url": u},
		})
	}
	input := map[string]any{
		"text":           text,
		"channelId":      chID,
		"schedulingType": "notification",
		"mode":           "addToQueue",
		"assets":         assets,
	}
	if t := strings.TrimSpace(title); t != "" {
		input["metadata"] = map[string]any{
			"tiktok": map[string]any{"title": clip(t, 90)},
		}
	}
	return c.createPost(input, "tiktok", chID)
}

// QueueTwitterThread publishes an X/Twitter thread immediately (shareNow).
// Same parts shape as Threads utas. X Premium long-form: parts dikirim utuh.
func (c *Client) QueueTwitterThread(parts []string) (*CreateResult, error) {
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	if len(cleaned) < 1 {
		return nil, fmt.Errorf("buffer: butuh minimal 1 bagian utas untuk X")
	}
	chID, err := c.TwitterChannelID()
	if err != nil {
		return nil, err
	}
	thread := make([]map[string]any, 0, len(cleaned))
	for _, p := range cleaned {
		thread = append(thread, map[string]any{"text": p})
	}
	input := map[string]any{
		"text":           cleaned[0],
		"channelId":      chID,
		"schedulingType": "automatic",
		"mode":           "shareNow", // publish sekarang — tidak masuk antrian jadwal
		"metadata": map[string]any{
			"twitter": map[string]any{
				"thread": thread,
			},
		},
	}
	return c.createPost(input, "twitter", chID)
}

func cleanURLs(in []string, max int) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, u := range in {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			continue
		}
		seen[u] = true
		out = append(out, u)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
