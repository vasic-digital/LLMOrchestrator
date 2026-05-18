// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.llmorchestrator/pkg/i18n"
)

// Round-115 §11.4 sentinel + mutation evidence: when a real Translator
// is installed via i18n.SetPkgTranslator, the migrated call sites
// across the 5 builder agents MUST route their user-facing error
// strings through it instead of the fmt.Sprintf fallback.
//
// CONST-050(A): mocks are permitted in unit tests only.

type captureTranslator struct {
	lastID string
	out    string
}

func (c *captureTranslator) T(
	_ context.Context,
	id string,
	_ map[string]any,
) (string, error) {
	c.lastID = id
	return c.out, nil
}

func (c *captureTranslator) TPlural(
	_ context.Context,
	id string,
	_ int,
	_ map[string]any,
) (string, error) {
	c.lastID = id
	return c.out, nil
}

func TestInvocationErrors_RouteUserFacingTextThroughI18n(t *testing.T) {
	tcs := []struct {
		name   string
		err    error
		wantID string
	}{
		{
			name:   "opencode",
			err:    &invocationError{op: "send", exitCode: 1, stderr: "boom"},
			wantID: "llmorchestrator_agent_opencode_invocation_failed_with_stderr",
		},
		{
			name:   "claudecode",
			err:    &claudeCodeInvocationError{op: "send", exitCode: 2, stderr: "x"},
			wantID: "llmorchestrator_agent_claudecode_invocation_failed_with_stderr",
		},
		{
			name:   "gemini",
			err:    &geminiInvocationError{op: "send", exitCode: 3, stderr: "y"},
			wantID: "llmorchestrator_agent_gemini_invocation_failed_with_stderr",
		},
		{
			name:   "junie",
			err:    &junieInvocationError{op: "send", exitCode: 4, stderr: "z"},
			wantID: "llmorchestrator_agent_junie_invocation_failed_with_stderr",
		},
		{
			name:   "qwencode",
			err:    &qwenCodeInvocationError{op: "send", exitCode: 5, stderr: "q"},
			wantID: "llmorchestrator_agent_qwencode_invocation_failed_with_stderr",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cap := &captureTranslator{out: "TRANSLATED_" + tc.name}
			i18n.SetPkgTranslator(cap)
			t.Cleanup(func() { i18n.SetPkgTranslator(nil) })

			got := tc.err.Error()
			if got != "TRANSLATED_"+tc.name {
				t.Fatalf("Error() = %q, want %q (i18n seam not wired)", got, "TRANSLATED_"+tc.name)
			}
			if cap.lastID != tc.wantID {
				t.Fatalf("translator received id %q, want %q", cap.lastID, tc.wantID)
			}
		})
	}
}

// Mutation evidence: with NoopTranslator (default), call site MUST
// fall back to fmt.Sprintf so the captured stderr stays visible to
// callers — this proves the bare-ID surface never leaks to users.
func TestInvocationErrors_NoopTranslator_FallsBackToFmt(t *testing.T) {
	i18n.SetPkgTranslator(nil) // reset to default Noop
	err := &invocationError{op: "send", exitCode: 1, stderr: "synthetic-stderr-XYZ"}
	got := err.Error()
	if got == "llmorchestrator_agent_opencode_invocation_failed_with_stderr" {
		t.Fatalf("Error() leaked bare message ID: %q", got)
	}
	wantSubstr := "synthetic-stderr-XYZ"
	if !strings.Contains(got, wantSubstr) {
		t.Fatalf("Error() = %q, want substring %q", got, wantSubstr)
	}
}
