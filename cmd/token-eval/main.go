package main

import (
	"os"

	"github.com/rcliao/token-eval/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
