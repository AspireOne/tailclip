//go:build !windows

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"tailclip/internal/app"
	"tailclip/internal/config"
	"tailclip/internal/logging"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(err)
	}

	logger, closer, err := logging.New(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer closer.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := app.Run(ctx, logger, cfg); err != nil {
		panic(err)
	}
}
