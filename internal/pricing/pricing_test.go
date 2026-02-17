package pricing

import (
	"math"
	"testing"

	"github.com/rcliao/token-eval/internal/store"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"claude-sonnet-4", "anthropic"},
		{"claude-opus-4", "anthropic"},
		{"gpt-4o", "openai"},
		{"gpt-4o-mini", "openai"},
		{"gemini-2.5-pro", "google"},
		{"deepseek-r1", "deepseek"},
		{"llama-3", "unknown"},
	}
	for _, tt := range tests {
		if got := DetectProvider(tt.model); got != tt.want {
			t.Errorf("DetectProvider(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestComputeCost(t *testing.T) {
	p := &store.Pricing{
		InputPerMTok:          3.0,
		OutputPerMTok:         15.0,
		CacheCreateMultiplier: 1.25,
		CacheReadMultiplier:   0.1,
	}

	cost := ComputeCost(p, 1000, 500, 0, 0)
	// input: 1000 * 3.0 / 1M = 0.003
	// output: 500 * 15.0 / 1M = 0.0075
	expected := 0.0105
	if math.Abs(cost-expected) > 0.0001 {
		t.Fatalf("expected %.4f, got %.4f", expected, cost)
	}

	// With cache
	cost = ComputeCost(p, 1000, 500, 2000, 5000)
	// cache create: 2000 * 3.0 * 1.25 / 1M = 0.0075
	// cache read: 5000 * 3.0 * 0.1 / 1M = 0.0015
	expected = 0.0105 + 0.0075 + 0.0015
	if math.Abs(cost-expected) > 0.0001 {
		t.Fatalf("expected %.4f, got %.4f", expected, cost)
	}

	// Nil pricing
	if ComputeCost(nil, 1000, 500, 0, 0) != 0 {
		t.Fatal("expected 0 for nil pricing")
	}
}

func TestLoadBundled(t *testing.T) {
	entries, err := LoadBundled()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 5 {
		t.Fatalf("expected at least 5 bundled entries, got %d", len(entries))
	}
	// Check one entry
	found := false
	for _, e := range entries {
		if e.Model == "claude-sonnet-4" {
			found = true
			if e.Provider != "anthropic" {
				t.Fatalf("expected anthropic, got %s", e.Provider)
			}
		}
	}
	if !found {
		t.Fatal("claude-sonnet-4 not found in bundled pricing")
	}
}
