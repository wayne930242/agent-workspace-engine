package exporter

import (
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
