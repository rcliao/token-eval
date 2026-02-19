package cli

import (
	"encoding/json"
	"fmt"

	"github.com/rcliao/token-eval/internal/store"
	"github.com/spf13/cobra"
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Aggregate summary of captured records",
	RunE:  runSummary,
}

var (
	sumProject string
	sumModel   string
	sumToday   bool
	sumLast    string
	sumBy      string
)

func init() {
	summaryCmd.Flags().StringVarP(&sumProject, "project", "p", "", "Filter by project")
	summaryCmd.Flags().StringVarP(&sumModel, "model", "m", "", "Filter by model")
	summaryCmd.Flags().BoolVar(&sumToday, "today", false, "Today only")
	summaryCmd.Flags().StringVarP(&sumLast, "last", "l", "", "Time window: 24h, 7d, 30d")
	summaryCmd.Flags().StringVar(&sumBy, "by", "", "Group by: model, task, project, day")
}

// SummaryOutput is the JSON output for summary.
type SummaryOutput struct {
	Period     string          `json:"period"`
	TotalCalls int            `json:"total_calls"`
	Input      int            `json:"input_tokens"`
	Output     int            `json:"output_tokens"`
	TotalCost  float64        `json:"total_cost_usd"`
	PassRate   *float64       `json:"pass_rate,omitempty"`
	AvgQuality *float64       `json:"avg_quality,omitempty"`
	Breakdown  []BreakdownRow `json:"breakdown,omitempty"`
}

// BreakdownRow is one row in the grouped breakdown.
type BreakdownRow struct {
	Key      string   `json:"key"`
	Calls    int      `json:"calls"`
	Cost     float64  `json:"cost_usd"`
	PassRate *float64 `json:"pass_rate,omitempty"`
}

func runSummary(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	last := sumLast
	if sumToday {
		last = "24h"
	}

	f := store.QueryFilter{
		Project: sumProject,
		Model:   sumModel,
		Last:    last,
		Limit:   100000,
		Full:    false,
	}

	records, err := s.QueryRecords(f)
	if err != nil {
		return fmt.Errorf("querying records: %w", err)
	}

	out := buildSummary(records, last)

	if sumBy != "" {
		out.Breakdown = buildBreakdown(records, sumBy)
	}

	if formatFlag == "text" {
		printSummaryText(out)
		return nil
	}

	j, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(j))
	return nil
}

func buildSummary(records []store.Record, period string) SummaryOutput {
	out := SummaryOutput{Period: period}
	if period == "" {
		out.Period = "all"
	}
	out.TotalCalls = len(records)

	var totalCost float64
	var passCount, resultCount int
	var qualitySum float64
	var qualityCount int

	for _, r := range records {
		out.Input += r.InputTokens
		out.Output += r.OutputTokens
		if r.CostUSD != nil {
			totalCost += *r.CostUSD
		}
		if r.Result != "" {
			resultCount++
			if r.Result == "pass" {
				passCount++
			}
		}
		if r.Quality != nil {
			qualitySum += float64(*r.Quality)
			qualityCount++
		}
	}
	out.TotalCost = totalCost
	if resultCount > 0 {
		pr := float64(passCount) / float64(resultCount)
		out.PassRate = &pr
	}
	if qualityCount > 0 {
		aq := qualitySum / float64(qualityCount)
		out.AvgQuality = &aq
	}
	return out
}

func buildBreakdown(records []store.Record, by string) []BreakdownRow {
	type bucket struct {
		calls       int
		cost        float64
		passCount   int
		resultCount int
	}
	groups := map[string]*bucket{}

	keyFn := func(r store.Record) string {
		switch by {
		case "model":
			return r.Model
		case "task":
			if r.Task == "" {
				return "(none)"
			}
			return r.Task
		case "project":
			return r.Project
		case "day":
			if len(r.CreatedAt) >= 10 {
				return r.CreatedAt[:10]
			}
			return r.CreatedAt
		default:
			return r.Model
		}
	}

	for _, r := range records {
		k := keyFn(r)
		b, ok := groups[k]
		if !ok {
			b = &bucket{}
			groups[k] = b
		}
		b.calls++
		if r.CostUSD != nil {
			b.cost += *r.CostUSD
		}
		if r.Result != "" {
			b.resultCount++
			if r.Result == "pass" {
				b.passCount++
			}
		}
	}

	var rows []BreakdownRow
	for k, b := range groups {
		row := BreakdownRow{Key: k, Calls: b.calls, Cost: b.cost}
		if b.resultCount > 0 {
			pr := float64(b.passCount) / float64(b.resultCount)
			row.PassRate = &pr
		}
		rows = append(rows, row)
	}
	return rows
}

func printSummaryText(out SummaryOutput) {
	fmt.Printf("Period: %s\n", out.Period)
	fmt.Printf("Total calls: %d\n", out.TotalCalls)
	fmt.Printf("Input tokens: %d\n", out.Input)
	fmt.Printf("Output tokens: %d\n", out.Output)
	fmt.Printf("Total cost: $%.4f\n", out.TotalCost)
	if out.PassRate != nil {
		fmt.Printf("Pass rate: %.1f%%\n", *out.PassRate*100)
	}
	if out.AvgQuality != nil {
		fmt.Printf("Avg quality: %.1f\n", *out.AvgQuality)
	}
	if len(out.Breakdown) > 0 {
		fmt.Println("\nBreakdown:")
		for _, b := range out.Breakdown {
			line := fmt.Sprintf("  %s: %d calls, $%.4f", b.Key, b.Calls, b.Cost)
			if b.PassRate != nil {
				line += fmt.Sprintf(", %.0f%% pass", *b.PassRate*100)
			}
			fmt.Println(line)
		}
	}
}
