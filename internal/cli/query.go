package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rcliao/token-eval/internal/store"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query [search text]",
	Short: "Query and search captured records",
	RunE:  runQuery,
}

var (
	qProject string
	qTask    string
	qModel   string
	qResult  string
	qLast    string
	qLimit   int
	qFull    bool
)

func init() {
	queryCmd.Flags().StringVarP(&qProject, "project", "p", "", "Filter by project")
	queryCmd.Flags().StringVarP(&qTask, "task", "t", "", "Filter by task")
	queryCmd.Flags().StringVarP(&qModel, "model", "m", "", "Filter by model")
	queryCmd.Flags().StringVar(&qResult, "result", "", "Filter: pass or fail")
	queryCmd.Flags().StringVarP(&qLast, "last", "l", "", "Time window: 24h, 7d, 30d")
	queryCmd.Flags().IntVar(&qLimit, "limit", 20, "Max results")
	queryCmd.Flags().BoolVar(&qFull, "full", false, "Include prompt/context/output")
}

func runQuery(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	f := store.QueryFilter{
		Project: qProject,
		Task:    qTask,
		Model:   qModel,
		Result:  qResult,
		Last:    qLast,
		Limit:   qLimit,
		Full:    qFull,
		Search:  strings.Join(args, " "),
	}

	records, err := s.QueryRecords(f)
	if err != nil {
		return fmt.Errorf("querying records: %w", err)
	}

	if records == nil {
		records = []store.Record{}
	}

	out, _ := json.MarshalIndent(records, "", "  ")
	fmt.Println(string(out))
	return nil
}
