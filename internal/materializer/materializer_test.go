package materializer

import (
	"strings"
	"testing"

	"github.com/wayne930242/agent-workspace-engine/internal/manifest"
)

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
