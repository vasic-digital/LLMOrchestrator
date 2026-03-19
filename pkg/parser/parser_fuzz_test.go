// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package parser

import (
	"testing"
)

func FuzzParser_Parse(f *testing.F) {
	// Seed corpus.
	f.Add("")
	f.Add("hello world")
	f.Add(`{"type": "click", "target": "button"}`)
	f.Add("```json\n{}\n```")
	f.Add("I found a visual bug on the screen")
	f.Add("click the submit button")
	f.Add(`{"actions": [{"type": "navigate"}]}`)
	f.Add(`{"issues": [{"severity": "high", "title": "Bug"}]}`)
	f.Add("```json\n{\"broken\": }\n```")
	f.Add("{{{{{")
	f.Add("}}}}}")
	f.Add(`{"type": "` + string(make([]byte, 100)) + `"}`)

	p := NewParser()
	f.Fuzz(func(t *testing.T, input string) {
		// Parser should never panic, regardless of input.
		_, _ = p.Parse(input)
		_, _ = p.ExtractJSON(input)
		_, _ = p.ExtractActions(input)
		_, _ = p.ExtractIssues(input)
	})
}

func FuzzParser_ExtractJSON(f *testing.F) {
	f.Add(`{}`)
	f.Add(`{"key": "value"}`)
	f.Add(`not json at all`)
	f.Add("```json\n{\"key\": 42}\n```")
	f.Add(`{{{`)
	f.Add(`}}}`)
	f.Add(`[1,2,3]`)
	f.Add(`{"nested": {"a": {"b": "c"}}}`)

	p := NewParser()
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = p.ExtractJSON(input)
	})
}

func FuzzParser_ExtractActions(f *testing.F) {
	f.Add(`{"actions": []}`)
	f.Add(`{"type": "click", "target": "x"}`)
	f.Add("click the button")
	f.Add("navigate to settings and scroll down")
	f.Add("")

	p := NewParser()
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = p.ExtractActions(input)
	})
}
