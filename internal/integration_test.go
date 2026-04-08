package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wayne930242/agent-workspace-engine/internal/exporter"
	"github.com/wayne930242/agent-workspace-engine/internal/manifest"
	"github.com/wayne930242/agent-workspace-engine/internal/planner"
	"github.com/wayne930242/agent-workspace-engine/internal/workspacefile"
)

func TestEndToEndAgentConfigLayer(t *testing.T) {
	t.Parallel()

	// Set up source directory
	dir := t.TempDir()
	for _, path := range []string{
		"src/api/handler.go",
		"src/shared/utils.go",
		"src/internal/secret.go",
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("content"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Create Workspacefile document (simulates parsed Workspacefile)
	doc := &workspacefile.Document{
		Source: filepath.Join(dir, "Workspacefile"),
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"test.ns"}, Line: 2},
			{Keyword: "NAME", Args: []string{"e2e-test"}, Line: 3},
			{Keyword: "AGENT", Args: []string{"claude-code"}, Line: 4},
			{Keyword: "FROM", Args: []string{"repo", ".", "INCLUDE", "src/api", "src/shared", "AS", "main"}, Line: 5},
			{Keyword: "COPY", Args: []string{"main:src/shared", "shared-copy"}, Line: 6},
			{Keyword: "PLUGIN", Args: []string{"npm", "@anthropic/superpowers"}, Line: 7},
			{Keyword: "RUN", Args: []string{"echo done"}, Line: 8},
			{Keyword: "SETTINGS", Args: []string{"model", "opus"}, Line: 9},
			{Keyword: "EXPORT", Args: []string{"runtime", "claude"}, Line: 10},
		},
	}

	// Planner stage
	m, err := planner.Build(doc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if m.Agent == nil || m.Agent.Runtime != "claude-code" {
		t.Fatal("Agent not set")
	}
	if len(m.Plugins) != 1 {
		t.Fatalf("Plugins = %d, want 1", len(m.Plugins))
	}
	if len(m.RunSteps) != 1 {
		t.Fatalf("RunSteps = %d, want 1", len(m.RunSteps))
	}
	if len(m.CopyRules) != 1 {
		t.Fatalf("CopyRules = %d, want 1", len(m.CopyRules))
	}
	if len(m.BaseRepo.Includes) != 2 {
		t.Fatalf("Includes = %d, want 2", len(m.BaseRepo.Includes))
	}

	// Exporter stage
	outDir := t.TempDir()
	if err := exporter.WriteBundle(outDir, m); err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}

	wsDir := filepath.Join(outDir, "workspace")

	// INCLUDE filtering: api and shared included, internal excluded
	if _, err := os.Stat(filepath.Join(wsDir, "src", "api", "handler.go")); err != nil {
		t.Fatal("included file src/api/handler.go missing")
	}
	if _, err := os.Stat(filepath.Join(wsDir, "src", "internal", "secret.go")); err == nil {
		t.Fatal("excluded file src/internal/secret.go should not exist")
	}

	// COPY: shared-copy directory should exist
	if _, err := os.Stat(filepath.Join(wsDir, "shared-copy", "utils.go")); err != nil {
		t.Fatal("COPY target shared-copy/utils.go missing")
	}

	// settings.json
	settingsData, err := os.ReadFile(filepath.Join(outDir, "exports", "claude", ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	if settings["model"] != "opus" {
		t.Fatalf("settings.model = %v, want opus", settings["model"])
	}

	// plugins.json
	if _, err := os.Stat(filepath.Join(outDir, "exports", "claude", "plugins.json")); err != nil {
		t.Fatal("plugins.json missing")
	}

	// setup.sh
	setupData, err := os.ReadFile(filepath.Join(outDir, "exports", "claude", "setup.sh"))
	if err != nil {
		t.Fatalf("read setup.sh: %v", err)
	}
	setupContent := string(setupData)
	if !strings.Contains(setupContent, "echo done") {
		t.Fatalf("setup.sh missing RUN command, got:\n%s", setupContent)
	}
	if !strings.Contains(setupContent, "claude plugin add @anthropic/superpowers") {
		t.Fatalf("setup.sh missing plugin install, got:\n%s", setupContent)
	}
}

// Compile-time import check — keep manifest reachable from this package.
var _ = manifest.WorkspaceManifest{}
