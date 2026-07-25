package ai

import (
	"fmt"
	"strings"
)

type ChatMessage struct {
	Role string `json:"role"` // user | model | assistant
	Text string `json:"text"`
}

type ChatRequest struct {
	Messages  []ChatMessage `json:"messages"`
	System    string        `json:"system"` // opsional; kosong = tanpa systemInstruction
	UseSearch bool          `json:"use_search"`
}

type ChatResult struct {
	Reply  string       `json:"reply"`
	Model  string       `json:"model"`
	Search bool         `json:"search"`
	Usage  *TokenUsage  `json:"usage,omitempty"`
	Quota  *QuotaStatus `json:"quota,omitempty"`
}

// Chat is a thin Gemini chat proxy — no forced persona / language / format.
func (c *Client) Chat(req ChatRequest) (*ChatResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("AI belum dikonfigurasi — set AI_API_KEY")
	}
	if c.provider != "gemini" && c.provider != "google" {
		return nil, fmt.Errorf("chat UI saat ini hanya untuk AI_PROVIDER=gemini")
	}

	contents := buildGeminiChatContents(req.Messages)
	if len(contents) == 0 {
		return nil, fmt.Errorf("kirim minimal 1 pesan")
	}

	model := c.model
	body := map[string]any{
		"contents": contents,
		"generationConfig": map[string]any{
			"temperature":     1,
			"maxOutputTokens": 8192,
		},
	}
	if system := strings.TrimSpace(req.System); system != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]string{{"text": system}},
		}
	}
	if req.UseSearch {
		model = c.searchModel
		if strings.TrimSpace(model) == "" {
			model = c.model
		}
		body["tools"] = []map[string]any{
			{"google_search": map[string]any{}},
		}
	}

	content, usage, err := c.chatGeminiRequest(model, body)
	if err != nil {
		return nil, err
	}
	if c.quota != nil {
		c.quota.record(usage)
	}

	out := &ChatResult{
		Reply:  content,
		Model:  model,
		Search: req.UseSearch,
		Usage:  usage,
	}
	if c.quota != nil {
		q := c.quota.status(c.provider, model)
		out.Quota = &q
	}
	return out, nil
}

func buildGeminiChatContents(msgs []ChatMessage) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		text := strings.TrimSpace(m.Text)
		if text == "" {
			continue
		}
		// Skip UI error bubbles from being replayed to the model.
		if strings.HasPrefix(text, "⚠️") {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(m.Role))
		switch role {
		case "assistant", "model", "gemini":
			role = "model"
		case "user", "human":
			role = "user"
		case "system":
			continue
		default:
			role = "user"
		}
		if len(out) > 0 && out[len(out)-1]["role"] == role {
			prevParts, _ := out[len(out)-1]["parts"].([]map[string]string)
			if len(prevParts) > 0 {
				prevParts[0]["text"] = prevParts[0]["text"] + "\n\n" + text
				out[len(out)-1]["parts"] = prevParts
				continue
			}
		}
		out = append(out, map[string]any{
			"role":  role,
			"parts": []map[string]string{{"text": text}},
		})
	}
	if len(out) > 0 && out[len(out)-1]["role"] != "user" {
		return nil
	}
	return out
}
