package ai

// mergeUsage combines token accounting across multi-step AI pipelines.
func mergeUsage(a, b *TokenUsage) *TokenUsage {
	if a == nil && b == nil {
		return nil
	}
	out := &TokenUsage{}
	if a != nil {
		*out = *a
	}
	if b != nil {
		out.PromptTokens += b.PromptTokens
		out.CompletionTokens += b.CompletionTokens
		out.TotalTokens += b.TotalTokens
	}
	if out.TotalTokens == 0 {
		out.TotalTokens = out.PromptTokens + out.CompletionTokens
	}
	return out
}
