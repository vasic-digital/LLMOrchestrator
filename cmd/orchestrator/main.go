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
<<<<<<< HEAD
		// CONST-046 round-387: version banner routed through i18n.
		fmt.Print(cliMsgf(
			"llmorchestrator_cli_version_banner",
			map[string]any{"version": version},
			"LLMOrchestrator v%s\n", version,
		))
=======
		fmt.Println(i18n.Trf("cli.version_line", map[string]any{"version": version}))
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
		os.Exit(0)
	}

	// Load configuration.
	cfg := config.LoadFromEnvironment()
	if envFile := os.Getenv("HELIX_ENV_FILE"); envFile != "" {
		var err error
		cfg, err = config.LoadFromEnv(envFile)
		if err != nil {
<<<<<<< HEAD
			// CONST-046 round-387: config-load error routed through i18n.
			fmt.Fprint(os.Stderr, cliMsgf(
				"llmorchestrator_cli_config_load_error",
				map[string]any{"error": err.Error()},
				"Error loading config: %v\n", err,
			))
=======
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_loading_config", map[string]any{"error": err}))
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
			os.Exit(1)
		}
	}

	if err := cfg.Validate(); err != nil {
<<<<<<< HEAD
		// CONST-046 round-387: invalid-config error routed through i18n.
		fmt.Fprint(os.Stderr, cliMsgf(
			"llmorchestrator_cli_config_invalid",
			map[string]any{"error": err.Error()},
			"Invalid config: %v\n", err,
		))
=======
		fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_invalid_config", map[string]any{"error": err}))
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
		os.Exit(1)
	}

	// Create agent pool.
	pool := agent.NewPool()

	// Register enabled agents.
	for _, name := range cfg.EnabledAgents {
		path, err := cfg.AgentBinaryPath(name)
		if err != nil {
<<<<<<< HEAD
			// CONST-046 round-387: agent-skipped warning routed through i18n.
			fmt.Fprint(os.Stderr, cliMsgf(
				"llmorchestrator_cli_agent_skipped",
				map[string]any{"agent": name, "error": err.Error()},
				"Warning: skipping agent %s: %v\n", name, err,
			))
=======
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.warning_skipping_agent",
				map[string]any{"agent": name, "error": err}))
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
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
<<<<<<< HEAD
			// CONST-046 round-387: unknown-agent error routed through i18n.
			fmt.Fprint(os.Stderr, cliMsgf(
				"llmorchestrator_cli_agent_unknown",
				map[string]any{"agent": name},
				"Unknown agent: %s\n", name,
			))
=======
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_unknown_agent", map[string]any{"agent": name}))
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
			continue
		}

		if err := pool.Register(a); err != nil {
<<<<<<< HEAD
			// CONST-046 round-387: agent-register error routed through i18n.
			fmt.Fprint(os.Stderr, cliMsgf(
				"llmorchestrator_cli_agent_register_error",
				map[string]any{"agent": name, "error": err.Error()},
				"Error registering agent %s: %v\n", name, err,
			))
			continue
		}
		// CONST-046 round-387: agent-registered notice routed through i18n.
		fmt.Print(cliMsgf(
			"llmorchestrator_cli_agent_registered",
			map[string]any{"name": a.Name(), "id": a.ID()},
			"Registered agent: %s (%s)\n", a.Name(), a.ID(),
		))
	}

	// CONST-046 round-387: ready banner routed through i18n.
	fmt.Print(cliMsgf(
		"llmorchestrator_cli_ready_banner",
		map[string]any{"version": version, "count": len(pool.Available())},
		"LLMOrchestrator v%s ready with %d agents\n", version, len(pool.Available()),
	))
=======
			fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_registering_agent",
				map[string]any{"agent": name, "error": err}))
			continue
		}
		fmt.Println(i18n.Trf("cli.registered_agent",
			map[string]any{"name": a.Name(), "id": a.ID()}))
	}

	fmt.Println(i18n.Trf("cli.ready",
		map[string]any{"version": version, "count": len(pool.Available())}))
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805

	// Wait for interrupt.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
<<<<<<< HEAD
	// CONST-046 round-387: shutting-down notice routed through i18n.
	fmt.Println(cliMsg(
		"llmorchestrator_cli_shutting_down",
		nil,
		"\nShutting down...",
	))

	if err := pool.Shutdown(ctx); err != nil {
		// CONST-046 round-387: shutdown error routed through i18n.
		fmt.Fprint(os.Stderr, cliMsgf(
			"llmorchestrator_cli_shutdown_error",
			map[string]any{"error": err.Error()},
			"Shutdown error: %v\n", err,
		))
		os.Exit(1)
	}

	// CONST-046 round-387: shutdown-complete notice routed through i18n.
	fmt.Println(cliMsg(
		"llmorchestrator_cli_shutdown_complete",
		nil,
		"Shutdown complete.",
	))
=======
	fmt.Println("\n" + i18n.Tr("cli.shutting_down"))

	if err := pool.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, i18n.Trf("cli.error_shutdown", map[string]any{"error": err}))
		os.Exit(1)
	}

	fmt.Println(i18n.Tr("cli.shutdown_complete"))
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
}
