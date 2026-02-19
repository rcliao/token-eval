package cli

import (
	"encoding/json"
	"fmt"

	"github.com/rcliao/token-eval/internal/store"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export records for eval datasets",
	RunE:  runExport,
}

var (
	exProject string
	exTask    string
	exResult  string
	exLast    string
	exModel   string
)

func init() {
	exportCmd.Flags().StringVarP(&exProject, "project", "p", "", "Filter by project")
	exportCmd.Flags().StringVarP(&exTask, "task", "t", "", "Filter by task")
	exportCmd.Flags().StringVarP(&exModel, "model", "m", "", "Filter by model")
	exportCmd.Flags().StringVar(&exResult, "result", "", "Filter: pass or fail")
	exportCmd.Flags().StringVarP(&exLast, "last", "l", "", "Time window: 24h, 7d, 30d")
}

func runExport(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	f := store.QueryFilter{
		Project: exProject,
		Task:    exTask,
		Model:   exModel,
		Result:  exResult,
		Last:    exLast,
		Limit:   10000, // export all matching
		Full:    true,  // always full for export
	}

	records, err := s.QueryRecords(f)
	if err != nil {
		return fmt.Errorf("querying records: %w", err)
	}

	if formatFlag == "jsonl" {
		for _, r := range records {
			line, _ := json.Marshal(r)
			fmt.Println(string(line))
		}
		return nil
	}

	// Default: pretty JSON array
	if records == nil {
		records = []store.Record{}
	}
	out, _ := json.MarshalIndent(records, "", "  ")
	fmt.Println(string(out))
	return nil
}
