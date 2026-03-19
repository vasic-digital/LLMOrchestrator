// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

// Package main provides the standalone LLMOrchestrator CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.llmorchestrator/pkg/adapter"
	"digital.vasic.llmorchestrator/pkg/config"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("LLMOrchestrator v%s\n", version)
		os.Exit(0)
	}

	// Load configuration.
	cfg := config.LoadFromEnvironment()
	if envFile := os.Getenv("HELIX_ENV_FILE"); envFile != "" {
		var err error
		cfg, err = config.LoadFromEnv(envFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		os.Exit(1)
	}

	// Create agent pool.
	pool := agent.NewPool()

	// Register enabled agents.
	for _, name := range cfg.EnabledAgents {
		path, err := cfg.AgentBinaryPath(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping agent %s: %v\n", name, err)
			continue
		}

		adapterCfg := adapter.AdapterConfig{
			BinaryPath: path,
			Timeout:    cfg.AgentTimeout,
			MaxRetries: cfg.MaxRetries,
			OutputDir:  cfg.SessionDir("default") + "/" + name,
		}

		var a agent.Agent
		switch name {
		case "opencode":
			a = adapter.NewOpenCodeAgent(name+"-0", adapterCfg)
		case "claude-code":
			a = adapter.NewClaudeCodeAgent(name+"-0", adapterCfg)
		case "gemini":
			a = adapter.NewGeminiAgent(name+"-0", adapterCfg)
		case "junie":
			a = adapter.NewJunieAgent(name+"-0", adapterCfg)
		case "qwen-code":
			a = adapter.NewQwenCodeAgent(name+"-0", adapterCfg)
		default:
			fmt.Fprintf(os.Stderr, "Unknown agent: %s\n", name)
			continue
		}

		if err := pool.Register(a); err != nil {
			fmt.Fprintf(os.Stderr, "Error registering agent %s: %v\n", name, err)
			continue
		}
		fmt.Printf("Registered agent: %s (%s)\n", a.Name(), a.ID())
	}

	fmt.Printf("LLMOrchestrator v%s ready with %d agents\n", version, len(pool.Available()))

	// Wait for interrupt.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\nShutting down...")

	if err := pool.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Shutdown complete.")
}
