<<<<<<< HEAD
=======
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
package i18n

import (
	"context"
<<<<<<< HEAD
	"testing"
)

// CONST-050(A): mocks/fakes permitted in unit tests only.

func TestNoopTranslator_T_ReturnsMessageIDVerbatim(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.T(
		context.Background(),
		"some.message.id",
		map[string]any{"x": 1},
	)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "some.message.id" {
		t.Fatalf(
			"NoopTranslator.T = %q, want %q",
			got, "some.message.id",
		)
	}
}

func TestNoopTranslator_TPlural_ReturnsMessageIDVerbatim(t *testing.T) {
	tr := NoopTranslator{}
	got, err := tr.TPlural(
		context.Background(),
		"plural.message.id",
		7,
		map[string]any{"y": "z"},
	)
	if err != nil {
		t.Fatalf("NoopTranslator.TPlural returned error: %v", err)
	}
	if got != "plural.message.id" {
		t.Fatalf(
			"NoopTranslator.TPlural = %q, want %q",
			got, "plural.message.id",
		)
	}
}

func TestSetPkgTranslator_AndPkg(t *testing.T) {
	defer SetPkgTranslator(nil) // restore default
	if _, ok := Pkg().(NoopTranslator); !ok {
		t.Fatalf("default Pkg() not NoopTranslator")
	}
	fake := &fakeTranslator{out: "FAKE_OUT"}
	SetPkgTranslator(fake)
	got, _ := Pkg().T(context.Background(), "anything", nil)
	if got != "FAKE_OUT" {
		t.Fatalf("Pkg().T = %q, want FAKE_OUT", got)
	}
	SetPkgTranslator(nil)
	if _, ok := Pkg().(NoopTranslator); !ok {
		t.Fatalf("Pkg() not reset to NoopTranslator on nil")
	}
}

// fakeTranslator is a CONST-050(A)-permitted unit-test mock.
type fakeTranslator struct {
	out string
}

func (f *fakeTranslator) T(
	_ context.Context,
	_ string,
	_ map[string]any,
) (string, error) {
	return f.out, nil
}

func (f *fakeTranslator) TPlural(
	_ context.Context,
	_ string,
	_ int,
	_ map[string]any,
) (string, error) {
	return f.out, nil
=======
	"strings"
	"testing"
)

// TestNoopTranslatorEchoesID confirms the safety default returns the
// message ID verbatim — a loud echo, never a silent empty string.
func TestNoopTranslatorEchoesID(t *testing.T) {
	got, err := NoopTranslator{}.T(context.Background(), "cli.ready", nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != "cli.ready" {
		t.Fatalf("NoopTranslator.T = %q, want loud echo %q", got, "cli.ready")
	}
}

// TestBundleTranslatorResolvesEnglish proves the embedded English
// bundle resolves real human-readable text for every migrated key —
// anti-bluff: this fails if a key is absent or the bundle did not load.
func TestBundleTranslatorResolvesEnglish(t *testing.T) {
	bt, err := NewBundleTranslator("en")
	if err != nil {
		t.Fatalf("NewBundleTranslator: %v", err)
	}

	cases := []struct {
		id       string
		data     map[string]any
		wantSub  string // substring that MUST appear after interpolation
	}{
		{"cli.version_line", map[string]any{"version": "0.1.0"}, "v0.1.0"},
		{"cli.ready", map[string]any{"version": "0.1.0", "count": 3}, "3 agents"},
		{"cli.registered_agent", map[string]any{"name": "opencode", "id": "opencode-0"}, "opencode-0"},
		{"cli.warning_skipping_agent", map[string]any{"agent": "junie", "error": "boom"}, "junie"},
		{"cli.shutdown_complete", nil, "Shutdown complete"},
		{"config.no_path_for_agent", map[string]any{"agent": "gemini"}, "gemini"},
		{"config.agent_binary_not_found", map[string]any{"path": "/x/y"}, "/x/y"},
		{"config.timeout_must_be_positive", nil, "timeout"},
		{"config.at_least_one_agent", nil, "at least one"},
	}

	for _, tc := range cases {
		got, gErr := bt.T(context.Background(), tc.id, tc.data)
		if gErr != nil {
			t.Errorf("T(%q) error: %v", tc.id, gErr)
			continue
		}
		// Anti-bluff: resolved text MUST differ from the bare ID.
		if got == tc.id {
			t.Errorf("T(%q) returned the ID verbatim — bundle key missing", tc.id)
		}
		if !strings.Contains(got, tc.wantSub) {
			t.Errorf("T(%q) = %q, want substring %q", tc.id, got, tc.wantSub)
		}
	}
}

// TestUnknownMessageIDLoudEcho is the paired-mutation guard: an
// unknown ID MUST surface an error AND echo the ID — never swallow it
// silently (which would be a §11.4 PASS-bluff at the i18n layer).
func TestUnknownMessageIDLoudEcho(t *testing.T) {
	bt, err := NewBundleTranslator("en")
	if err != nil {
		t.Fatalf("NewBundleTranslator: %v", err)
	}
	got, gErr := bt.T(context.Background(), "does.not.exist", nil)
	if gErr == nil {
		t.Fatal("expected error for unknown message ID, got nil")
	}
	if got != "does.not.exist" {
		t.Fatalf("unknown ID = %q, want loud echo of the ID", got)
	}
}

// TestInterpolateLeavesUnmatchedBraces is the paired-mutation guard
// for the interpolation engine: a placeholder with no matching data
// key MUST remain visible, not silently vanish.
func TestInterpolateLeavesUnmatchedBraces(t *testing.T) {
	got := interpolate("agent {agent} at {missing}", map[string]any{"agent": "junie"})
	if !strings.Contains(got, "junie") {
		t.Fatalf("interpolate dropped the supplied placeholder: %q", got)
	}
	if !strings.Contains(got, "{missing}") {
		t.Fatalf("interpolate silently dropped an unmatched placeholder: %q", got)
	}
}

// TestFallbackLocaleUsedWhenActiveMissesKey proves locale fallback:
// switching to a non-existent locale still resolves via the English
// fallback rather than echoing the ID.
func TestFallbackLocaleUsedWhenActiveMissesKey(t *testing.T) {
	bt, err := NewBundleTranslator("en")
	if err != nil {
		t.Fatalf("NewBundleTranslator: %v", err)
	}
	xx := bt.WithLocale("xx") // no xx bundle exists
	got, gErr := xx.T(context.Background(), "cli.shutdown_complete", nil)
	if gErr != nil {
		t.Fatalf("fallback resolution errored: %v", gErr)
	}
	if got == "cli.shutdown_complete" {
		t.Fatal("fallback to English did not engage — got bare ID")
	}
}

// TestGlobalTranslatorWiring confirms SetGlobal/Tr/Trf round-trip and
// that a nil Translator degrades safely to the Noop default.
func TestGlobalTranslatorWiring(t *testing.T) {
	t.Cleanup(func() { SetGlobal(nil) })

	bt, err := NewBundleTranslator("en")
	if err != nil {
		t.Fatalf("NewBundleTranslator: %v", err)
	}
	SetGlobal(bt)
	if got := Trf("cli.version_line", map[string]any{"version": "9.9"}); !strings.Contains(got, "v9.9") {
		t.Fatalf("Trf via global = %q, want interpolated version", got)
	}
	if got := Tr("cli.shutting_down"); got == "cli.shutting_down" {
		t.Fatalf("Tr via global returned bare ID %q", got)
	}

	// Paired-mutation guard: nil install must fall back to Noop, not panic.
	SetGlobal(nil)
	if got := Tr("cli.shutting_down"); got != "cli.shutting_down" {
		t.Fatalf("nil-install global = %q, want Noop loud echo", got)
	}
}

// TestEnglishBundleIsFallback proves NewBundleTranslator rejects a
// fallback locale with no embedded bundle — a misconfiguration must
// fail loudly, not silently produce a half-working Translator.
func TestEnglishBundleIsFallback(t *testing.T) {
	if _, err := NewBundleTranslator("zz"); err == nil {
		t.Fatal("expected error for fallback locale with no bundle, got nil")
	}
	locales := mustTranslator(t).Locales()
	found := false
	for _, l := range locales {
		if l == "en" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Locales() = %v, expected to include 'en'", locales)
	}
}

func mustTranslator(t *testing.T) *BundleTranslator {
	t.Helper()
	bt, err := NewBundleTranslator("en")
	if err != nil {
		t.Fatalf("NewBundleTranslator: %v", err)
	}
	return bt
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
}
