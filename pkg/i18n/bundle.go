// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package i18n

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed bundles/*.yaml
var bundleFS embed.FS

// BundleTranslator resolves message IDs from embedded per-locale YAML
// bundles. It is concurrency-safe for reads after construction.
type BundleTranslator struct {
	// locale -> messageID -> template
	messages map[string]map[string]string
	fallback string // locale used when the active locale lacks a key
	active   string
}

// NewBundleTranslator loads every embedded bundles/active.<lang>.yaml
// file. fallbackLocale is consulted when the active locale is missing
// a key; it MUST be present among the loaded bundles. The returned
// Translator defaults its active locale to fallbackLocale — callers
// switch via WithLocale.
func NewBundleTranslator(fallbackLocale string) (*BundleTranslator, error) {
	entries, err := bundleFS.ReadDir("bundles")
	if err != nil {
		return nil, fmt.Errorf("i18n: read embedded bundles: %w", err)
	}

	bt := &BundleTranslator{
		messages: make(map[string]map[string]string),
		fallback: fallbackLocale,
		active:   fallbackLocale,
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "active.") || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		locale := strings.TrimSuffix(strings.TrimPrefix(name, "active."), ".yaml")
		raw, rErr := bundleFS.ReadFile("bundles/" + name)
		if rErr != nil {
			return nil, fmt.Errorf("i18n: read bundle %s: %w", name, rErr)
		}
		var msgs map[string]string
		if uErr := yaml.Unmarshal(raw, &msgs); uErr != nil {
			return nil, fmt.Errorf("i18n: parse bundle %s: %w", name, uErr)
		}
		bt.messages[locale] = msgs
	}

	if _, ok := bt.messages[fallbackLocale]; !ok {
		return nil, fmt.Errorf("i18n: fallback locale %q has no embedded bundle", fallbackLocale)
	}
	return bt, nil
}

// WithLocale returns a shallow copy of bt with the active locale set to
// locale. The underlying message maps are shared (read-only), so this
// is cheap and concurrency-safe.
func (bt *BundleTranslator) WithLocale(locale string) *BundleTranslator {
	cp := *bt
	cp.active = locale
	return &cp
}

// Locales returns the sorted set of locales backed by an embedded
// bundle. Useful for callers exposing a --locale flag.
func (bt *BundleTranslator) Locales() []string {
	out := make([]string, 0, len(bt.messages))
	for l := range bt.messages {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// T resolves messageID against the active locale, falling back to the
// fallback locale and finally to a loud echo of messageID itself.
// {brace} placeholders are substituted from templateData.
func (bt *BundleTranslator) T(_ context.Context, messageID string, templateData map[string]any) (string, error) {
	tmpl, ok := bt.lookup(bt.active, messageID)
	if !ok {
		tmpl, ok = bt.lookup(bt.fallback, messageID)
	}
	if !ok {
		// Loud echo: never silently swallow an unknown ID.
		return messageID, fmt.Errorf("i18n: unknown message id %q", messageID)
	}
	return interpolate(tmpl, templateData), nil
}

func (bt *BundleTranslator) lookup(locale, id string) (string, bool) {
	m, ok := bt.messages[locale]
	if !ok {
		return "", false
	}
	v, ok := m[id]
	return v, ok
}

// interpolate replaces every {key} occurrence in tmpl with the
// stringified data[key]. Unmatched braces are left intact so a missing
// placeholder is visible rather than silently dropped.
func interpolate(tmpl string, data map[string]any) string {
	if len(data) == 0 || !strings.ContainsRune(tmpl, '{') {
		return tmpl
	}
	var b strings.Builder
	b.Grow(len(tmpl))
	for i := 0; i < len(tmpl); {
		if tmpl[i] == '{' {
			if end := strings.IndexByte(tmpl[i:], '}'); end > 0 {
				key := tmpl[i+1 : i+end]
				if val, ok := data[key]; ok {
					b.WriteString(fmt.Sprintf("%v", val))
					i += end + 1
					continue
				}
			}
		}
		b.WriteByte(tmpl[i])
		i++
	}
	return b.String()
}

var (
	globalMu     sync.RWMutex
	globalTrans  Translator = NoopTranslator{}
)

// SetGlobal installs t as the process-wide Translator consulted by the
// package-level Tr/Trf helpers. The CLI calls this once at boot.
func SetGlobal(t Translator) {
	if t == nil {
		t = NoopTranslator{}
	}
	globalMu.Lock()
	globalTrans = t
	globalMu.Unlock()
}

// Global returns the currently-installed process-wide Translator.
func Global() Translator {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalTrans
}

// Tr resolves messageID with no placeholders via the global Translator.
// On resolution failure it returns the loud echo from the Translator —
// it never panics and never returns an empty string.
func Tr(messageID string) string {
	s, _ := Global().T(context.Background(), messageID, nil)
	return s
}

// Trf resolves messageID with {brace} placeholders supplied as data via
// the global Translator.
func Trf(messageID string, data map[string]any) string {
	s, _ := Global().T(context.Background(), messageID, data)
	return s
}
