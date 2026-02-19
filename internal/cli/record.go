package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/rcliao/token-eval/internal/pricing"
	"github.com/rcliao/token-eval/internal/store"
	"github.com/spf13/cobra"
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Capture an LLM call record",
	RunE:  runRecord,
}

var (
	recProject      string
	recModel        string
	recTask         string
	recSession      string
	recProvider     string
	recPrompt       string
	recContext       string
	recIntent       string
	recResponse     string
	recInputTokens  int
	recOutputTokens int
	recCacheCreate  int
	recCacheRead    int
	recDuration     int
	recResult       string
	recQuality      int
	recMeta         string
	recBatch        bool
)

func init() {
	recordCmd.Flags().StringVarP(&recProject, "project", "p", "", "Project name (required)")
	recordCmd.Flags().StringVarP(&recModel, "model", "m", "", "Model name (required)")
	recordCmd.Flags().IntVarP(&recInputTokens, "input", "i", 0, "Input token count (required)")
	recordCmd.Flags().IntVarP(&recOutputTokens, "output", "o", 0, "Output token count (required)")
	recordCmd.Flags().StringVarP(&recTask, "task", "t", "", "Task name")
	recordCmd.Flags().StringVar(&recSession, "session", "", "Session ID")
	recordCmd.Flags().StringVar(&recProvider, "provider", "", "Provider (auto-detected if omitted)")
	recordCmd.Flags().StringVar(&recPrompt, "prompt", "", "Prompt text")
	recordCmd.Flags().StringVar(&recContext, "context", "", "Context text")
	recordCmd.Flags().StringVar(&recIntent, "intent", "", "Intent text")
	recordCmd.Flags().StringVar(&recResponse, "response", "", "Model response")
	recordCmd.Flags().IntVar(&recCacheCreate, "cache-create", 0, "Cache creation tokens")
	recordCmd.Flags().IntVar(&recCacheRead, "cache-read", 0, "Cache read tokens")
	recordCmd.Flags().IntVar(&recDuration, "duration", 0, "Duration in ms")
	recordCmd.Flags().StringVar(&recResult, "result", "", "Result: pass or fail")
	recordCmd.Flags().IntVarP(&recQuality, "quality", "q", -1, "Quality score 0-100")
	recordCmd.Flags().StringVar(&recMeta, "meta", "", "JSON metadata")

	recordCmd.Flags().BoolVar(&recBatch, "batch", false, "Read JSONL from stdin (one record per line)")

	// project and model validated in runRecord (not required for --batch)
}

// stdinJSON is the JSON structure accepted via stdin.
type stdinJSON struct {
	Task         string `json:"task"`
	Session      string `json:"session_id"`
	Provider     string `json:"provider"`
	Prompt       string `json:"prompt"`
	Context      string `json:"context"`
	Intent       string `json:"intent"`
	Output       string `json:"output"`
	InputTokens  *int   `json:"input_tokens"`
	OutputTokens *int   `json:"output_tokens"`
	CacheCreate  *int   `json:"cache_creation"`
	CacheRead    *int   `json:"cache_read"`
	Duration     *int   `json:"duration_ms"`
	Result       string `json:"result"`
	Quality      *int   `json:"quality"`
	Meta         string `json:"meta"`
}

func runRecord(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	pricing.SeedDefaults(s)

	// Batch mode: read JSONL from stdin
	if recBatch {
		return runBatchRecord(s)
	}

	// Check for stdin JSON (single record)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err == nil && len(data) > 0 {
			var sj stdinJSON
			if err := json.Unmarshal(data, &sj); err == nil {
				mergeStdin(&sj)
			}
		}
	}

	if recProject == "" {
		return fmt.Errorf("required flag \"project\" not set")
	}
	if recModel == "" {
		return fmt.Errorf("required flag \"model\" not set")
	}

	if recProvider == "" {
		recProvider = pricing.DetectProvider(recModel)
	}

	r := &store.Record{
		Project:       recProject,
		Task:          recTask,
		SessionID:     recSession,
		Model:         recModel,
		Provider:      recProvider,
		Prompt:        recPrompt,
		Context:       recContext,
		Intent:        recIntent,
		Output:        recResponse,
		Result:        recResult,
		InputTokens:   recInputTokens,
		OutputTokens:  recOutputTokens,
		CacheCreation: recCacheCreate,
		CacheRead:     recCacheRead,
		Meta:          recMeta,
	}

	if recQuality >= 0 {
		r.Quality = &recQuality
	}
	if recDuration > 0 {
		d := recDuration
		r.DurationMs = &d
	}

	// Compute cost
	p, _ := s.GetPricing(recModel)
	cost := pricing.ComputeCost(p, r.InputTokens, r.OutputTokens, r.CacheCreation, r.CacheRead)
	if cost > 0 {
		r.CostUSD = &cost
	}

	if err := s.InsertRecord(r); err != nil {
		return fmt.Errorf("inserting record: %w", err)
	}

	out, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(out))
	return nil
}

// batchRecord is the JSON shape for JSONL batch input.
type batchRecord struct {
	Project      string `json:"project"`
	Model        string `json:"model"`
	Task         string `json:"task"`
	SessionID    string `json:"session_id"`
	Provider     string `json:"provider"`
	Prompt       string `json:"prompt"`
	Context      string `json:"context"`
	Intent       string `json:"intent"`
	Output       string `json:"output"`
	Result       string `json:"result"`
	Quality      *int   `json:"quality"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	CacheCreate  int    `json:"cache_creation"`
	CacheRead    int    `json:"cache_read"`
	DurationMs   *int   `json:"duration_ms"`
	Meta         string `json:"meta"`
}

func runBatchRecord(s *store.Store) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB lines
	var count int

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var br batchRecord
		if err := json.Unmarshal(line, &br); err != nil {
			return fmt.Errorf("line %d: %w", count+1, err)
		}
		if br.Project == "" || br.Model == "" {
			return fmt.Errorf("line %d: project and model are required", count+1)
		}
		if br.Provider == "" {
			br.Provider = pricing.DetectProvider(br.Model)
		}

		r := &store.Record{
			Project:       br.Project,
			Task:          br.Task,
			SessionID:     br.SessionID,
			Model:         br.Model,
			Provider:      br.Provider,
			Prompt:        br.Prompt,
			Context:       br.Context,
			Intent:        br.Intent,
			Output:        br.Output,
			Result:        br.Result,
			Quality:       br.Quality,
			InputTokens:   br.InputTokens,
			OutputTokens:  br.OutputTokens,
			CacheCreation: br.CacheCreate,
			CacheRead:     br.CacheRead,
			DurationMs:    br.DurationMs,
			Meta:          br.Meta,
		}

		p, _ := s.GetPricing(r.Model)
		cost := pricing.ComputeCost(p, r.InputTokens, r.OutputTokens, r.CacheCreation, r.CacheRead)
		if cost > 0 {
			r.CostUSD = &cost
		}

		if err := s.InsertRecord(r); err != nil {
			return fmt.Errorf("line %d: %w", count+1, err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	result := map[string]int{"recorded": count}
	out, _ := json.Marshal(result)
	fmt.Println(string(out))
	return nil
}

func mergeStdin(sj *stdinJSON) {
	if sj.Task != "" && recTask == "" {
		recTask = sj.Task
	}
	if sj.Session != "" && recSession == "" {
		recSession = sj.Session
	}
	if sj.Provider != "" && recProvider == "" {
		recProvider = sj.Provider
	}
	if sj.Prompt != "" && recPrompt == "" {
		recPrompt = sj.Prompt
	}
	if sj.Context != "" && recContext == "" {
		recContext = sj.Context
	}
	if sj.Intent != "" && recIntent == "" {
		recIntent = sj.Intent
	}
	if sj.Output != "" && recResponse == "" {
		recResponse = sj.Output
	}
	if sj.Result != "" && recResult == "" {
		recResult = sj.Result
	}
	if sj.InputTokens != nil && recInputTokens == 0 {
		recInputTokens = *sj.InputTokens
	}
	if sj.OutputTokens != nil && recOutputTokens == 0 {
		recOutputTokens = *sj.OutputTokens
	}
	if sj.CacheCreate != nil && recCacheCreate == 0 {
		recCacheCreate = *sj.CacheCreate
	}
	if sj.CacheRead != nil && recCacheRead == 0 {
		recCacheRead = *sj.CacheRead
	}
	if sj.Duration != nil && recDuration == 0 {
		recDuration = *sj.Duration
	}
	if sj.Quality != nil && recQuality < 0 {
		recQuality = *sj.Quality
	}
	if sj.Meta != "" && recMeta == "" {
		recMeta = sj.Meta
	}
}

func openStore() (*store.Store, error) {
	path := dbPath
	if path == "" {
		path = store.DefaultDBPath()
	}
	return store.New(path)
}
