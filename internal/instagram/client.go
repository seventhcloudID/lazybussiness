package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	BaseURL    = "https://graph.instagram.com"
	APIVersion = "v22.0"
)

type Client struct {
	mu        sync.RWMutex
	token     string
	http      *http.Client
	userID    string
	tokenPath string
}

func New() *Client {
	return NewWithTokenPath(filepath.Join(".data", "ig_access_token"))
}

func NewWithTokenPath(tokenPath string) *Client {
	c := &Client{
		http:      &http.Client{Timeout: 120 * time.Second},
		tokenPath: tokenPath,
	}
	c.loadTokenFile()
	return c
}

func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = strings.TrimSpace(token)
	c.userID = ""
	c.persistTokenLocked()
}

func (c *Client) ClearToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
	c.userID = ""
	if c.tokenPath != "" {
		_ = os.Remove(c.tokenPath)
	}
}

func (c *Client) persistTokenLocked() {
	if c.tokenPath == "" {
		return
	}
	if c.token == "" {
		_ = os.Remove(c.tokenPath)
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.tokenPath), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(c.tokenPath, []byte(c.token), 0o600)
}

func (c *Client) loadTokenFile() {
	if c.tokenPath == "" {
		return
	}
	b, err := os.ReadFile(c.tokenPath)
	if err != nil {
		return
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return
	}
	c.token = tok
}

func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *Client) Connected() bool {
	return c.Token() != ""
}

func (c *Client) UserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

func (c *Client) setUserID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userID = id
}

// SetUserID menyimpan IG user id dari OAuth (tanpa panggil /me).
func (c *Client) SetUserID(id string) {
	c.setUserID(strings.TrimSpace(id))
}

type APIError struct {
	Status int
	Body   json.RawMessage
}

func (e *APIError) Error() string {
	if e != nil && isUnsupportedGET(e.Body) {
		return "instagram: akun belum bisa diakses API (butuh akun Professional/Business atau Creator, dan di mode Development akun harus jadi Tester + accept invite Meta). Error mentah: Unsupported request - method type: get"
	}
	return fmt.Sprintf("instagram api: status %d: %s", e.Status, string(e.Body))
}

func isUnsupportedGET(body json.RawMessage) bool {
	s := strings.ToLower(string(body))
	return strings.Contains(s, "unsupported request") && strings.Contains(s, "method type")
}

func (c *Client) Do(method, path string, query url.Values, form url.Values) (json.RawMessage, error) {
	token := c.Token()
	if token == "" {
		return nil, fmt.Errorf("token Instagram belum di-set")
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Versioned Graph path, kecuali endpoint token exchange di root.
	fullPath := path
	if path != "/access_token" && path != "/refresh_access_token" && !strings.HasPrefix(path, "/"+APIVersion+"/") {
		fullPath = "/" + APIVersion + path
	}

	u, err := url.Parse(BaseURL + fullPath)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for k, vs := range query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("access_token", token)
	u.RawQuery = q.Encode()

	var body io.Reader
	contentType := ""
	if form != nil {
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, &APIError{Status: res.StatusCode, Body: raw}
	}
	return raw, nil
}

func (c *Client) GetMe() (json.RawMessage, error) {
	// Prefer explicit user id (lebih andal di Instagram Login API daripada /me).
	if id := strings.TrimSpace(c.UserID()); id != "" {
		if raw, err := c.GetUser(id); err == nil {
			return raw, nil
		}
	}
	raw, err := c.Do(http.MethodGet, "/me", url.Values{
		"fields": {"id,username,account_type"},
	}, nil)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && isUnsupportedGET(apiErr.Body) {
			// Retry field lebih minimal — kadang field ekstra memicu error Meta.
			raw2, err2 := c.Do(http.MethodGet, "/me", url.Values{"fields": {"id,username"}}, nil)
			if err2 == nil {
				var me struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(raw2, &me) == nil && me.ID != "" {
					c.setUserID(me.ID)
				}
				return raw2, nil
			}
		}
		return nil, err
	}
	var me struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &me) == nil && me.ID != "" {
		c.setUserID(me.ID)
	}
	return raw, nil
}

func (c *Client) GetUser(id string) (json.RawMessage, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("user id kosong")
	}
	raw, err := c.Do(http.MethodGet, "/"+id, url.Values{
		"fields": {"id,username,account_type"},
	}, nil)
	if err != nil {
		return nil, err
	}
	c.setUserID(id)
	return raw, nil
}

func (c *Client) GetMedia(limit string) (json.RawMessage, error) {
	if limit == "" {
		limit = "25"
	}
	return c.Do(http.MethodGet, "/me/media", url.Values{
		"fields": {"id,caption,media_type,media_url,permalink,thumbnail_url,timestamp,like_count,comments_count,username"},
		"limit":  {limit},
	}, nil)
}

// MediaDetail is a single IG media node (feed / carousel parent).
type MediaDetail struct {
	ID           string `json:"id"`
	Caption      string `json:"caption"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Permalink    string `json:"permalink"`
	Timestamp    string `json:"timestamp"`
	Children     *struct {
		Data []MediaDetail `json:"data"`
	} `json:"children"`
}

func (c *Client) GetMediaByID(id string) (*MediaDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("media_id wajib")
	}
	raw, err := c.Do(http.MethodGet, "/"+id, url.Values{
		"fields": {"id,caption,media_type,media_url,thumbnail_url,permalink,timestamp,like_count,comments_count,children{id,media_type,media_url,thumbnail_url}"},
	}, nil)
	if err != nil {
		return nil, err
	}
	var m MediaDetail
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, fmt.Errorf("media tidak ditemukan")
	}
	return &m, nil
}

// MediaMetrics ringkasan metrik satu post IG (untuk Lazy Track).
type MediaMetrics struct {
	Views   float64
	Likes   float64
	Replies float64 // comments
	Reach   float64
	Raw     map[string]float64
}

// GetMediaMetrics gabungkan field media + insights (views/impressions/reach).
func (c *Client) GetMediaMetrics(mediaID string) (*MediaMetrics, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return nil, fmt.Errorf("media_id wajib")
	}
	out := &MediaMetrics{Raw: map[string]float64{}}

	// Baseline dari node media (selalu tersedia tanpa insight permission khusus).
	rawNode, err := c.Do(http.MethodGet, "/"+mediaID, url.Values{
		"fields": {"id,like_count,comments_count"},
	}, nil)
	if err != nil {
		return nil, err
	}
	var node struct {
		ID            string  `json:"id"`
		LikeCount     float64 `json:"like_count"`
		CommentsCount float64 `json:"comments_count"`
	}
	_ = json.Unmarshal(rawNode, &node)
	out.Likes = node.LikeCount
	out.Replies = node.CommentsCount
	out.Raw["like_count"] = node.LikeCount
	out.Raw["comments_count"] = node.CommentsCount

	// Insights: coba set modern dulu, lalu fallback legacy.
	for _, metrics := range []string{
		"views,reach,likes,comments,saved,total_interactions",
		"impressions,reach,engagement",
		"views,reach",
	} {
		raw, ierr := c.Do(http.MethodGet, "/"+mediaID+"/insights", url.Values{
			"metric": {metrics},
		}, nil)
		if ierr != nil {
			continue
		}
		parsed := parseInsightValues(raw)
		if len(parsed) == 0 {
			continue
		}
		for k, v := range parsed {
			out.Raw[k] = v
		}
		break
	}

	if v, ok := out.Raw["views"]; ok && v > 0 {
		out.Views = v
	} else if v, ok := out.Raw["impressions"]; ok && v > 0 {
		out.Views = v
	} else if v, ok := out.Raw["reach"]; ok && v > 0 {
		out.Views = v
	}
	if v, ok := out.Raw["likes"]; ok && v > out.Likes {
		out.Likes = v
	}
	if v, ok := out.Raw["comments"]; ok && v > out.Replies {
		out.Replies = v
	}
	if v, ok := out.Raw["reach"]; ok {
		out.Reach = v
	}
	return out, nil
}

func parseInsightValues(raw json.RawMessage) map[string]float64 {
	var wrap struct {
		Data []struct {
			Name   string `json:"name"`
			Values []struct {
				Value any `json:"value"`
			} `json:"values"`
			TotalValue *struct {
				Value any `json:"value"`
			} `json:"total_value"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return nil
	}
	out := map[string]float64{}
	for _, d := range wrap.Data {
		var val float64
		if d.TotalValue != nil {
			val = anyFloat(d.TotalValue.Value)
		} else if len(d.Values) > 0 {
			val = anyFloat(d.Values[0].Value)
		}
		out[d.Name] = val
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func anyFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}

// ImageURLsFromMedia collects photo URLs for Buffer (IMAGE / CAROUSEL album children).
// VIDEO / REELS → error (Buffer TikTok path kita = photo carousel).
func ImageURLsFromMedia(m *MediaDetail) ([]string, error) {
	if m == nil {
		return nil, fmt.Errorf("media kosong")
	}
	t := strings.ToUpper(strings.TrimSpace(m.MediaType))
	switch t {
	case "IMAGE":
		u := strings.TrimSpace(m.MediaURL)
		if u == "" {
			u = strings.TrimSpace(m.ThumbnailURL)
		}
		if u == "" {
			return nil, fmt.Errorf("media tidak punya URL gambar")
		}
		return []string{u}, nil
	case "CAROUSEL_ALBUM":
		if m.Children == nil || len(m.Children.Data) == 0 {
			return nil, fmt.Errorf("carousel tanpa children")
		}
		out := make([]string, 0, len(m.Children.Data))
		for _, ch := range m.Children.Data {
			ct := strings.ToUpper(strings.TrimSpace(ch.MediaType))
			if ct != "" && ct != "IMAGE" {
				continue // skip video slides in mixed carousel
			}
			u := strings.TrimSpace(ch.MediaURL)
			if u == "" {
				u = strings.TrimSpace(ch.ThumbnailURL)
			}
			if u != "" {
				out = append(out, u)
			}
			if len(out) >= 10 {
				break
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("carousel tidak punya slide foto")
		}
		return out, nil
	default:
		return nil, fmt.Errorf("hanya IMAGE atau CAROUSEL yang bisa dikirim ke Buffer TikTok (bukan %s)", t)
	}
}

// RefreshToken memperpanjang long-lived Instagram user token (~60 hari).
func (c *Client) RefreshToken() (json.RawMessage, error) {
	token := c.Token()
	if token == "" {
		return nil, fmt.Errorf("token Instagram belum di-set")
	}
	u := BaseURL + "/refresh_access_token?" + url.Values{
		"grant_type":   {"ig_refresh_token"},
		"access_token": {token},
	}.Encode()

	res, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, &APIError{Status: res.StatusCode, Body: raw}
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(raw, &out) == nil && out.AccessToken != "" {
		c.SetToken(out.AccessToken)
	}
	return raw, nil
}

// ExchangeLongLived menukar short-lived token jadi long-lived (butuh INSTAGRAM_APP_SECRET).
func (c *Client) ExchangeLongLived(shortToken, appSecret string) (json.RawMessage, error) {
	shortToken = strings.TrimSpace(shortToken)
	appSecret = strings.TrimSpace(appSecret)
	if shortToken == "" || appSecret == "" {
		return nil, fmt.Errorf("short token dan app secret wajib")
	}
	u := BaseURL + "/access_token?" + url.Values{
		"grant_type":    {"ig_exchange_token"},
		"client_secret": {appSecret},
		"access_token":  {shortToken},
	}.Encode()

	res, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, &APIError{Status: res.StatusCode, Body: raw}
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(raw, &out) == nil && out.AccessToken != "" {
		c.SetToken(out.AccessToken)
	}
	return raw, nil
}

func (c *Client) EnsureUserID() (string, error) {
	if id := c.UserID(); id != "" {
		return id, nil
	}
	if _, err := c.GetMe(); err != nil {
		return "", err
	}
	id := c.UserID()
	if id == "" {
		return "", fmt.Errorf("tidak bisa baca Instagram user id")
	}
	return id, nil
}

type containerID struct {
	ID string `json:"id"`
}

// CreateCarouselItem membuat child image container (is_carousel_item=true).
func (c *Client) CreateCarouselItem(imageURL string) (string, error) {
	userID, err := c.EnsureUserID()
	if err != nil {
		return "", err
	}
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return "", fmt.Errorf("image_url wajib")
	}
	raw, err := c.Do(http.MethodPost, "/"+userID+"/media", nil, url.Values{
		"image_url":        {imageURL},
		"is_carousel_item": {"true"},
	})
	if err != nil {
		return "", err
	}
	var out containerID
	if err := json.Unmarshal(raw, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("gagal buat carousel item: %s", string(raw))
	}
	return out.ID, nil
}

// CreateCarouselContainer membuat parent CAROUSEL dari child container IDs.
func (c *Client) CreateCarouselContainer(childIDs []string, caption string) (string, error) {
	userID, err := c.EnsureUserID()
	if err != nil {
		return "", err
	}
	if len(childIDs) < 2 {
		return "", fmt.Errorf("carousel minimal 2 gambar")
	}
	if len(childIDs) > 10 {
		return "", fmt.Errorf("carousel maksimal 10 gambar")
	}
	form := url.Values{
		"media_type": {"CAROUSEL"},
		"children":   {strings.Join(childIDs, ",")},
	}
	if strings.TrimSpace(caption) != "" {
		form.Set("caption", caption)
	}
	raw, err := c.Do(http.MethodPost, "/"+userID+"/media", nil, form)
	if err != nil {
		return "", err
	}
	var out containerID
	if err := json.Unmarshal(raw, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("gagal buat carousel container: %s", string(raw))
	}
	return out.ID, nil
}

// GetContainerStatus status container IG (EXPIRED / ERROR / FINISHED / IN_PROGRESS / PUBLISHED).
func (c *Client) GetContainerStatus(creationID string) (string, json.RawMessage, error) {
	creationID = strings.TrimSpace(creationID)
	if creationID == "" {
		return "", nil, fmt.Errorf("creation_id wajib")
	}
	raw, err := c.Do(http.MethodGet, "/"+creationID, url.Values{
		"fields": {"status_code,status"},
	}, nil)
	if err != nil {
		return "", nil, err
	}
	var out struct {
		StatusCode string `json:"status_code"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.StatusCode, raw, nil
}

// WaitContainer menunggu container siap publish.
func (c *Client) WaitContainer(creationID string, maxWait time.Duration) error {
	if maxWait <= 0 {
		maxWait = 90 * time.Second
	}
	deadline := time.Now().Add(maxWait)
	for {
		code, raw, err := c.GetContainerStatus(creationID)
		if err != nil {
			return err
		}
		switch strings.ToUpper(code) {
		case "FINISHED", "PUBLISHED":
			return nil
		case "ERROR", "EXPIRED":
			return fmt.Errorf("container gagal (%s): %s", code, string(raw))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout menunggu container (status=%s)", code)
		}
		time.Sleep(2 * time.Second)
	}
}

// PublishMedia publish creation_id ke feed.
func (c *Client) PublishMedia(creationID string) (json.RawMessage, error) {
	userID, err := c.EnsureUserID()
	if err != nil {
		return nil, err
	}
	creationID = strings.TrimSpace(creationID)
	if creationID == "" {
		return nil, fmt.Errorf("creation_id wajib")
	}
	if err := c.WaitContainer(creationID, 90*time.Second); err != nil {
		return nil, err
	}
	return c.Do(http.MethodPost, "/"+userID+"/media_publish", nil, url.Values{
		"creation_id": {creationID},
	})
}

// PublishCarousel membuat carousel dari daftar image URL publik + caption.
func (c *Client) PublishCarousel(imageURLs []string, caption string) (map[string]any, error) {
	var cleaned []string
	for _, u := range imageURLs {
		u = strings.TrimSpace(u)
		if u != "" {
			cleaned = append(cleaned, u)
		}
	}
	if len(cleaned) < 2 {
		return nil, fmt.Errorf("minimal 2 image_url untuk carousel")
	}
	if len(cleaned) > 10 {
		return nil, fmt.Errorf("maksimal 10 slide")
	}

	childIDs := make([]string, 0, len(cleaned))
	for i, img := range cleaned {
		id, err := c.CreateCarouselItem(img)
		if err != nil {
			return nil, fmt.Errorf("slide %d: %w", i+1, err)
		}
		childIDs = append(childIDs, id)
		if err := c.WaitContainer(id, 60*time.Second); err != nil {
			return nil, fmt.Errorf("slide %d belum siap: %w", i+1, err)
		}
	}

	parentID, err := c.CreateCarouselContainer(childIDs, caption)
	if err != nil {
		return nil, err
	}
	pub, err := c.PublishMedia(parentID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"children":  childIDs,
		"container": parentID,
		"published": json.RawMessage(pub),
	}, nil
}
