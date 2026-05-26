<<<<<<< HEAD
// Package i18n is the LLMOrchestrator submodule's hardcoded-content
// abstraction layer for CONST-046 compliance.
//
// The LLMOrchestrator submodule is generic infrastructure consumed by
// many projects (CONST-051(B) — fully decoupled). It must not embed a
// project-specific i18n implementation. Instead, this package defines
// a minimal Translator interface that consumers inject at the parent-
// project boundary.
//
// Built-in: NoopTranslator returns each message ID verbatim,
// preserving the previous user-visible behaviour when no consumer
// translator is wired. Package-level translator state is set via
// SetPkgTranslator(); call sites consult Pkg() to translate.
package i18n

import (
	"context"
	"sync"
)

// Translator is the abstraction every CONST-046 migrated call site
// uses to externalise its user-facing strings. Consumers inject real
// implementations (e.g. a YAML-backed i18n adapter, an ICU-MessageFormat
// backend, etc.). Implementations MUST be safe for concurrent use by
// multiple goroutines.
type Translator interface {
	// T returns the user-facing string for messageID with templateData
	// substituted. It returns the substituted string and a nil error on
	// success, or an empty string and a non-nil error if substitution
	// failed.
	T(
		ctx context.Context,
		messageID string,
		templateData map[string]any,
	) (string, error)

	// TPlural returns the count-aware user-facing string for messageID.
	// count selects the plural form (CLDR rules).
	TPlural(
		ctx context.Context,
		messageID string,
		count int,
		templateData map[string]any,
	) (string, error)
}

// NoopTranslator is the safety default: it returns the message ID
// verbatim, ignoring templateData and count. This preserves the
// previous behaviour of code that has not yet been wired to a real
// translator (CONST-035 anti-bluff: no silent string substitution).
type NoopTranslator struct{}

// T returns the messageID verbatim.
func (NoopTranslator) T(
	_ context.Context,
	id string,
	_ map[string]any,
) (string, error) {
	return id, nil
}

// TPlural returns the messageID verbatim, ignoring count.
func (NoopTranslator) TPlural(
	_ context.Context,
	id string,
	_ int,
	_ map[string]any,
) (string, error) {
	return id, nil
}

var (
	pkgMu sync.RWMutex
	pkg   Translator = NoopTranslator{}
)

// SetPkgTranslator installs the package-level translator. Consumers
// call this at boot to inject their real Translator implementation.
// Passing nil resets to NoopTranslator{}.
func SetPkgTranslator(t Translator) {
	pkgMu.Lock()
	defer pkgMu.Unlock()
	if t == nil {
		pkg = NoopTranslator{}
		return
	}
	pkg = t
}

// Pkg returns the currently-installed package-level translator. Safe
// for concurrent use. Defaults to NoopTranslator{}.
func Pkg() Translator {
	pkgMu.RLock()
	defer pkgMu.RUnlock()
	return pkg
}
=======
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
>>>>>>> 4350384757760aabcf8df00be609fff98e9f1805
