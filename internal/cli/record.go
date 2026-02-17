package cli

import (
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

	recordCmd.MarkFlagRequired("project")
	recordCmd.MarkFlagRequired("model")
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
	// Check for stdin JSON
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

	if recProvider == "" {
		recProvider = pricing.DetectProvider(recModel)
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	// Seed default pricing
	pricing.SeedDefaults(s)

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

	// Output the record
	out, _ := json.MarshalIndent(r, "", "  ")
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
