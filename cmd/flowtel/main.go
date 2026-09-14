package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/atharvamhaske/flowtel/internal/config"
	"github.com/atharvamhaske/flowtel/internal/daemon"
	"github.com/atharvamhaske/flowtel/internal/ingest"
	"github.com/atharvamhaske/flowtel/internal/version"
)

func main() {
	debug := os.Getenv("FLOWTEL_DEBUG") != ""
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--debug" {
		debug = true
		args = args[1:]
	}
	slog.SetDefault(newLogger(debug))

	if len(args) == 1 && args[0] == "version" {
		fmt.Printf("%s commit=%s date=%s\n", version.Version, version.Commit, version.Date)
		return
	}
	if len(args) >= 1 && args[0] == "ingest" {
		if err := runIngest(args[1:]); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		return
	}
	if len(args) >= 1 && args[0] == "daemon" {
		if err := runDaemon(args[1:]); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		return
	}
	if len(args) >= 1 && args[0] == "status" {
		if err := runStatus(args[1:]); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		return
	}
	if len(args) >= 1 && args[0] == "pi" {
		if err := runPi(args[1:]); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		return
	}
	if len(args) >= 1 && args[0] == "doctor" {
		if err := runDoctor(args[1:]); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "usage: flowtel [--debug] version | flowtel ingest --input PATH [--best-effort] | flowtel daemon serve | flowtel status [--socket PATH] | flowtel pi run [--socket PATH] [pi args...] | flowtel doctor [--socket PATH]")
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
	slog.Info("daemon starting", "socket", config.SocketPath, "data_dir", config.DataDir)
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
