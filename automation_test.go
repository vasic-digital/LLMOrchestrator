// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package llmorchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomation_ProjectStructure(t *testing.T) {
	root := findProjectRoot(t)

	requiredDirs := []string{
		"cmd/orchestrator",
		"pkg/agent",
		"pkg/adapter",
		"pkg/protocol",
		"pkg/parser",
		"pkg/config",
	}

	for _, dir := range requiredDirs {
		path := filepath.Join(root, dir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("required directory %s not found: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestAutomation_GoModExists(t *testing.T) {
	root := findProjectRoot(t)
	path := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("go.mod not found: %v", err)
	}
	if !strings.Contains(string(data), "digital.vasic.llmorchestrator") {
		t.Error("go.mod does not contain expected module path")
	}
}

func TestAutomation_GoVet(t *testing.T) {
	root := findProjectRoot(t)
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet failed: %v\n%s", err, string(output))
	}
}

func TestAutomation_GoBuild(t *testing.T) {
	root := findProjectRoot(t)
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
}

func TestAutomation_RequiredFiles(t *testing.T) {
	root := findProjectRoot(t)

	requiredFiles := []string{
		"go.mod",
		"Makefile",
		"README.md",
		"LICENSE",
		"CLAUDE.md",
		".env.example",
		"cmd/orchestrator/main.go",
		"pkg/agent/agent.go",
		"pkg/agent/pool.go",
		"pkg/agent/health.go",
		"pkg/adapter/base.go",
		"pkg/adapter/opencode.go",
		"pkg/adapter/claudecode.go",
		"pkg/adapter/gemini.go",
		"pkg/adapter/junie.go",
		"pkg/adapter/qwencode.go",
		"pkg/protocol/message.go",
		"pkg/protocol/pipe.go",
		"pkg/protocol/file.go",
		"pkg/parser/parser.go",
		"pkg/config/config.go",
	}

	for _, file := range requiredFiles {
		path := filepath.Join(root, file)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("required file %s not found: %v", file, err)
		}
	}
}

func TestAutomation_TestFilesExist(t *testing.T) {
	root := findProjectRoot(t)

	testFiles := []string{
		"pkg/agent/agent_test.go",
		"pkg/agent/pool_test.go",
		"pkg/agent/pool_stress_test.go",
		"pkg/agent/health_test.go",
		"pkg/adapter/adapter_test.go",
		"pkg/adapter/adapter_integration_test.go",
		"pkg/protocol/pipe_test.go",
		"pkg/protocol/file_test.go",
		"pkg/protocol/message_test.go",
		"pkg/protocol/protocol_integration_test.go",
		"pkg/parser/parser_test.go",
		"pkg/parser/parser_fuzz_test.go",
		"pkg/parser/parser_security_test.go",
		"pkg/config/config_test.go",
	}

	for _, file := range testFiles {
		path := filepath.Join(root, file)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("test file %s not found: %v", file, err)
		}
	}
}

func TestAutomation_DocumentationFiles(t *testing.T) {
	root := findProjectRoot(t)

	docFiles := []string{
		"README.md",
		"ARCHITECTURE.md",
		"API_REFERENCE.md",
		"USER_GUIDE.md",
		"CONTRIBUTING.md",
		"CHANGELOG.md",
		"CLAUDE.md",
		"AGENTS.md",
		"VIDEO_COURSE.md",
		"LICENSE",
		".env.example",
	}

	for _, file := range docFiles {
		path := filepath.Join(root, file)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("documentation file %s not found: %v", file, err)
		}
	}
}

func TestAutomation_NoTODOsInProduction(t *testing.T) {
	root := findProjectRoot(t)
	prodDirs := []string{"pkg/agent", "pkg/adapter", "pkg/protocol", "pkg/parser", "pkg/config"}

	for _, dir := range prodDirs {
		dirPath := filepath.Join(root, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dirPath, entry.Name()))
			if err != nil {
				continue
			}
			content := string(data)
			if strings.Contains(content, "TODO") || strings.Contains(content, "FIXME") || strings.Contains(content, "HACK") {
				t.Errorf("production file %s/%s contains TODO/FIXME/HACK", dir, entry.Name())
			}
		}
	}
}

func TestAutomation_LicenseHeaders(t *testing.T) {
	root := findProjectRoot(t)
	prodDirs := []string{"pkg/agent", "pkg/adapter", "pkg/protocol", "pkg/parser", "pkg/config"}

	for _, dir := range prodDirs {
		dirPath := filepath.Join(root, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dirPath, entry.Name()))
			if err != nil {
				continue
			}
			content := string(data)
			if !strings.Contains(content, "SPDX-License-Identifier") {
				t.Errorf("file %s/%s missing SPDX license header", dir, entry.Name())
			}
		}
	}
}

func TestAutomation_MakefileTargets(t *testing.T) {
	root := findProjectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatalf("Makefile not found: %v", err)
	}
	content := string(data)

	requiredTargets := []string{"test", "build", "vet", "clean", "help"}
	for _, target := range requiredTargets {
		if !strings.Contains(content, target+":") {
			t.Errorf("Makefile missing target: %s", target)
		}
	}
}

func TestAutomation_UpstreamsScripts(t *testing.T) {
	root := findProjectRoot(t)
	upstreamsDir := filepath.Join(root, "Upstreams")

	info, err := os.Stat(upstreamsDir)
	if err != nil {
		t.Fatalf("Upstreams directory not found: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Upstreams is not a directory")
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}
