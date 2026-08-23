package ai

import "strings"

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// cleanStrList normalizes model-produced string lists, removes empty and
// duplicate entries, and caps the result so downstream layouts stay bounded.
func cleanStrList(items []string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	cleaned := make([]string, 0, min(len(items), limit))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		key := strings.ToLower(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, item)
		if len(cleaned) == limit {
			break
		}
	}
	return cleaned
}
