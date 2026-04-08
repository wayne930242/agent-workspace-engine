package materializer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wayne930242/agent-workspace-engine/internal/manifest"
)

func TestMaterializeWithIncludeFilter(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	// Create source structure
	for _, path := range []string{
		"src/api/handler.go",
		"src/shared/utils.go",
		"src/internal/secret.go",
		"README.md",
	} {
		full := filepath.Join(sourceDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte("content"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	outDir := t.TempDir()
	m := &manifest.WorkspaceManifest{
		Version:   "1",
		Namespace: "test",
		Name:      "test",
		SourceDir: sourceDir,
		BaseRepo: manifest.RepoRef{
			Kind:     "repo",
			Path:     ".",
			Includes: []string{"src/api", "src/shared"},
		},
	}

	if err := MaterializeWithOptions(outDir, m, Options{}); err != nil {
		t.Fatalf("MaterializeWithOptions() error = %v", err)
	}

	wsDir := filepath.Join(outDir, "workspace")

	// Included paths must exist
	for _, path := range []string{"src/api/handler.go", "src/shared/utils.go"} {
		if _, err := os.Stat(filepath.Join(wsDir, path)); err != nil {
			t.Fatalf("expected included file %s: %v", path, err)
		}
	}
	// Excluded paths must NOT exist
	for _, path := range []string{"src/internal/secret.go", "README.md"} {
		if _, err := os.Stat(filepath.Join(wsDir, path)); err == nil {
			t.Fatalf("expected excluded file %s to not exist", path)
		}
	}
}

func TestMaterializeWithCopyRules(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	// Base repo with src/utils/helper.go
	if err := os.MkdirAll(filepath.Join(sourceDir, "src", "utils"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "src", "utils", "helper.go"), []byte("package utils"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// External file
	externalDir := t.TempDir()
	externalFile := filepath.Join(externalDir, "config.json")
	if err := os.WriteFile(externalFile, []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatalf("write external: %v", err)
	}

	outDir := t.TempDir()
	m := &manifest.WorkspaceManifest{
		Version:   "1",
		Namespace: "test",
		Name:      "test",
		SourceDir: sourceDir,
		BaseRepo: manifest.RepoRef{
			Kind:  "repo",
			Path:  ".",
			Alias: "main",
		},
		CopyRules: []manifest.CopyRule{
			{Source: "main:src/utils", Dest: "utils"},
			{Source: externalFile, Dest: "config.json"},
		},
	}

	if err := MaterializeWithOptions(outDir, m, Options{}); err != nil {
		t.Fatalf("MaterializeWithOptions() error = %v", err)
	}

	wsDir := filepath.Join(outDir, "workspace")

	// Repo-aliased copy
	if _, err := os.Stat(filepath.Join(wsDir, "utils", "helper.go")); err != nil {
		t.Fatalf("expected copied file utils/helper.go: %v", err)
	}
	// External absolute-path copy
	if _, err := os.Stat(filepath.Join(wsDir, "config.json")); err != nil {
		t.Fatalf("expected copied file config.json: %v", err)
	}
}

func TestMaterializeStrictAuthFailsBeforeClone(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	m := &manifest.WorkspaceManifest{
		SourceDir: dir,
		BaseRepo: manifest.RepoRef{
			Kind:         "git",
			URL:          "git@github.com:org/private-repo.git",
			AuthStrategy: "unknown-strategy",
		},
	}

	err := MaterializeWithOptions(dir, m, Options{StrictAuth: true})
	if err == nil {
		t.Fatalf("expected strict auth check to fail")
	}
	if !strings.Contains(err.Error(), "auth strategy") {
		t.Fatalf("unexpected error: %v", err)
	}
}
