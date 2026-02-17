package cli

import (
	"encoding/json"
	"fmt"

	"github.com/rcliao/token-eval/internal/pricing"
	"github.com/rcliao/token-eval/internal/store"
	"github.com/spf13/cobra"
)

var priceCmd = &cobra.Command{
	Use:   "price",
	Short: "Manage model pricing table",
}

var priceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all pricing entries",
	RunE:  runPriceList,
}

var priceSetCmd = &cobra.Command{
	Use:   "set <model>",
	Short: "Add or update a model's pricing",
	Args:  cobra.ExactArgs(1),
	RunE:  runPriceSet,
}

var priceRmCmd = &cobra.Command{
	Use:   "rm <model>",
	Short: "Remove a model's pricing",
	Args:  cobra.ExactArgs(1),
	RunE:  runPriceRm,
}

var (
	psInput    float64
	psOutput   float64
	psProvider string
)

func init() {
	priceSetCmd.Flags().Float64Var(&psInput, "input", 0, "Input price per million tokens (required)")
	priceSetCmd.Flags().Float64Var(&psOutput, "output", 0, "Output price per million tokens (required)")
	priceSetCmd.Flags().StringVar(&psProvider, "provider", "", "Provider (auto-detected if omitted)")
	priceSetCmd.MarkFlagRequired("input")
	priceSetCmd.MarkFlagRequired("output")

	priceCmd.AddCommand(priceListCmd)
	priceCmd.AddCommand(priceSetCmd)
	priceCmd.AddCommand(priceRmCmd)
}

func runPriceList(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	pricing.SeedDefaults(s)

	prices, err := s.ListPricing()
	if err != nil {
		return err
	}
	if prices == nil {
		prices = []store.Pricing{}
	}
	out, _ := json.MarshalIndent(prices, "", "  ")
	fmt.Println(string(out))
	return nil
}

func runPriceSet(cmd *cobra.Command, args []string) error {
	model := args[0]
	provider := psProvider
	if provider == "" {
		provider = pricing.DetectProvider(model)
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	p := store.Pricing{
		Model:         model,
		Provider:      provider,
		InputPerMTok:  psInput,
		OutputPerMTok: psOutput,
	}
	if err := s.UpsertPricing(p); err != nil {
		return err
	}

	out, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(out))
	return nil
}

func runPriceRm(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return s.DeletePricing(args[0])
}
