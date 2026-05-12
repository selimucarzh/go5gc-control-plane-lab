package main

import (
	"context"
	"log"
	"os"

	"go5gc-control-plane-lab/internal/config"
	"go5gc-control-plane-lab/internal/controlplane"
	"go5gc-control-plane-lab/internal/sim"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		log.Fatalf("parse config: %v", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)

	var client controlplane.Client
	switch cfg.Mode {
	case config.ModeMock:
		client = controlplane.NewMockClient()
	case config.ModeHTTP:
		client = controlplane.NewHTTPClient(cfg.BaseURL, cfg.Timeout)
	default:
		log.Fatalf("unsupported mode: %s", cfg.Mode)
	}

	simulator := sim.NewSimulator(client, logger, cfg)
	if err := simulator.Run(context.Background()); err != nil {
		log.Fatalf("run simulator: %v", err)
	}
}
