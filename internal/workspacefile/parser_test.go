package workspacefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileSkipsCommentsAndPreservesQuotedFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "Workspacefile")
	content := `# comment

NAMESPACE moldplan.pm
NAME pm-service
PROMPT file "prompts/pm service.md"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	doc, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if got, want := len(doc.Instructions), 3; got != want {
		t.Fatalf("instruction count = %d, want %d", got, want)
	}

	prompt := doc.Instructions[2]
	if got, want := prompt.Keyword, "PROMPT"; got != want {
		t.Fatalf("prompt keyword = %q, want %q", got, want)
	}

	if got, want := prompt.Args[1], "prompts/pm service.md"; got != want {
		t.Fatalf("prompt path = %q, want %q", got, want)
	}
}
