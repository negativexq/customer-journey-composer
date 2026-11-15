package ai

// Usage contains provider-reported counters only. Nil details mean unknown, not zero.
// Anthropic input includes uncached, cache-read and cache-creation input tokens.
type Usage struct {
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	ReasoningTokens     *int64 `json:"reasoning_tokens"`
	CachedInputTokens   *int64 `json:"cached_input_tokens"`
	CacheCreationTokens *int64 `json:"cache_creation_tokens"`
}
type ProviderResult struct {
	Output         []byte `json:"-"`
	RequestedModel string `json:"requested_model"`
	Model          string `json:"model,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	DurationMS     int64  `json:"duration_ms"`
	HTTPStatus     int    `json:"http_status,omitempty"`
	Usage          *Usage `json:"usage"`
}

// Bounded diagnostic counters contain no prompts, candidates, node labels,
// customer values or arbitrary provider strings.
type ValidationSummary struct {
	Schema   int `json:"schema"`
	Workflow int `json:"workflow"`
	Message  int `json:"message"`
	Behavior int `json:"behavior"`
	Facts    int `json:"facts"`
}
type Attempt struct {
	Validation *ValidationSummary `json:"validation,omitempty"`
	Provider   string             `json:"provider"`
	ProviderResult
	ProviderSucceeded bool   `json:"provider_succeeded"`
	Accepted          bool   `json:"accepted"`
	ErrorCode         string `json:"error_code,omitempty"`
}
type GenerationReport struct {
	Attempts []Attempt `json:"attempts"`
	Usage    *Usage    `json:"usage"`
}

func addOptional(a, b *int64) *int64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a + *b
	return &v
}
func (r *GenerationReport) finish() {
	if len(r.Attempts) == 0 {
		return
	}
	var total *Usage
	for _, a := range r.Attempts {
		if a.Usage == nil {
			r.Usage = nil
			return
		}
		if total == nil {
			copy := *a.Usage
			total = &copy
			continue
		}
		total.InputTokens += a.Usage.InputTokens
		total.OutputTokens += a.Usage.OutputTokens
		total.ReasoningTokens = addOptional(total.ReasoningTokens, a.Usage.ReasoningTokens)
		total.CachedInputTokens = addOptional(total.CachedInputTokens, a.Usage.CachedInputTokens)
		total.CacheCreationTokens = addOptional(total.CacheCreationTokens, a.Usage.CacheCreationTokens)
	}
	r.Usage = total
}
