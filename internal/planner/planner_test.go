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

func TestBuildParsesRunWithoutAgent(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "RUN", Args: []string{"echo hello"}, Line: 5},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := len(m.RunSteps); got != 1 {
		t.Fatalf("RunSteps count = %d, want 1", got)
	}
	if got := m.RunSteps[0].Command; got != "echo hello" {
		t.Fatalf("RunSteps[0].Command = %q, want %q", got, "echo hello")
	}
	if got := m.RunSteps[0].Line; got != 5 {
		t.Fatalf("RunSteps[0].Line = %d, want 5", got)
	}
}

func TestBuildParsesRunMultipleArgs(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "RUN", Args: []string{"npm", "install", "--frozen-lockfile"}, Line: 5},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := m.RunSteps[0].Command; got != "npm install --frozen-lockfile" {
		t.Fatalf("RunSteps[0].Command = %q, want %q", got, "npm install --frozen-lockfile")
	}
}

func TestBuildParsesFromWithInclude(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", ".", "INCLUDE", "src/api", "src/shared", "AS", "main"}, Line: 4},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := len(m.BaseRepo.Includes); got != 2 {
		t.Fatalf("BaseRepo.Includes count = %d, want 2", got)
	}
	if got := m.BaseRepo.Includes[0]; got != "src/api" {
		t.Fatalf("BaseRepo.Includes[0] = %q, want src/api", got)
	}
	if got := m.BaseRepo.Includes[1]; got != "src/shared" {
		t.Fatalf("BaseRepo.Includes[1] = %q, want src/shared", got)
	}
	if got := m.BaseRepo.Alias; got != "main" {
		t.Fatalf("BaseRepo.Alias = %q, want main", got)
	}
}

func TestBuildParsesCopyInstruction(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", ".", "AS", "main"}, Line: 4},
			{Keyword: "COPY", Args: []string{"main:src/utils", "utils"}, Line: 5},
			{Keyword: "COPY", Args: []string{"/tmp/config.json", "."}, Line: 6},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := len(m.CopyRules); got != 2 {
		t.Fatalf("CopyRules count = %d, want 2", got)
	}
	if got := m.CopyRules[0].Source; got != "main:src/utils" {
		t.Fatalf("CopyRules[0].Source = %q, want main:src/utils", got)
	}
	if got := m.CopyRules[0].Dest; got != "utils" {
		t.Fatalf("CopyRules[0].Dest = %q, want utils", got)
	}
	if got := m.CopyRules[1].Source; got != "/tmp/config.json" {
		t.Fatalf("CopyRules[1].Source = %q, want /tmp/config.json", got)
	}
}

func TestBuildParsesPluginAllKinds(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "AGENT", Args: []string{"claude-code"}, Line: 5},
			{Keyword: "PLUGIN", Args: []string{"npm", "@anthropic/superpowers"}, Line: 6},
			{Keyword: "PLUGIN", Args: []string{"git", "github:user/repo", "REF", "main"}, Line: 7},
			{Keyword: "PLUGIN", Args: []string{"path", "./plugins/local"}, Line: 8},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got := len(m.Plugins); got != 3 {
		t.Fatalf("Plugins count = %d, want 3", got)
	}

	tests := []struct {
		idx    int
		kind   string
		source string
		ref    string
	}{
		{0, "npm", "@anthropic/superpowers", ""},
		{1, "git", "github:user/repo", "main"},
		{2, "path", "./plugins/local", ""},
	}
	for _, tt := range tests {
		p := m.Plugins[tt.idx]
		if p.Kind != tt.kind {
			t.Fatalf("Plugins[%d].Kind = %q, want %q", tt.idx, p.Kind, tt.kind)
		}
		if p.Source != tt.source {
			t.Fatalf("Plugins[%d].Source = %q, want %q", tt.idx, p.Source, tt.source)
		}
		if p.Ref != tt.ref {
			t.Fatalf("Plugins[%d].Ref = %q, want %q", tt.idx, p.Ref, tt.ref)
		}
	}
}

func TestBuildRejectsPluginInvalidKind(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "AGENT", Args: []string{"claude-code"}, Line: 5},
			{Keyword: "PLUGIN", Args: []string{"invalid", "something"}, Line: 6},
		},
	}

	_, err := Build(doc)
	if err == nil {
		t.Fatal("Build() error = nil, want error about invalid PLUGIN kind")
	}
}

func TestBuildParsesSettingsKeyValue(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "AGENT", Args: []string{"claude-code"}, Line: 5},
			{Keyword: "SETTINGS", Args: []string{"model", "claude-sonnet-4-20250514"}, Line: 6},
			{Keyword: "SETTINGS", Args: []string{"allowedTools", "Edit,Write,Bash"}, Line: 7},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got := m.Settings["model"]; got != "claude-sonnet-4-20250514" {
		t.Fatalf("Settings[model] = %q, want claude-sonnet-4-20250514", got)
	}
	if got := m.Settings["allowedTools"]; got != "Edit,Write,Bash" {
		t.Fatalf("Settings[allowedTools] = %q, want Edit,Write,Bash", got)
	}
}

func TestBuildParsesSettingsMCPSkip(t *testing.T) {
	t.Parallel()

	doc := &workspacefile.Document{
		Source: "Workspacefile",
		Instructions: []workspacefile.Instruction{
			{Keyword: "VERSION", Args: []string{"1"}, Line: 1},
			{Keyword: "NAMESPACE", Args: []string{"demo"}, Line: 2},
			{Keyword: "NAME", Args: []string{"test"}, Line: 3},
			{Keyword: "FROM", Args: []string{"repo", "."}, Line: 4},
			{Keyword: "AGENT", Args: []string{"claude-code"}, Line: 5},
			{Keyword: "SETTINGS", Args: []string{"MCP", "SKIP"}, Line: 6},
		},
	}

	m, err := Build(doc)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := m.Agent.MCPInject; got != "skip" {
		t.Fatalf("Agent.MCPInject = %q, want skip", got)
	}
}
