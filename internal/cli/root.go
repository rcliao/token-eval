package cli

import (
	"github.com/spf13/cobra"
)

var (
	dbPath     string
	formatFlag string
)

var rootCmd = &cobra.Command{
	Use:   "token-eval",
	Short: "Capture and query LLM call records for prompt evaluation",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&dbPath, "db", "d", "", "Database path (default: $TOKEN_EVAL_DB or ~/.token-eval/eval.db)")
	rootCmd.PersistentFlags().StringVarP(&formatFlag, "format", "f", "json", "Output format: json or text")

	rootCmd.AddCommand(recordCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(summaryCmd)
	rootCmd.AddCommand(priceCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
