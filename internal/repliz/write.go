package repliz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ScheduleReq struct {
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	Topic          string           `json:"topic,omitempty"`
	Type           string           `json:"type"`
	Medias         []Media          `json:"medias"`
	AccountID      string           `json:"accountId"`
	ScheduleAt     string           `json:"scheduleAt"`
	AdditionalInfo map[string]any   `json:"additionalInfo,omitempty"`
	Replies        []map[string]any `json:"replies"`
}

func (c *Client) CreateSchedule(ctx context.Context, in ScheduleReq) (string, error) {
	if !c.Ready() {
		return "", fmt.Errorf("Repliz belum disambungkan")
	}
	in.AccountID = strings.TrimSpace(in.AccountID)
	if in.AccountID == "" {
		return "", fmt.Errorf("account id Repliz wajib")
	}
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))
	if in.Type == "" {
		in.Type = "text"
	}
	if in.Medias == nil {
		in.Medias = []Media{}
	}
	if in.Replies == nil {
		in.Replies = []map[string]any{}
	}
	if strings.TrimSpace(in.ScheduleAt) == "" {
		in.ScheduleAt = time.Now().UTC().Add(20 * time.Second).Format("2006-01-02T15:04:05.000Z")
	}
	raw, _, err := c.do(ctx, http.MethodPost, "/public/schedule", in)
	if err != nil {
		return "", err
	}
	var out struct {
		ScheduleID string `json:"scheduleId"`
		ID         string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	id := strings.TrimSpace(out.ScheduleID)
	if id == "" {
		id = strings.TrimSpace(out.ID)
	}
	if id == "" {
		return "", fmt.Errorf("Repliz jadwal tanpa id")
	}
	return id, nil
}

func (c *Client) UploadBytes(ctx context.Context, filename, mime string, data []byte) (string, error) {
	if !c.Ready() {
		return "", fmt.Errorf("Repliz belum disambungkan")
	}
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	initBody := map[string]any{
		"filename": filename,
		"size":     len(data),
		"mimetype": mime,
	}
	raw, _, err := c.do(ctx, http.MethodPost, "/public/storage/file/init", initBody)
	if err != nil {
		return "", err
	}
	var init struct {
		ID     string `json:"id"`
		Upload string `json:"upload"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(raw, &init); err != nil {
		return "", fmt.Errorf("repliz init JSON: %w", err)
	}
	if init.Upload == "" || init.ID == "" {
		return "", fmt.Errorf("repliz init tidak mengembalikan upload URL")
	}
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, init.Upload, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	putReq.Header.Set("Content-Type", mime)
	putRes, err := c.client().Do(putReq)
	if err != nil {
		return "", fmt.Errorf("unggah file: %w", err)
	}
	io.Copy(io.Discard, putRes.Body)
	putRes.Body.Close()
	if putRes.StatusCode >= 400 {
		return "", fmt.Errorf("unggah file gagal: %s", putRes.Status)
	}
	if _, _, err := c.do(ctx, http.MethodPost, "/public/storage/file/"+init.ID+"/complete", nil); err != nil {
		return "", err
	}
	if strings.TrimSpace(init.URL) == "" {
		return "", fmt.Errorf("repliz tidak mengembalikan URL publik")
	}
	return init.URL, nil
}

type CommentOwner struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Picture  string `json:"picture"`
}

type Comment struct {
	ID        string       `json:"id"`
	CommentID string       `json:"commentId"`
	Text      string       `json:"text"`
	Message   string       `json:"message"`
	Username  string       `json:"username"`
	Author    string       `json:"author"`
	Name      string       `json:"name"`
	Owner     CommentOwner `json:"owner"`
	CreatedAt string       `json:"createdAt"`
	Timestamp string       `json:"timestamp"`
	ParentID  string       `json:"parentId"`
	CommentOf string       `json:"commentIdParent"`
	IsOwner   bool         `json:"isOwner"`
	FromMe    bool         `json:"fromMe"`
}

func (c Comment) CID() string {
	if s := strings.TrimSpace(c.ID); s != "" {
		return s
	}
	return strings.TrimSpace(c.CommentID)
}

func (c Comment) Body() string {
	if s := strings.TrimSpace(c.Text); s != "" {
		return s
	}
	return strings.TrimSpace(c.Message)
}

func (c Comment) User() string {
	for _, s := range []string{c.Username, c.Author, c.Name, c.Owner.Username, c.Owner.Name} {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func (c Comment) Picture() string {
	return strings.TrimSpace(c.Owner.Picture)
}

func (c *Client) ListComments(ctx context.Context, contentID, accountID string) ([]Comment, error) {
	contentID = strings.TrimSpace(contentID)
	accountID = strings.TrimSpace(accountID)
	if contentID == "" || accountID == "" {
		return nil, fmt.Errorf("content id wajib")
	}
	q := url.Values{}
	q.Set("accountId", accountID)
	raw, _, err := c.do(ctx, http.MethodGet, "/public/content/"+url.PathEscape(contentID)+"/comment?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Docs []Comment `json:"docs"`
		Data []Comment `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, fmt.Errorf("repliz komentar JSON: %w", err)
	}
	list := wrap.Docs
	if len(list) == 0 {
		list = wrap.Data
	}
	if len(list) == 0 {
		var arr []Comment
		if json.Unmarshal(raw, &arr) == nil {
			list = arr
		}
	}
	return list, nil
}

func (c *Client) CreateComment(ctx context.Context, contentID, accountID, text, parentCommentID string) (string, error) {
	contentID = strings.TrimSpace(contentID)
	accountID = strings.TrimSpace(accountID)
	text = strings.TrimSpace(text)
	if contentID == "" || accountID == "" || text == "" {
		return "", fmt.Errorf("konten, akun, dan teks wajib")
	}
	body := map[string]any{
		"accountId": accountID,
		"text":      text,
	}
	if p := strings.TrimSpace(parentCommentID); p != "" {
		body["commentId"] = p
	}
	raw, _, err := c.do(ctx, http.MethodPost, "/public/content/"+url.PathEscape(contentID)+"/comment", body)
	if err != nil {
		return "", err
	}
	var out struct {
		CommentID string `json:"commentId"`
		ID        string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	id := strings.TrimSpace(out.CommentID)
	if id == "" {
		id = strings.TrimSpace(out.ID)
	}
	if id == "" {
		return "", fmt.Errorf("Repliz komentar tanpa id")
	}
	return id, nil
}

func (c *Client) DeleteComment(ctx context.Context, contentID, commentID, accountID string) error {
	contentID = strings.TrimSpace(contentID)
	commentID = strings.TrimSpace(commentID)
	accountID = strings.TrimSpace(accountID)
	if contentID == "" || commentID == "" {
		return fmt.Errorf("content/comment id wajib")
	}
	q := url.Values{}
	if accountID != "" {
		q.Set("accountId", accountID)
	}
	path := "/public/content/" + url.PathEscape(contentID) + "/comment/" + url.PathEscape(commentID)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	_, _, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *Client) RepliesAsFeed(ctx context.Context, contentID, accountID, meUser string) (json.RawMessage, error) {
	list, err := c.ListComments(ctx, contentID, accountID)
	if err != nil {
		return nil, err
	}
	meUser = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(meUser), "@"))
	rows := make([]map[string]any, 0, len(list))
	for _, cm := range list {
		id := cm.CID()
		if id == "" {
			continue
		}
		user := cm.User()
		mine := cm.IsOwner || cm.FromMe
		if !mine && meUser != "" && strings.ToLower(strings.TrimPrefix(user, "@")) == meUser {
			mine = true
		}
		stamp := strings.TrimSpace(cm.Timestamp)
		if stamp == "" {
			stamp = strings.TrimSpace(cm.CreatedAt)
		}
		rows = append(rows, map[string]any{
			"id":        id,
			"text":      cm.Body(),
			"username":  user,
			"picture":   cm.Picture(),
			"timestamp": stamp,
			"is_mine":   mine,
			"answered":  false,
			"children":  []any{},
		})
	}
	return json.Marshal(map[string]any{
		"data":   rows,
		"source": "repliz",
	})
}

func MediaTypeFromKind(kind, imageURL, videoURL string) string {
	kind = strings.ToUpper(strings.TrimSpace(kind))
	switch kind {
	case "IMAGE", "CAROUSEL_ALBUM", "CAROUSEL":
		if imageURL != "" {
			return "image"
		}
	case "VIDEO", "REEL":
		if videoURL != "" {
			return "video"
		}
	case "ALBUM":
		return "album"
	}
	if videoURL != "" {
		return "video"
	}
	if imageURL != "" {
		return "image"
	}
	return "text"
}

func ReplyParts(parts []string) []map[string]any {
	out := make([]map[string]any, 0)
	for i, p := range parts {
		if i == 0 {
			continue
		}
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, map[string]any{
			"title":       "",
			"description": p,
			"type":        "text",
			"medias":      []any{},
		})
	}
	return out
}
