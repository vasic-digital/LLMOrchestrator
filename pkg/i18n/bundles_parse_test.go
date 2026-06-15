// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package i18n

import (
	"context"
	"strings"
	"testing"
)

// Regression guard (§11.4.115) for the committed-corruption defect where
// pkg/i18n/bundles/active.en.yaml shipped with unresolved git merge-conflict
// markers (<<<<<<< / ======= / >>>>>>>), making NewBundleTranslator fail at
// construction with "yaml: unmarshal errors: ... cannot unmarshal !!str into
// map[string]string" and killing the entire embedded-bundle i18n path for
// every consumer.
//
// These tests exercise the REAL embedded bundle files (no in-memory fakes) so
// they catch BOTH (a) parse-time corruption (conflict markers, wrong YAML
// shape) AND (b) silent placeholder-syntax regressions ({{.field}} shipping
// literally instead of substituting). RED on the pre-fix corrupted/mis-shaped
// YAML; GREEN after the resolution.

// TestEmbeddedBundles_AllLocalesConstruct constructs a BundleTranslator for
// EVERY embedded active.<lang>.yaml locale and asserts construction succeeds.
// A conflict-marker-corrupted or wrong-shape bundle makes this FAIL — the
// direct guard for the original defect.
func TestEmbeddedBundles_AllLocalesConstruct(t *testing.T) {
	// Discover every embedded locale up front so a newly-added bundle is
	// covered automatically (no hardcoded locale list to drift).
	entries, err := bundleFS.ReadDir("bundles")
	if err != nil {
		t.Fatalf("read embedded bundles dir: %v", err)
	}
	var locales []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "active.") && strings.HasSuffix(n, ".yaml") {
			locales = append(locales, strings.TrimSuffix(strings.TrimPrefix(n, "active."), ".yaml"))
		}
	}
	if len(locales) == 0 {
		t.Fatal("no embedded active.<lang>.yaml bundles found")
	}

	for _, loc := range locales {
		loc := loc
		t.Run(loc, func(t *testing.T) {
			bt, cErr := NewBundleTranslator(loc)
			if cErr != nil {
				t.Fatalf("NewBundleTranslator(%q) failed (corrupted/mis-shaped bundle?): %v", loc, cErr)
			}
			if bt == nil {
				t.Fatalf("NewBundleTranslator(%q) returned nil translator", loc)
			}
			if len(bt.messages[loc]) == 0 {
				t.Fatalf("locale %q parsed to an empty message map", loc)
			}
		})
	}
}

// TestEmbeddedBundles_RepresentativeKeysResolve asserts that one
// representative key from EACH live-consumer group resolves to a substituted,
// non-echo string via the real embedded en bundle. This catches both the
// missing-key failure mode and the {{.field}}-ships-literally failure mode:
// every expectation below requires the {brace} placeholder to be replaced by
// the supplied value.
func TestEmbeddedBundles_RepresentativeKeysResolve(t *testing.T) {
	bt, err := NewBundleTranslator("en")
	if err != nil {
		t.Fatalf("NewBundleTranslator(en): %v", err)
	}
	ctx := context.Background()

	cases := []struct {
		name string
		id   string
		data map[string]any
		want string
	}{
		{
			name: "cli_version_banner", // cmd/orchestrator/main.go
			id:   "llmorchestrator_cli_version_banner",
			data: map[string]any{"version": "1.2.3"},
			want: "LLMOrchestrator v1.2.3\n",
		},
		{
			name: "cli_agent_registered", // cmd/orchestrator/main.go (two fields)
			id:   "llmorchestrator_cli_agent_registered",
			data: map[string]any{"name": "claudecode", "id": "cc-1"},
			want: "Registered agent: claudecode (cc-1)\n",
		},
		{
			name: "agent_with_stderr", // pkg/agent/*_agent.go
			id:   "llmorchestrator_agent_claudecode_invocation_failed_with_stderr",
			data: map[string]any{"sentinel": "claude code invocation failed", "op": "Run", "exitCode": 2, "stderr": "boom"},
			want: "claude code invocation failed: Run exit 2: boom",
		},
		{
			name: "agent_wrapped", // pkg/agent/*_agent.go wrapped branch
			id:   "llmorchestrator_agent_gemini_invocation_failed_wrapped",
			data: map[string]any{"sentinel": "gemini invocation failed", "op": "Run", "wrapped": "context canceled"},
			want: "gemini invocation failed: Run: context canceled",
		},
		{
			name: "config_with_placeholder", // pkg/config/config.go Trf
			id:   "config.no_path_for_agent",
			data: map[string]any{"agent": "qwencode"},
			want: "no path configured for agent: qwencode",
		},
		{
			name: "config_plain", // pkg/config/config.go Tr (no placeholder)
			id:   "config.at_least_one_agent",
			data: nil,
			want: "at least one agent must be enabled",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, gErr := bt.T(ctx, tc.id, tc.data)
			if gErr != nil {
				t.Fatalf("T(%q) error: %v", tc.id, gErr)
			}
			if got != tc.want {
				t.Fatalf("T(%q) = %q, want %q", tc.id, got, tc.want)
			}
			// Defence against the {{.field}}-ships-literally regression: a
			// resolved user-facing string must never contain Go-template
			// double braces.
			if strings.Contains(got, "{{") || strings.Contains(got, "}}") {
				t.Fatalf("T(%q) leaked Go-template syntax (placeholder not substituted): %q", tc.id, got)
			}
		})
	}
}
