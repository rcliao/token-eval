package store

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInsertAndQuery(t *testing.T) {
	s := tempDB(t)

	r := &Record{
		Project:      "test-project",
		Task:         "test-task",
		Model:        "claude-sonnet-4",
		Provider:     "anthropic",
		Prompt:       "hello world",
		Context:      "some context",
		Intent:       "test intent",
		Output:       "test output",
		InputTokens:  100,
		OutputTokens: 50,
	}
	if err := s.InsertRecord(r); err != nil {
		t.Fatal(err)
	}
	if r.ID == "" {
		t.Fatal("expected ID to be set")
	}

	records, err := s.QueryRecords(QueryFilter{Project: "test-project"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Project != "test-project" {
		t.Fatalf("expected project test-project, got %s", records[0].Project)
	}
}

func TestFTSSearch(t *testing.T) {
	s := tempDB(t)

	s.InsertRecord(&Record{
		Project: "p1", Model: "m1",
		Prompt: "implement FTS5 search functionality",
		Intent: "add full-text search",
		InputTokens: 100, OutputTokens: 50,
	})
	s.InsertRecord(&Record{
		Project: "p1", Model: "m1",
		Prompt: "write unit tests for the API",
		Intent: "test coverage",
		InputTokens: 100, OutputTokens: 50,
	})

	records, err := s.QueryRecords(QueryFilter{Search: "FTS5"})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 FTS result, got %d", len(records))
	}
}

func TestQueryFilters(t *testing.T) {
	s := tempDB(t)

	s.InsertRecord(&Record{Project: "p1", Task: "t1", Model: "claude-sonnet-4", Result: "pass", InputTokens: 100, OutputTokens: 50})
	s.InsertRecord(&Record{Project: "p1", Task: "t2", Model: "gpt-4o", Result: "fail", InputTokens: 100, OutputTokens: 50})
	s.InsertRecord(&Record{Project: "p2", Task: "t1", Model: "claude-sonnet-4", InputTokens: 100, OutputTokens: 50})

	tests := []struct {
		name   string
		filter QueryFilter
		want   int
	}{
		{"by project", QueryFilter{Project: "p1"}, 2},
		{"by model", QueryFilter{Model: "gpt-4o"}, 1},
		{"by result", QueryFilter{Result: "pass"}, 1},
		{"by task", QueryFilter{Project: "p1", Task: "t1"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs, err := s.QueryRecords(tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(recs) != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, len(recs))
			}
		})
	}
}

func TestQueryFull(t *testing.T) {
	s := tempDB(t)
	s.InsertRecord(&Record{
		Project: "p1", Model: "m1",
		Prompt: "the prompt", Context: "the context", Output: "the output",
		InputTokens: 100, OutputTokens: 50,
	})

	// Without full — prompt/context/output should be empty
	recs, _ := s.QueryRecords(QueryFilter{Project: "p1"})
	if recs[0].Prompt != "" {
		t.Fatal("expected empty prompt without --full")
	}

	// With full
	recs, _ = s.QueryRecords(QueryFilter{Project: "p1", Full: true})
	if recs[0].Prompt != "the prompt" {
		t.Fatalf("expected 'the prompt', got '%s'", recs[0].Prompt)
	}
}

func TestPricing(t *testing.T) {
	s := tempDB(t)

	p := Pricing{
		Model: "claude-sonnet-4", Provider: "anthropic",
		InputPerMTok: 3.0, OutputPerMTok: 15.0,
	}
	if err := s.UpsertPricing(p); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetPricing("claude-sonnet-4")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.InputPerMTok != 3.0 {
		t.Fatalf("unexpected pricing: %+v", got)
	}

	prices, err := s.ListPricing()
	if err != nil {
		t.Fatal(err)
	}
	if len(prices) != 1 {
		t.Fatalf("expected 1 price, got %d", len(prices))
	}

	// Not found
	got, err = s.GetPricing("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil for nonexistent")
	}
}

func TestDefaultDBPath(t *testing.T) {
	// With env var
	os.Setenv("TOKEN_EVAL_DB", "/tmp/custom.db")
	defer os.Unsetenv("TOKEN_EVAL_DB")
	if p := DefaultDBPath(); p != "/tmp/custom.db" {
		t.Fatalf("expected /tmp/custom.db, got %s", p)
	}

	// Without env var
	os.Unsetenv("TOKEN_EVAL_DB")
	p := DefaultDBPath()
	if !filepath.IsAbs(p) {
		t.Fatalf("expected absolute path, got %s", p)
	}
}
