package repliz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	BaseURL   string
	AccessKey string
	SecretKey string
	HTTP      *http.Client
}

type Account struct {
	ID          string `json:"id"`
	OID         string `json:"_id"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	Picture     string `json:"picture"`
	Type        string `json:"type"`
	IsConnected bool   `json:"isConnected"`
}

func (a Account) AccountID() string {
	if strings.TrimSpace(a.ID) != "" {
		return a.ID
	}
	return strings.TrimSpace(a.OID)
}

type Media struct {
	Alt             string `json:"alt"`
	CustomThumbnail bool   `json:"customThumbnail"`
	Type            string `json:"type"`
	Thumbnail       string `json:"thumbnail,omitempty"`
	URL             string `json:"url"`
}

// FlexString accepts a JSON string or number (unix seconds / ms).
type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	*f = FlexString(string(b))
	return nil
}

func (f FlexString) String() string { return string(f) }

type Post struct {
	ID          string     `json:"id"`
	OID         string     `json:"_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Topic       string     `json:"topic"`
	Type        string     `json:"type"`
	CreatedAt   FlexString `json:"createdAt"`
	PublishedAt FlexString `json:"publishedAt,omitempty"`
	URL         string     `json:"url,omitempty"`
	Medias      []Media    `json:"medias,omitempty"`
}

func (p Post) CreatedStamp() string {
	if s := strings.TrimSpace(p.CreatedAt.String()); s != "" {
		return s
	}
	return strings.TrimSpace(p.PublishedAt.String())
}

func (p Post) PostID() string {
	if strings.TrimSpace(p.ID) != "" {
		return strings.TrimSpace(p.ID)
	}
	return strings.TrimSpace(p.OID)
}

func NewFromEnv() *Client {
	return &Client{
		BaseURL:   NormalizeBase(os.Getenv("REPLIZ_BASE_URL")),
		AccessKey: strings.TrimSpace(os.Getenv("REPLIZ_ACCESS_KEY")),
		SecretKey: strings.TrimSpace(os.Getenv("REPLIZ_SECRET_KEY")),
	}
}

func (c *Client) Ready() bool {
	return c != nil && strings.TrimSpace(c.AccessKey) != "" && strings.TrimSpace(c.SecretKey) != ""
}

func NormalizeBase(raw string) string {
	b := strings.TrimRight(strings.TrimSpace(raw), "/")
	for {
		lower := strings.ToLower(b)
		if !strings.HasSuffix(lower, "/public") {
			break
		}
		b = strings.TrimRight(b[:len(b)-len("/public")], "/")
	}
	if b == "" {
		return "https://api.repliz.com"
	}
	return b
}

func (c *Client) base() string {
	if c == nil {
		return "https://api.repliz.com"
	}
	return NormalizeBase(c.BaseURL)
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(c.AccessKey, c.SecretKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		msg := strings.TrimSpace(string(raw))
		var er struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &er) == nil && er.Message != "" {
			msg = er.Message
		}
		if msg == "" {
			msg = res.Status
		}
		return raw, res.StatusCode, fmt.Errorf("repliz %s: %s", res.Status, msg)
	}
	return raw, res.StatusCode, nil
}

func (c *Client) ListAccounts(ctx context.Context) ([]Account, error) {
	q := url.Values{}
	q.Set("page", "1")
	q.Set("limit", "50")
	raw, _, err := c.do(ctx, http.MethodGet, "/public/account?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Docs []Account `json:"docs"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("repliz account JSON: %w", err)
	}
	for i := range out.Docs {
		if out.Docs[i].ID == "" {
			out.Docs[i].ID = out.Docs[i].AccountID()
		}
	}
	return out.Docs, nil
}

func (c *Client) GetAccount(ctx context.Context, accountID string) (Account, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return Account{}, fmt.Errorf("account id kosong")
	}
	raw, _, err := c.do(ctx, http.MethodGet, "/public/account/"+url.PathEscape(accountID), nil)
	if err != nil {
		return Account{}, err
	}
	var a Account
	if err := json.Unmarshal(raw, &a); err != nil {
		return Account{}, fmt.Errorf("repliz akun JSON: %w", err)
	}
	if a.ID == "" {
		a.ID = a.AccountID()
	}
	return a, nil
}

func (c *Client) GetAccountStatistic(ctx context.Context, accountID string) (map[string]any, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id kosong")
	}
	raw, _, err := c.do(ctx, http.MethodGet, "/public/account/"+url.PathEscape(accountID)+"/statistic", nil)
	if err != nil {
		return nil, err
	}
	return compactStatistic(raw), nil
}

func (c *Client) ListContentUpTo(ctx context.Context, accountID string, limit int) ([]Post, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id kosong")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 40 {
		limit = 40
	}
	all := make([]Post, 0, limit)
	seen := map[string]bool{}
	token := ""
	for page := 0; page < 3 && len(all) < limit; page++ {
		posts, next, err := c.listContentPage(ctx, accountID, token)
		if err != nil {
			if len(all) == 0 {
				return nil, err
			}
			return all, nil
		}
		added := 0
		for _, p := range posts {
			id := p.PostID()
			if id != "" && seen[id] {
				continue
			}
			if id != "" {
				seen[id] = true
			}
			all = append(all, p)
			added++
			if len(all) >= limit {
				break
			}
		}
		if next == "" || added == 0 {
			break
		}
		token = next
	}
	return all, nil
}

func (c *Client) listContentPage(ctx context.Context, accountID, token string) ([]Post, string, error) {
	q := url.Values{}
	q.Set("accountId", accountID)
	q.Set("type", "media")
	if strings.TrimSpace(token) != "" {
		q.Set("nextToken", token)
	}
	raw, _, err := c.do(ctx, http.MethodGet, "/public/content?"+q.Encode(), nil)
	if err != nil {
		return nil, "", err
	}
	var out struct {
		Docs      []Post `json:"docs"`
		NextToken string `json:"nextToken"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, "", fmt.Errorf("repliz konten JSON: %w", err)
	}
	for i := range out.Docs {
		if out.Docs[i].ID == "" {
			out.Docs[i].ID = out.Docs[i].PostID()
		}
		out.Docs[i].Title = clip(out.Docs[i].Title, 180)
		out.Docs[i].Description = clip(out.Docs[i].Description, 320)
	}
	return out.Docs, strings.TrimSpace(out.NextToken), nil
}

func (c *Client) GetContentStatistic(ctx context.Context, contentID, accountID string) (map[string]float64, error) {
	contentID = strings.TrimSpace(contentID)
	accountID = strings.TrimSpace(accountID)
	if contentID == "" || accountID == "" {
		return nil, fmt.Errorf("content id kosong")
	}
	q := url.Values{}
	q.Set("accountId", accountID)
	raw, status, err := c.do(ctx, http.MethodGet, "/public/content/"+url.PathEscape(contentID)+"/statistic?"+q.Encode(), nil)
	if err != nil {
		if status == http.StatusNotFound {
			return map[string]float64{}, nil
		}
		return nil, err
	}
	return compactPostStats(raw), nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
