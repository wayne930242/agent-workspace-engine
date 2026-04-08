package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wayne930242/agent-workspace-engine/internal/workspacefile"
)

func TestBuildManifestFromWorkspacefileDocument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "prompts"), 0o755); err != nil {
		t.Fatalf("mkdir prompts: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "overlays", "namespaces", "moldplan.pm"), 0o755); err != nil {
		t.Fatalf("mkdir overlays: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts", "pm-service.md"), []byte("prompt body"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mcp"), 0o755); err != nil {
		t.Fatalf("mkdir mcp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mcp", "base.json"), []byte(`{"servers":{"filesystem":{}}}`), 0o644); err != nil {
		t.Fatalf("write mcp file: %v", err)
	}

	doc := &workspacefile.Document{
		Source: filepath.Join(dir, "Workspacefile"),
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"moldplan.pm"}, Line: 2},
			{Keyword: "NAME", Args: []string{"pm-service"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", ".", "AS", "main"}, Line: 4},
			{Keyword: "ATTACH", Args: []string{"repo", "../infra-dashboard", "AS", "dashboard"}, Line: 5},
			{Keyword: "OVERLAY", Args: []string{"namespace", "moldplan.pm"}, Line: 6},
			{Keyword: "PROMPT", Args: []string{"file", "prompts/pm-service.md"}, Line: 7},
			{Keyword: "MCP", Args: []string{"FILE", "mcp/base.json", "MERGE"}, Line: 8},
			{Keyword: "MCP", Args: []string{"SERVER", "github", "COMMAND", "node ./mcp/github.js", "ENV", "GITHUB_TOKEN", "RUNTIME", "codex", "claude", "AUTH", "gh"}, Line: 9},
			{Keyword: "TOOLS", Args: []string{"mcp", "linear_*", "nomad_*"}, Line: 10},
			{Keyword: "EXPORT", Args: []string{"runtime", "codex"}, Line: 11},
			{Keyword: "EXPORT", Args: []string{"runtime", "claude"}, Line: 12},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got, want := m.Namespace, "moldplan.pm"; got != want {
		t.Fatalf("Namespace = %q, want %q", got, want)
	}
	if got, want := m.Name, "pm-service"; got != want {
		t.Fatalf("Name = %q, want %q", got, want)
	}
	if got, want := m.BaseRepo.Alias, "main"; got != want {
		t.Fatalf("BaseRepo.Alias = %q, want %q", got, want)
	}
	if got, want := len(m.AttachedRepos), 1; got != want {
		t.Fatalf("AttachedRepos count = %d, want %d", got, want)
	}
	if got, want := len(m.RuntimeExports), 2; got != want {
		t.Fatalf("RuntimeExports count = %d, want %d", got, want)
	}
	if got, want := m.PromptContent, "prompt body"; got != want {
		t.Fatalf("PromptContent = %q, want %q", got, want)
	}
	if got, want := len(m.ResolvedOverlays), 1; got != want {
		t.Fatalf("ResolvedOverlays count = %d, want %d", got, want)
	}
	if got, want := len(m.MCPFiles), 1; got != want {
		t.Fatalf("MCPFiles count = %d, want %d", got, want)
	}
	if got, want := len(m.MCPServers), 1; got != want {
		t.Fatalf("MCPServers count = %d, want %d", got, want)
	}
	if got, want := m.MCPServers[0].AuthStrategy, "gh"; got != want {
		t.Fatalf("MCPServers[0].AuthStrategy = %q, want %q", got, want)
	}
}

func TestBuildRequiresFromAndMetadata(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 1},
		},
	}

	if _, err := Build(doc); err == nil {
		t.Fatal("Build() error = nil, want error")
	}
}

func TestBuildParsesAgentInstruction(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "AGENT", Args: []string{"claude-code"}, Line: 5},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if m.Agent == nil {
		t.Fatal("Agent is nil")
	}
	if got, want := m.Agent.Runtime, "claude-code"; got != want {
		t.Fatalf("Agent.Runtime = %q, want %q", got, want)
	}
	if got, want := m.Agent.MCPInject, "auto"; got != want {
		t.Fatalf("Agent.MCPInject = %q, want %q (default)", got, want)
	}
}

func TestBuildRejectsPluginWithoutAgent(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "PLUGIN", Args: []string{"npm", "@anthropic/superpowers"}, Line: 5},
		},
	}

	_, err := Build(doc)
	if err == nil {
		t.Fatal("Build() error = nil, want error about PLUGIN requiring AGENT")
	}
}

func TestBuildRejectsSettingsWithoutAgent(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "SETTINGS", Args: []string{"model", "opus"}, Line: 5},
		},
	}

	_, err := Build(doc)
	if err == nil {
		t.Fatal("Build() error = nil, want error about SETTINGS requiring AGENT")
	}
}

func TestBuildSupportsGitSourceDefinition(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 1},
			{Keyword: "NAME", Args: []string{"private-workspace"}, Line: 2},
			{Keyword: "FROM", Args: []string{"git", "git@github.com:org/private-repo.git", "REF", "main", "AUTH", "ssh-agent", "AS", "main"}, Line: 3},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got, want := m.BaseRepo.Kind, "git"; got != want {
		t.Fatalf("BaseRepo.Kind = %q, want %q", got, want)
	}
	if got, want := m.BaseRepo.AuthStrategy, "ssh-agent"; got != want {
		t.Fatalf("BaseRepo.AuthStrategy = %q, want %q", got, want)
	}
}
