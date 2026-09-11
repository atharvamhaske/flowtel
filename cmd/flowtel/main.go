package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/atharvamhaske/flowtel/internal/config"
	"github.com/atharvamhaske/flowtel/internal/daemon"
	"github.com/atharvamhaske/flowtel/internal/ingest"
	"github.com/atharvamhaske/flowtel/internal/version"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Printf("%s commit=%s date=%s\n", version.Version, version.Commit, version.Date)
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "ingest" {
		if err := runIngest(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "flowtel: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "daemon" {
		if err := runDaemon(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "flowtel: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "status" {
		if err := runStatus(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "flowtel: %v\n", err)
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "usage: flowtel version | flowtel ingest --input PATH [--best-effort] | flowtel daemon serve | flowtel status [--socket PATH]")
	os.Exit(2)
}

func runDaemon(args []string) error {
	if len(args) == 0 || args[0] != "serve" {
		return fmt.Errorf("usage: flowtel daemon serve [--socket PATH] [--data-dir PATH]")
	}
	defaults, err := daemon.DefaultConfig()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("daemon serve", flag.ContinueOnError)
	socketPath := flags.String("socket", defaults.SocketPath, "local Unix socket path")
	dataDir := flags.String("data-dir", defaults.DataDir, "daemon journal directory")
	maxLineBytes := flags.Int("max-line-bytes", defaults.MaxLineBytes, "maximum JSON-RPC frame size")
	if err := flags.Parse(args[1:]); err != nil {
		return fmt.Errorf("parse daemon flags: %w", err)
	}
	config := defaults
	config.SocketPath = *socketPath
	config.DataDir = *dataDir
	config.MaxLineBytes = *maxLineBytes
	server, err := daemon.New(config, nil)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return server.Serve(ctx)
}

func runIngest(args []string) error {
	flags := flag.NewFlagSet("ingest", flag.ContinueOnError)
	inputPath := flags.String("input", "", "Pi session JSONL path")
	bestEffort := flags.Bool("best-effort", false, "continue after malformed JSONL lines")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse ingest flags: %w", err)
	}
	if *inputPath == "" {
		return fmt.Errorf("input path is required")
	}
	input, err := os.Open(*inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer func() { _ = input.Close() }()
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	return ingest.Run(context.Background(), input, cfg, *bestEffort)
}
