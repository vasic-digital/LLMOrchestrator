// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package i18n

import (
	"context"
	"strings"
	"testing"
)

// CONST-050(A): mocks/fakes permitted in unit tests only. These tests
// exercise the BundleTranslator pure logic (lookup / fallback /
// interpolation / locale switching / global helpers) with no network,
// no daemon, and no dependency on the embedded bundle files. The
// translator is constructed directly from in-memory message maps so
// the genuine resolution + substitution behaviour is asserted.

// newTestBundle builds a BundleTranslator from in-memory locale maps,
// mirroring the struct NewBundleTranslator would populate from YAML.
func newTestBundle(active, fallback string, messages map[string]map[string]string) *BundleTranslator {
	return &BundleTranslator{
		messages: messages,
		fallback: fallback,
		active:   active,
	}
}

func sampleMessages() map[string]map[string]string {
	return map[string]map[string]string{
		"en": {
			"greeting":   "Hello {name}",
			"plain":      "No placeholders here",
			"count_only": "You have {count} items",
			"only_in_en": "english-exclusive",
			"two_fields": "{a} then {b}",
		},
		"sr": {
			"greeting": "Zdravo {name}",
			// "only_in_en" deliberately absent → fallback path
		},
	}
}

func TestBundleTranslator_T_ActiveLocaleResolution(t *testing.T) {
	bt := newTestBundle("sr", "en", sampleMessages())
	got, err := bt.T(context.Background(), "greeting", map[string]any{"name": "Ana"})
	if err != nil {
		t.Fatalf("T returned error: %v", err)
	}
	if got != "Zdravo Ana" {
		t.Fatalf("T(active=sr greeting) = %q, want %q", got, "Zdravo Ana")
	}
}

func TestBundleTranslator_T_FallsBackToFallbackLocale(t *testing.T) {
	bt := newTestBundle("sr", "en", sampleMessages())
	// "only_in_en" is absent in sr → must resolve from en fallback.
	got, err := bt.T(context.Background(), "only_in_en", nil)
	if err != nil {
		t.Fatalf("T returned error on fallback: %v", err)
	}
	if got != "english-exclusive" {
		t.Fatalf("T fallback = %q, want %q", got, "english-exclusive")
	}
}

func TestBundleTranslator_T_UnknownIDLoudEcho(t *testing.T) {
	bt := newTestBundle("en", "en", sampleMessages())
	got, err := bt.T(context.Background(), "does.not.exist", nil)
	if err == nil {
		t.Fatalf("T on unknown id returned nil error; want loud error")
	}
	// Loud echo: the unknown id itself is returned, never empty.
	if got != "does.not.exist" {
		t.Fatalf("T unknown-id echo = %q, want %q", got, "does.not.exist")
	}
	if !strings.Contains(err.Error(), "unknown message id") {
		t.Fatalf("error %q does not mention unknown message id", err.Error())
	}
}

func TestBundleTranslator_T_Interpolation(t *testing.T) {
	tests := []struct {
		name string
		id   string
		data map[string]any
		want string
	}{
		{"single", "greeting", map[string]any{"name": "Bob"}, "Hello Bob"},
		{"two_fields", "two_fields", map[string]any{"a": "x", "b": "y"}, "x then y"},
		{"no_placeholders", "plain", map[string]any{"unused": 1}, "No placeholders here"},
		{"nil_data_leaves_braces", "greeting", nil, "Hello {name}"},
		{"missing_key_leaves_brace", "greeting", map[string]any{"other": 1}, "Hello {name}"},
		{"int_value_stringified", "count_only", map[string]any{"count": 5}, "You have 5 items"},
	}
	bt := newTestBundle("en", "en", sampleMessages())
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bt.T(context.Background(), tc.id, tc.data)
			if err != nil {
				t.Fatalf("T err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("T(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}

func TestBundleTranslator_WithLocale_SwitchesActiveOnly(t *testing.T) {
	bt := newTestBundle("en", "en", sampleMessages())
	sr := bt.WithLocale("sr")

	// Original unchanged (shallow copy semantics).
	if bt.active != "en" {
		t.Fatalf("original active mutated to %q", bt.active)
	}
	if sr.active != "sr" {
		t.Fatalf("copy active = %q, want sr", sr.active)
	}
	// Copy shares the read-only message maps + fallback.
	if sr.fallback != "en" {
		t.Fatalf("copy fallback = %q, want en", sr.fallback)
	}
	got, err := sr.T(context.Background(), "greeting", map[string]any{"name": "Ivan"})
	if err != nil {
		t.Fatalf("T err: %v", err)
	}
	if got != "Zdravo Ivan" {
		t.Fatalf("WithLocale(sr).T = %q, want %q", got, "Zdravo Ivan")
	}
}

func TestBundleTranslator_Locales_SortedSet(t *testing.T) {
	bt := newTestBundle("en", "en", map[string]map[string]string{
		"sr": {}, "en": {}, "de": {},
	})
	got := bt.Locales()
	want := []string{"de", "en", "sr"}
	if len(got) != len(want) {
		t.Fatalf("Locales() len = %d (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Locales()[%d] = %q, want %q (sorted)", i, got[i], want[i])
		}
	}
}

func TestBundleTranslator_TPlural_InjectsCount(t *testing.T) {
	bt := newTestBundle("en", "en", sampleMessages())
	got, err := bt.TPlural(context.Background(), "count_only", 3, nil)
	if err != nil {
		t.Fatalf("TPlural err: %v", err)
	}
	if got != "You have 3 items" {
		t.Fatalf("TPlural injected = %q, want %q", got, "You have 3 items")
	}
}

func TestBundleTranslator_TPlural_ExplicitCountNotOverwritten(t *testing.T) {
	bt := newTestBundle("en", "en", sampleMessages())
	// Caller supplies count=99 in templateData; the count arg (3) must
	// NOT overwrite it.
	got, err := bt.TPlural(
		context.Background(),
		"count_only",
		3,
		map[string]any{"count": 99},
	)
	if err != nil {
		t.Fatalf("TPlural err: %v", err)
	}
	if got != "You have 99 items" {
		t.Fatalf("TPlural explicit-count = %q, want %q", got, "You have 99 items")
	}
}

func TestBundleTranslator_TPlural_DoesNotMutateCallerMap(t *testing.T) {
	bt := newTestBundle("en", "en", sampleMessages())
	caller := map[string]any{"name": "X"}
	_, err := bt.TPlural(context.Background(), "greeting", 7, caller)
	if err != nil {
		t.Fatalf("TPlural err: %v", err)
	}
	if _, leaked := caller["count"]; leaked {
		t.Fatalf("TPlural mutated caller map by injecting count: %v", caller)
	}
}

func TestInterpolate_UnterminatedBraceLeftIntact(t *testing.T) {
	// An opening brace with no closing brace must be emitted verbatim.
	got := interpolate("start {unterminated", map[string]any{"unterminated": "X"})
	if got != "start {unterminated" {
		t.Fatalf("interpolate unterminated = %q, want literal passthrough", got)
	}
}

func TestInterpolate_EmptyDataShortCircuit(t *testing.T) {
	got := interpolate("Hello {name}", nil)
	if got != "Hello {name}" {
		t.Fatalf("interpolate(nil data) = %q, want unchanged", got)
	}
}

func TestBundleTranslator_GlobalHelpers_TrAndTrf(t *testing.T) {
	// Save + restore the process-wide translator to avoid cross-test
	// contamination.
	prev := Global()
	defer SetGlobal(prev)

	bt := newTestBundle("en", "en", sampleMessages())
	SetGlobal(bt)

	if got := Tr("plain"); got != "No placeholders here" {
		t.Fatalf("Tr = %q, want %q", got, "No placeholders here")
	}
	if got := Trf("greeting", map[string]any{"name": "Mia"}); got != "Hello Mia" {
		t.Fatalf("Trf = %q, want %q", got, "Hello Mia")
	}
	// Unknown id: Tr never returns empty — loud echo of the id.
	if got := Tr("nope.nope"); got != "nope.nope" {
		t.Fatalf("Tr unknown = %q, want loud echo %q", got, "nope.nope")
	}
}

func TestSetGlobal_NilResetsToNoop(t *testing.T) {
	prev := Global()
	defer SetGlobal(prev)

	SetGlobal(newTestBundle("en", "en", sampleMessages()))
	SetGlobal(nil)
	if _, ok := Global().(NoopTranslator); !ok {
		t.Fatalf("SetGlobal(nil) did not reset to NoopTranslator, got %T", Global())
	}
	// NoopTranslator echoes the id through the Tr helper.
	if got := Tr("any.id"); got != "any.id" {
		t.Fatalf("Tr via Noop = %q, want %q", got, "any.id")
	}
}
