package exporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wayne930242/agent-workspace-engine/internal/manifest"
)

func TestWriteBundleCreatesManifestAndRuntimeExports(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "overlay"), 0o755); err != nil {
		t.Fatalf("mkdir overlay: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "overlay", "overlay.txt"), []byte("overlay"), 0o644); err != nil {
		t.Fatalf("write overlay file: %v", err)
	}

	dir := t.TempDir()
	m := &manifest.WorkspaceManifest{
		Version:   "1",
		Namespace: "moldplan.pm",
		Name:      "pm-service",
		SourceDir: sourceDir,
		BaseRepo: manifest.RepoRef{
			Kind: "repo",
			Path: ".",
		},
		ResolvedOverlays: []manifest.ResolvedOverlay{
			{
				Namespace: "moldplan.pm",
				SourceDir: filepath.Join(sourceDir, "overlay"),
			},
		},
		RuntimeExports: []manifest.RuntimeExport{
			{Runtime: "codex"},
			{Runtime: "claude"},
		},
		PromptContent: "prompt body",
		MCPFiles: []manifest.MCPFile{
			{Path: "configs/base.json", Content: `{"servers":{"filesystem":{}}}`, Merge: true},
		},
		MCPServers: []manifest.MCPServer{
			{Name: "github", Command: "node ./mcp/github.js", Env: []string{"GITHUB_TOKEN"}, RuntimeTargets: []string{"codex", "claude"}, AuthStrategy: "gh"},
		},
	}

	if err := WriteBundle(dir, m); err != nil {
		t.Fatalf("WriteBundle() error = %v", err)
	}

	paths := []string{
		filepath.Join(dir, "workspace-manifest.json"),
		filepath.Join(dir, "control-plane-mapping.json"),
		filepath.Join(dir, "workspace", "base.txt"),
		filepath.Join(dir, "workspace", "overlay.txt"),
		filepath.Join(dir, "exports", "codex", "AGENTS.md"),
		filepath.Join(dir, "exports", "codex", "mcp", "configs", "base.json"),
		filepath.Join(dir, "exports", "codex", "mcp", "mcp-manifest.json"),
		filepath.Join(dir, "exports", "codex", "mcp", "merged.json"),
		filepath.Join(dir, "exports", "codex", "runtime.json"),
		filepath.Join(dir, "exports", "claude", "CLAUDE.md"),
		filepath.Join(dir, "exports", "claude", "PROMPT.md"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
	}
}

func TestWriteBundleGeneratesClaudeSettings(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}

	dir := t.TempDir()
	m := &manifest.WorkspaceManifest{
		Version:   "1",
		Namespace: "test",
		Name:      "test",
		SourceDir: sourceDir,
		BaseRepo:  manifest.RepoRef{Kind: "repo", Path: "."},
		Agent:     &manifest.AgentConfig{Runtime: "claude-code", MCPInject: "auto"},
		Settings: map[string]string{
			"model": "claude-sonnet-4-20250514",
		},
		MCPServers: []manifest.MCPServer{
			{Name: "github", Command: "node ./mcp/github.js", Env: []string{"GITHUB_TOKEN"}, RuntimeTargets: []string{"claude"}},
		},
		RuntimeExports: []manifest.RuntimeExport{{Runtime: "claude"}},
	}

	if err := WriteBundle(dir, m); err != nil {
		t.Fatalf("WriteBundle() error = %v", err)
	}

	settingsPath := filepath.Join(dir, "exports", "claude", ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}

	if got, ok := settings["model"].(string); !ok || got != "claude-sonnet-4-20250514" {
		t.Fatalf("settings.model = %v, want claude-sonnet-4-20250514", settings["model"])
	}

	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("settings.mcpServers missing or wrong type: %v", settings["mcpServers"])
	}
	if _, ok := mcpServers["github"]; !ok {
		t.Fatal("settings.mcpServers.github missing")
	}
}

func TestWriteBundleSkipsMCPInjectWhenDisabled(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}

	dir := t.TempDir()
	m := &manifest.WorkspaceManifest{
		Version:   "1",
		Namespace: "test",
		Name:      "test",
		SourceDir: sourceDir,
		BaseRepo:  manifest.RepoRef{Kind: "repo", Path: "."},
		Agent:     &manifest.AgentConfig{Runtime: "claude-code", MCPInject: "skip"},
		MCPServers: []manifest.MCPServer{
			{Name: "github", Command: "node ./mcp/github.js"},
		},
		RuntimeExports: []manifest.RuntimeExport{{Runtime: "claude"}},
	}

	if err := WriteBundle(dir, m); err != nil {
		t.Fatalf("WriteBundle() error = %v", err)
	}

	settingsPath := filepath.Join(dir, "exports", "claude", ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}

	if _, ok := settings["mcpServers"]; ok {
		t.Fatal("settings.mcpServers should not exist when MCP inject is skip")
	}
}
