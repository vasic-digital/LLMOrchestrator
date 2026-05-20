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

	"digital.vasic.llmorchestrator/pkg/adapter"
	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.llmorchestrator/pkg/config"
	"digital.vasic.llmorchestrator/pkg/i18n"
)

const version = "0.1.0"

// initI18n wires the embedded locale bundles into the global
// Translator so every user-facing string below resolves through the
// CONST-046 seam. The active locale honours LLMORCHESTRATOR_LOCALE /
// LANG; an unknown locale degrades gracefully to the English bundle.
func initI18n() {
	bt, err := i18n.NewBundleTranslator("en")
	if err != nil {
		// Fall back to the loud-echo NoopTranslator (set by default);
		// the CLI stays usable, just untranslated.
		return
	}
	if loc := resolveLocale(); loc != "" {
		bt = bt.WithLocale(loc)
	}
	i18n.SetGlobal(bt)
}

// resolveLocale derives a two-letter locale from the environment.
func resolveLocale() string {
	for _, key := range []string{"LLMORCHESTRATOR_LOCALE", "LC_ALL", "LANG"} {
		if v := os.Getenv(key); v != "" {
			if len(v) >= 2 {
				return v[:2]
			}
		}
	}
	return ""
}

func main() {
	initI18n()

	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(i18n.Trf("cli.version_line", map[string]any{"version": version}))
		os.Exit(0)
	}

	// Load configuration.
	cfg := config.LoadFromEnvironment()
	if envFile := os.Getenv("HELIX_ENV_FILE"); envFile != "" {
		var err error
		cfg, err = config.LoadFromEnv(envFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_loading_config", map[string]any{"error": err}))
			os.Exit(1)
		}
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_invalid_config", map[string]any{"error": err}))
		os.Exit(1)
	}

	// Create agent pool.
	pool := agent.NewPool()

	// Register enabled agents.
	for _, name := range cfg.EnabledAgents {
		path, err := cfg.AgentBinaryPath(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.warning_skipping_agent",
				map[string]any{"agent": name, "error": err}))
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
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_unknown_agent", map[string]any{"agent": name}))
			continue
		}

		if err := pool.Register(a); err != nil {
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_registering_agent",
				map[string]any{"agent": name, "error": err}))
			continue
		}
		fmt.Println(i18n.Trf("cli.registered_agent",
			map[string]any{"name": a.Name(), "id": a.ID()}))
	}

	fmt.Println(i18n.Trf("cli.ready",
		map[string]any{"version": version, "count": len(pool.Available())}))

	// Wait for interrupt.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\n" + i18n.Tr("cli.shutting_down"))

	if err := pool.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_shutdown", map[string]any{"error": err}))
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("cli.shutdown_complete"))
}
