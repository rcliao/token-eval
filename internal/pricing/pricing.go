package pricing

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/rcliao/token-eval/internal/store"
)

//go:embed bundled.json
var bundledJSON []byte

// BundledEntry represents a bundled pricing entry.
type BundledEntry struct {
	Model         string  `json:"model"`
	Provider      string  `json:"provider"`
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
}

// LoadBundled returns the embedded default pricing entries.
func LoadBundled() ([]BundledEntry, error) {
	var entries []BundledEntry
	if err := json.Unmarshal(bundledJSON, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// SeedDefaults loads bundled pricing into the store (only inserts if not present).
func SeedDefaults(s *store.Store) error {
	entries, err := LoadBundled()
	if err != nil {
		return err
	}
	for _, e := range entries {
		existing, err := s.GetPricing(e.Model)
		if err != nil {
			return err
		}
		if existing != nil {
			continue // don't overwrite user customizations
		}
		if err := s.UpsertPricing(store.Pricing{
			Model:         e.Model,
			Provider:      e.Provider,
			InputPerMTok:  e.InputPerMTok,
			OutputPerMTok: e.OutputPerMTok,
		}); err != nil {
			return err
		}
	}
	return nil
}

// DetectProvider guesses the provider from the model name.
func DetectProvider(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude"):
		return "anthropic"
	case strings.HasPrefix(m, "gpt"):
		return "openai"
	case strings.HasPrefix(m, "gemini"):
		return "google"
	case strings.HasPrefix(m, "deepseek"):
		return "deepseek"
	default:
		return "unknown"
	}
}

// ComputeCost calculates the cost of an LLM call using the pricing table.
func ComputeCost(p *store.Pricing, inputTokens, outputTokens, cacheCreation, cacheRead int) float64 {
	if p == nil {
		return 0
	}
	inputCost := float64(inputTokens) * p.InputPerMTok / 1_000_000
	outputCost := float64(outputTokens) * p.OutputPerMTok / 1_000_000
	cacheCost := float64(cacheCreation) * p.InputPerMTok * p.CacheCreateMultiplier / 1_000_000
	cacheReadCost := float64(cacheRead) * p.InputPerMTok * p.CacheReadMultiplier / 1_000_000
	return inputCost + outputCost + cacheCost + cacheReadCost
}
