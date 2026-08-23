package ai

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type AccountBriefInput struct {
	Handle   string
	Name     string
	IGHandle string
	Memory   Memory
	Snapshot map[string]any
}

func FormatAccountBrief(in AccountBriefInput) string {
	var b strings.Builder
	b.WriteString("## Connected account (real data — do not invent)\n")
	if h := strings.TrimSpace(in.Handle); h != "" {
		if !strings.HasPrefix(h, "@") {
			h = "@" + h
		}
		b.WriteString("Threads: " + h + "\n")
	}
	if n := strings.TrimSpace(in.Name); n != "" {
		b.WriteString("Name: " + n + "\n")
	}
	if ig := strings.TrimSpace(in.IGHandle); ig != "" {
		if !strings.HasPrefix(ig, "@") {
			ig = "@" + ig
		}
		b.WriteString("Instagram: " + ig + "\n")
	}

	if in.Snapshot != nil {
		if prof, _ := in.Snapshot["profile"].(map[string]any); prof != nil {
			if bio := strings.TrimSpace(fmt.Sprint(nz(prof["biography"]))); bio != "" && bio != "<nil>" {
				b.WriteString("Bio: " + clipRunes(bio, 280) + "\n")
			}
			if in.Handle == "" {
				if u := strings.TrimSpace(fmt.Sprint(nz(prof["username"]))); u != "" && u != "<nil>" {
					b.WriteString("Threads: @" + strings.TrimPrefix(u, "@") + "\n")
				}
			}
		}
		if am, _ := in.Snapshot["account_metrics"].(map[string]any); am != nil {
			if m30, _ := am["last_30d"].(map[string]any); len(m30) > 0 {
				b.WriteString("\n### Account metrics (last 30d)\n")
				keys := make([]string, 0, len(m30))
				for k := range m30 {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					b.WriteString(fmt.Sprintf("- %s: %v\n", k, m30[k]))
				}
			}
		}
		if sample, _ := in.Snapshot["sample"].(map[string]any); sample != nil {
			b.WriteString("\n### Sample stats\n")
			b.WriteString(fmt.Sprintf("- posts_in_sample: %v\n", nz(sample["post_count"])))
			b.WriteString(fmt.Sprintf("- avg_ER%%: %v\n", nz(sample["avg_er_pct"])))
			b.WriteString(fmt.Sprintf("- views_p50: %v · p75: %v\n", nz(sample["views_p50"]), nz(sample["views_p75"])))
			b.WriteString(fmt.Sprintf("- median_chars: %v\n", nz(sample["median_chars"])))
			if mix, _ := sample["format_mix"].(map[string]any); len(mix) > 0 {
				b.WriteString(fmt.Sprintf("- format_mix: %v\n", mix))
			}
			if mix, _ := sample["daypart_mix"].(map[string]any); len(mix) > 0 {
				b.WriteString(fmt.Sprintf("- daypart_mix: %v\n", mix))
			}
		}
	}

	niches := NicheList(in.Memory)
	if len(niches) > 0 {
		b.WriteString("\n### Niche (user-set)\n" + strings.Join(niches, "\n") + "\n")
	}
	if br := strings.TrimSpace(in.Memory.Brand); br != "" {
		b.WriteString("Brand: " + br + "\n")
	}
	if instr := strings.TrimSpace(in.Memory.Instructions); instr != "" {
		b.WriteString("\n### Writing rules\n" + clipRunes(instr, 900) + "\n")
	}
	if n := len(in.Memory.Lessons.DoMore); n > 0 {
		b.WriteString("\n### Do more\n")
		for i, it := range in.Memory.Lessons.DoMore {
			if i >= 6 {
				break
			}
			b.WriteString("- " + clipRunes(it.Pattern, 180))
			if it.Evidence != "" {
				b.WriteString(" — " + clipRunes(it.Evidence, 120))
			}
			b.WriteString("\n")
		}
	}
	if n := len(in.Memory.Lessons.Avoid); n > 0 {
		b.WriteString("\n### Avoid\n")
		for i, it := range in.Memory.Lessons.Avoid {
			if i >= 6 {
				break
			}
			b.WriteString("- " + clipRunes(it.Pattern, 180) + "\n")
		}
	}
	if len(in.Memory.Daily) > 0 {
		d := in.Memory.Daily[0]
		b.WriteString("\n### Latest daily focus\n")
		b.WriteString(strings.TrimSpace(d.Date+" · "+d.Focus+" · "+d.Notes) + "\n")
	}

	posts := snapshotPostList(in.Snapshot)
	if len(posts) > 0 {
		top := append([]map[string]any(nil), posts...)
		sort.Slice(top, func(i, j int) bool {
			return num(top[i]["score"]) > num(top[j]["score"])
		})
		b.WriteString("\n### Strongest posts\n")
		for i, p := range top {
			if i >= 5 {
				break
			}
			b.WriteString(formatChatPost(p) + "\n")
		}
		b.WriteString("\n### Weakest posts\n")
		for i := len(top) - 1; i >= 0 && i >= len(top)-3; i-- {
			b.WriteString(formatChatPost(top[i]) + "\n")
		}
		b.WriteString("\n### Recent posts\n")
		for i, p := range posts {
			if i >= 8 {
				break
			}
			b.WriteString(formatChatPost(p) + "\n")
		}
	}

	out := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(out) > 7000 {
		out = clipRunes(out, 7000) + "\n…"
	}
	return out
}

func snapshotPostList(snapshot map[string]any) []map[string]any {
	if snapshot == nil {
		return nil
	}
	if typed, ok := snapshot["posts"].([]map[string]any); ok {
		return typed
	}
	raw, ok := snapshot["posts"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func formatChatPost(p map[string]any) string {
	hook := clipRunes(strings.ReplaceAll(fmt.Sprint(nz(p["text"])), "\n", " / "), 160)
	return fmt.Sprintf("- views %.0f · likes %.0f · replies %.0f · ER %.2f%% · %s · %s",
		num(p["views"]), num(p["likes"]), num(p["replies"]), num(p["engagement_rate"]),
		fmt.Sprint(nz(p["media_type"])), hook)
}

func nz(v any) any {
	if v == nil {
		return ""
	}
	return v
}

func num(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		var f float64
		_, _ = fmt.Sscanf(fmt.Sprint(v), "%f", &f)
		return f
	}
}
