// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package parser

import (
	"strings"
	"testing"

	"digital.vasic.llmorchestrator/pkg/agent"
)

func TestParser_NewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestParser_Parse_EmptyInput(t *testing.T) {
	p := NewParser()
	_, err := p.Parse("")
	if err != ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got: %v", err)
	}
}

func TestParser_Parse_SimpleText(t *testing.T) {
	p := NewParser()
	result, err := p.Parse("I see a login form with two fields.")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if result.Raw != "I see a login form with two fields." {
		t.Errorf("unexpected raw: %q", result.Raw)
	}
	if result.Content == "" {
		t.Error("content should not be empty")
	}
}

func TestParser_Parse_WithJSON(t *testing.T) {
	p := NewParser()
	raw := `Here is my analysis:
` + "```json\n" + `{"type": "click", "target": "submit_button", "confidence": 0.95}` + "\n```"

	result, err := p.Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if result.JSON == nil {
		t.Fatal("expected JSON to be extracted")
	}
	if result.JSON["type"] != "click" {
		t.Errorf("unexpected type: %v", result.JSON["type"])
	}
}

func TestParser_Parse_TooLong(t *testing.T) {
	p := NewParser()
	long := strings.Repeat("x", MaxResponseLength+1)
	_, err := p.Parse(long)
	if err != ErrResponseTooLong {
		t.Errorf("expected ErrResponseTooLong, got: %v", err)
	}
}

func TestParser_ExtractJSON_CodeBlock(t *testing.T) {
	p := NewParser()
	raw := "Here is the result:\n```json\n{\"key\": \"value\", \"count\": 42}\n```\n"

	result, err := p.ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON failed: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("unexpected key: %v", result["key"])
	}
	if result["count"] != float64(42) {
		t.Errorf("unexpected count: %v", result["count"])
	}
}

func TestParser_ExtractJSON_BareObject(t *testing.T) {
	p := NewParser()
	raw := `The analysis found {"severity": "high", "issue": "truncation"} in the response.`

	result, err := p.ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON failed: %v", err)
	}
	if result["severity"] != "high" {
		t.Errorf("unexpected severity: %v", result["severity"])
	}
}

func TestParser_ExtractJSON_PureJSON(t *testing.T) {
	p := NewParser()
	raw := `{"action": "navigate", "target": "/settings"}`

	result, err := p.ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON failed: %v", err)
	}
	if result["action"] != "navigate" {
		t.Errorf("unexpected action: %v", result["action"])
	}
}

func TestParser_ExtractJSON_NoJSON(t *testing.T) {
	p := NewParser()
	_, err := p.ExtractJSON("No JSON here, just plain text.")
	if err != ErrNoJSONFound {
		t.Errorf("expected ErrNoJSONFound, got: %v", err)
	}
}

func TestParser_ExtractJSON_Empty(t *testing.T) {
	p := NewParser()
	_, err := p.ExtractJSON("")
	if err != ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got: %v", err)
	}
}

func TestParser_ExtractActions_FromJSON(t *testing.T) {
	p := NewParser()
	raw := `{"actions": [{"type": "click", "target": "button", "confidence": 0.9}, {"type": "type", "target": "input", "value": "hello"}]}`

	actions, err := p.ExtractActions(raw)
	if err != nil {
		t.Fatalf("ExtractActions failed: %v", err)
	}
	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions, got %d", len(actions))
	}
	if actions[0].Type != "click" {
		t.Errorf("expected click action, got %s", actions[0].Type)
	}
	if actions[1].Value != "hello" {
		t.Errorf("expected value 'hello', got %q", actions[1].Value)
	}
}

func TestParser_ExtractActions_FromText(t *testing.T) {
	p := NewParser()
	raw := "I would click the submit button to proceed."

	actions, err := p.ExtractActions(raw)
	if err != nil {
		t.Fatalf("ExtractActions failed: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action from text")
	}

	found := false
	for _, a := range actions {
		if a.Type == "click" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a click action to be extracted")
	}
}

func TestParser_ExtractActions_Empty(t *testing.T) {
	p := NewParser()
	_, err := p.ExtractActions("")
	if err != ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got: %v", err)
	}
}

func TestParser_ExtractActions_NoActions(t *testing.T) {
	p := NewParser()
	actions, _ := p.ExtractActions("The sky is blue and water is wet.")
	if actions != nil && len(actions) > 0 {
		t.Errorf("expected no actions, got %d", len(actions))
	}
}

func TestParser_ExtractActions_SingleJSONAction(t *testing.T) {
	p := NewParser()
	raw := `{"type": "scroll", "value": "down", "confidence": 0.8}`

	actions, err := p.ExtractActions(raw)
	if err != nil {
		t.Fatalf("ExtractActions failed: %v", err)
	}
	if len(actions) < 1 {
		t.Fatal("expected at least 1 action")
	}
}

func TestParser_ExtractIssues_FromJSON(t *testing.T) {
	p := NewParser()
	raw := `{"issues": [{"type": "visual", "severity": "high", "title": "Button truncated", "description": "Save button text is cut off"}]}`

	issues, err := p.ExtractIssues(raw)
	if err != nil {
		t.Fatalf("ExtractIssues failed: %v", err)
	}
	if len(issues) < 1 {
		t.Fatal("expected at least 1 issue")
	}
	if issues[0].Title != "Button truncated" {
		t.Errorf("unexpected title: %q", issues[0].Title)
	}
	if issues[0].Severity != "high" {
		t.Errorf("unexpected severity: %q", issues[0].Severity)
	}
}

func TestParser_ExtractIssues_FromText(t *testing.T) {
	p := NewParser()
	raw := "I found a visual bug: the header text is partially hidden."

	issues, err := p.ExtractIssues(raw)
	if err != nil {
		t.Fatalf("ExtractIssues failed: %v", err)
	}
	if len(issues) < 1 {
		t.Fatal("expected at least 1 issue from text")
	}
}

func TestParser_ExtractIssues_Empty(t *testing.T) {
	p := NewParser()
	_, err := p.ExtractIssues("")
	if err != ErrEmptyInput {
		t.Errorf("expected ErrEmptyInput, got: %v", err)
	}
}

func TestParser_ExtractIssues_WithEvidence(t *testing.T) {
	p := NewParser()
	raw := `{"type": "visual", "severity": "medium", "title": "Misaligned", "description": "Elements are misaligned", "evidence": ["/screenshots/001.png", "/screenshots/002.png"]}`

	issues, err := p.ExtractIssues(raw)
	if err != nil {
		t.Fatalf("ExtractIssues failed: %v", err)
	}
	if len(issues) < 1 {
		t.Fatal("expected at least 1 issue")
	}
	if len(issues[0].Evidence) != 2 {
		t.Errorf("expected 2 evidence items, got %d", len(issues[0].Evidence))
	}
}

func TestParser_ExtractContent_RemovesCodeBlocks(t *testing.T) {
	raw := "Here is the analysis:\n```json\n{\"key\": \"value\"}\n```\nAnd more text."
	content := extractContent(raw)
	if strings.Contains(content, "```") {
		t.Error("content should not contain code blocks")
	}
	if !strings.Contains(content, "Here is the analysis") {
		t.Error("content should contain text before code block")
	}
	if !strings.Contains(content, "And more text") {
		t.Error("content should contain text after code block")
	}
}

func TestParser_DeduplicateActions(t *testing.T) {
	actions := []agent.Action{
		{Type: "click", Target: "button"},
		{Type: "click", Target: "button"}, // duplicate
		{Type: "type", Target: "input", Value: "hello"},
	}

	result := deduplicateActions(actions)
	if len(result) != 2 {
		t.Errorf("expected 2 unique actions, got %d", len(result))
	}
}

func TestParser_ActionKeywords(t *testing.T) {
	tests := []struct {
		keyword    string
		actionType string
	}{
		{"click", "click"},
		{"tap", "click"},
		{"press", "click"},
		{"type", "type"},
		{"enter", "type"},
		{"scroll", "scroll"},
		{"swipe", "scroll"},
		{"navigate", "navigate"},
		{"go to", "navigate"},
		{"open", "navigate"},
		{"back", "back"},
		{"home", "home"},
	}

	for _, tt := range tests {
		if got, ok := actionKeywords[tt.keyword]; !ok || got != tt.actionType {
			t.Errorf("keyword %q: expected %q, got %q (ok=%v)", tt.keyword, tt.actionType, got, ok)
		}
	}
}

func TestParser_Parse_ComplexResponse(t *testing.T) {
	p := NewParser()
	raw := `I analyzed the screen and found several elements.

The main content area shows a text editor with markdown formatting.

I detected a visual bug: the toolbar icons are too small on this screen.

Here are the recommended actions:
` + "```json\n" + `{
  "actions": [
    {"type": "click", "target": "hamburger_menu", "confidence": 0.92},
    {"type": "navigate", "target": "settings", "confidence": 0.88}
  ],
  "issues": [
    {"type": "visual", "severity": "medium", "title": "Small toolbar icons", "description": "Toolbar icons appear too small at this resolution"}
  ]
}` + "\n```"

	result, err := p.Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.Raw != raw {
		t.Error("raw should be preserved")
	}
	if result.JSON == nil {
		t.Error("JSON should be extracted")
	}
	if len(result.Actions) == 0 {
		t.Error("actions should be extracted")
	}
	if len(result.Issues) == 0 {
		t.Error("issues should be extracted")
	}
}

func TestParser_ExtractJSON_NestedObject(t *testing.T) {
	p := NewParser()
	raw := `{"outer": {"inner": "value"}, "list": [1, 2, 3]}`

	result, err := p.ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON failed: %v", err)
	}
	if result["outer"] == nil {
		t.Error("nested object should be preserved")
	}
}

func TestParser_IssueTypes(t *testing.T) {
	p := NewParser()
	issueTexts := map[string]string{
		"visual":        "I found a visual bug on the screen",
		"ux":            "There is a ux issue with the navigation flow",
		"accessibility": "The accessibility of this element is poor",
		"crash":         "The app shows a crash dialog",
		"performance":   "I detected a performance issue with loading",
		"functional":    "There is a functional bug in the save feature",
	}

	for expectedType, text := range issueTexts {
		issues, err := p.ExtractIssues(text)
		if err != nil {
			t.Errorf("ExtractIssues failed for %s: %v", expectedType, err)
			continue
		}
		if len(issues) == 0 {
			t.Errorf("expected issue for type %s", expectedType)
			continue
		}
		if issues[0].Type != expectedType {
			t.Errorf("expected type %s, got %s", expectedType, issues[0].Type)
		}
	}
}

func TestParser_ExtractActions_Confidence(t *testing.T) {
	p := NewParser()
	raw := `{"type": "click", "target": "button", "confidence": 0.95}`

	actions, _ := p.ExtractActions(raw)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	found := false
	for _, a := range actions {
		if a.Confidence == 0.95 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected action with confidence 0.95")
	}
}
