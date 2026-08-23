package repliz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func NormalizePlatform(p string) (string, error) {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "instagram", "threads", "tiktok", "twitter", "shopee", "facebook", "youtube", "linkedin":
		return p, nil
	default:
		return "", fmt.Errorf("platform tidak didukung")
	}
}

func (c *Client) AuthorizeURL(ctx context.Context, platform, redirect string) (string, error) {
	platform, err := NormalizePlatform(platform)
	if err != nil {
		return "", err
	}
	redirect = strings.TrimSpace(redirect)
	if redirect == "" {
		return "", fmt.Errorf("redirect OAuth wajib")
	}
	q := url.Values{}
	q.Set("redirect", redirect)
	raw, _, err := c.do(ctx, http.MethodGet, "/public/account/"+platform+"/authorize?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || strings.TrimSpace(out.URL) == "" {
		return "", fmt.Errorf("repliz authorize tanpa url")
	}
	return out.URL, nil
}

func (c *Client) ConnectCode(ctx context.Context, platform, code, accountID string) (string, error) {
	platform, err := NormalizePlatform(platform)
	if err != nil {
		return "", err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("kode OAuth kosong")
	}
	path := "/public/account/" + platform + "/connect"
	if id := strings.TrimSpace(accountID); id != "" {
		path += "/" + url.PathEscape(id)
	}
	raw, _, err := c.do(ctx, http.MethodPost, path, map[string]any{"code": code})
	if err != nil {
		return "", err
	}
	return parseAccountID(raw, accountID), nil
}

func (c *Client) DeleteAccount(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("account id kosong")
	}
	_, _, err := c.do(ctx, http.MethodDelete, "/public/account/"+url.PathEscape(accountID), nil)
	return err
}

func parseAccountID(raw []byte, fallback string) string {
	var out struct {
		AccountID string `json:"accountId"`
		ID        string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	if strings.TrimSpace(out.AccountID) != "" {
		return out.AccountID
	}
	if strings.TrimSpace(out.ID) != "" {
		return out.ID
	}
	return strings.TrimSpace(fallback)
}
