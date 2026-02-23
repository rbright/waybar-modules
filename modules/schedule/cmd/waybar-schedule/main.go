package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rbright/waybar-schedule/internal/app"
	"github.com/rbright/waybar-schedule/internal/config"
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

	timeout := cfg.Timeout + 5*time.Second
	if timeout < 10*time.Second {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := app.Run(ctx, args, cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println("waybar-schedule <status|refresh|join-next|join-item N|select-calendars>")
}
