// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

// Package i18n declares LLMOrchestrator's hardcoded-content
// abstraction per CONST-046 (round-383 §11.4 anti-bluff sweep,
// 2026-05-19). Every user-facing string emitted by the CLI or
// surfaced through configuration-validation errors flows through a
// Translator so non-English operators are not silently handed
// untranslatable English literals.
//
// The package is fully decoupled (CONST-051): it embeds its own
// locale bundles and depends on nothing from a consuming project.
// The package-level Tr() / Trf() helpers fall back to a loud
// message-ID echo when no Translator is wired — never a silent
// swallow, which would be a §11.4 PASS-bluff at the i18n layer.
package i18n

import "context"

// Translator is the contract every CONST-046-migrated user-facing
// string in LLMOrchestrator resolves through.
type Translator interface {
	// T resolves messageID against the active locale. templateData
	// supplies named placeholders for {brace}-style interpolation;
	// pass nil when the message has no placeholders.
	T(ctx context.Context, messageID string, templateData map[string]any) (string, error)
}

// NoopTranslator returns the messageID verbatim. SAFETY default for
// unit tests within this package and backward-compat for callers who
// have not yet wired a real Translator. Production paths SHOULD inject
// a bundle-backed Translator (the CLI wires one at boot).
type NoopTranslator struct{}

// T returns id unchanged (loud echo). Never returns an error.
func (NoopTranslator) T(_ context.Context, id string, _ map[string]any) (string, error) {
	return id, nil
}
