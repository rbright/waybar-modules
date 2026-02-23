package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rbright/waybar-agent-usage/internal/app"
	"github.com/rbright/waybar-agent-usage/internal/config"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			printUsage()
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout+5*time.Second)
	defer cancel()

	if err := app.Run(ctx, args, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println("waybar-agent-usage <codex|claude> [--refresh]")
}
