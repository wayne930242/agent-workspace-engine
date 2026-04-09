# Agent Configuration Layer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add AGENT, PLUGIN, RUN, SETTINGS, COPY, and INCLUDE support to the Workspacefile DSL, with claude-code exporter enhancements that produce ready-to-use settings.json and plugin manifests.

**Architecture:** Six new DSL instructions flow through the existing 4-stage pipeline (Parser → Planner → Manifest → Exporter). The parser already handles them as generic instructions. The planner gains gate logic (AGENT-gated instructions) and new manifest field population. The exporter gains a claude-code–specific path that produces `.claude/settings.json`, `plugins.json`, and `setup.sh`.

**Tech Stack:** Go 1.24, standard library only (no external deps).

---

## Task 1: Add new types to manifest

**Files:**
- Modify: `internal/manifest/types.go`

**Step 1: Write the failing test**

Create `internal/manifest/types_test.go`:

```go
package manifest

import (
	"encoding/json"
	"testing"
)

func TestManifestNewFieldsSerialization(t *testing.T) {
	t.Parallel()

	m := WorkspaceManifest{
		Version:   "1",
		Namespace: "test",
		Name:      "test",
		Agent:     &AgentConfig{Runtime: "claude-code", MCPInject: "auto"},
		Plugins: []PluginRef{
			{Kind: "npm", Source: "@anthropic/superpowers"},
			{Kind: "git", Source: "github:user/repo", Ref: "main"},
			{Kind: "path", Source: "./plugins/local"},
		},
		RunSteps: []RunStep{
			{Command: "npm install", Line: 10},
		},
		Settings: map[string]string{
			"model": "claude-sonnet-4-20250514",
		},
		CopyRules: []CopyRule{
			{Source: "main:src/utils", Dest: "utils"},
			{Source: "/tmp/config.json", Dest: "."},
		},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded WorkspaceManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Agent == nil || decoded.Agent.Runtime != "claude-code" {
		t.Fatalf("Agent = %+v, want runtime claude-code", decoded.Agent)
	}
	if got := len(decoded.Plugins); got != 3 {
		t.Fatalf("Plugins count = %d, want 3", got)
	}
	if got := decoded.Plugins[1].Ref; got != "main" {
		t.Fatalf("Plugins[1].Ref = %q, want %q", got, "main")
	}
	if got := len(decoded.RunSteps); got != 1 {
		t.Fatalf("RunSteps count = %d, want 1", got)
	}
	if got := decoded.Settings["model"]; got != "claude-sonnet-4-20250514" {
		t.Fatalf("Settings[model] = %q, want claude-sonnet-4-20250514", got)
	}
	if got := len(decoded.CopyRules); got != 2 {
		t.Fatalf("CopyRules count = %d, want 2", got)
	}
	if got := decoded.CopyRules[0].Source; got != "main:src/utils" {
		t.Fatalf("CopyRules[0].Source = %q, want main:src/utils", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/manifest/ -run TestManifestNewFieldsSerialization -v`
Expected: FAIL — `AgentConfig`, `PluginRef`, `RunStep`, `CopyRule` types undefined.

**Step 3: Write minimal implementation**

Add to `internal/manifest/types.go`:

```go
// Add fields to WorkspaceManifest struct:
Agent     *AgentConfig      `json:"agent,omitempty"`
Plugins   []PluginRef       `json:"plugins,omitempty"`
RunSteps  []RunStep         `json:"run_steps,omitempty"`
Settings  map[string]string `json:"settings,omitempty"`
CopyRules []CopyRule        `json:"copy_rules,omitempty"`

// Add new types after RuntimeExport:

type AgentConfig struct {
	Runtime   string `json:"runtime"`
	MCPInject string `json:"mcp_inject"`
}

type PluginRef struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

type RunStep struct {
	Command string `json:"command"`
	Line    int    `json:"line"`
}

type CopyRule struct {
	Source string `json:"source"`
	Dest   string `json:"dest"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/manifest/ -run TestManifestNewFieldsSerialization -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/types.go internal/manifest/types_test.go
git commit -m "feat(manifest): add Agent, Plugin, RunStep, CopyRule, Settings types"
```

---

## Task 2: Add INCLUDE support to FROM/ATTACH in manifest

**Files:**
- Modify: `internal/manifest/types.go` (RepoRef)

**Step 1: Write the failing test**

Add to `internal/manifest/types_test.go`:

```go
func TestRepoRefIncludeSerialization(t *testing.T) {
	t.Parallel()

	ref := RepoRef{
		Kind:     "repo",
		Path:     ".",
		Alias:    "main",
		Includes: []string{"src/api", "src/shared"},
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded RepoRef
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got := len(decoded.Includes); got != 2 {
		t.Fatalf("Includes count = %d, want 2", got)
	}
	if got := decoded.Includes[0]; got != "src/api" {
		t.Fatalf("Includes[0] = %q, want src/api", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/manifest/ -run TestRepoRefIncludeSerialization -v`
Expected: FAIL — `Includes` field undefined on `RepoRef`.

**Step 3: Write minimal implementation**

Add `Includes` field to `RepoRef` in `internal/manifest/types.go`:

```go
type RepoRef struct {
	Kind         string   `json:"kind"`
	Path         string   `json:"path,omitempty"`
	URL          string   `json:"url,omitempty"`
	Alias        string   `json:"alias,omitempty"`
	Ref          string   `json:"ref,omitempty"`
	AuthStrategy string   `json:"auth_strategy,omitempty"`
	Includes     []string `json:"includes,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/manifest/ -run TestRepoRefIncludeSerialization -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/manifest/types.go internal/manifest/types_test.go
git commit -m "feat(manifest): add Includes field to RepoRef for selective file inclusion"
```

---

## Task 3: Planner — parse AGENT instruction with gate logic

**Files:**
- Modify: `internal/planner/planner.go`
- Test: `internal/planner/planner_test.go`

**Step 1: Write the failing test**

Add to `internal/planner/planner_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run "TestBuildParsesAgent|TestBuildRejectsPlugin|TestBuildRejectsSettings" -v`
Expected: FAIL — `AGENT`, `PLUGIN`, `SETTINGS` are unsupported instructions.

**Step 3: Write minimal implementation**

In `internal/planner/planner.go`, add a `agentDeclared` flag and three new cases inside the `switch inst.Keyword` block:

```go
// At the top of Build(), after creating m:
var agentDeclared bool

// Inside the switch:
case "AGENT":
    if len(inst.Args) != 1 {
        return nil, fmt.Errorf("line %d: AGENT requires exactly one argument", inst.Line)
    }
    if agentDeclared {
        return nil, fmt.Errorf("line %d: duplicate AGENT instruction", inst.Line)
    }
    agentDeclared = true
    m.Agent = &manifest.AgentConfig{
        Runtime:   inst.Args[0],
        MCPInject: "auto",
    }

case "PLUGIN":
    if !agentDeclared {
        return nil, fmt.Errorf("line %d: PLUGIN requires an AGENT declaration first", inst.Line)
    }
    plugin, err := parsePlugin(inst)
    if err != nil {
        return nil, err
    }
    m.Plugins = append(m.Plugins, plugin)

case "SETTINGS":
    if !agentDeclared {
        return nil, fmt.Errorf("line %d: SETTINGS requires an AGENT declaration first", inst.Line)
    }
    if err := parseSettings(inst, m); err != nil {
        return nil, err
    }
```

Add helper functions (bottom of `planner.go`):

```go
func parsePlugin(inst workspacefile.Instruction) (manifest.PluginRef, error) {
    if len(inst.Args) < 2 {
        return manifest.PluginRef{}, fmt.Errorf("line %d: PLUGIN syntax: PLUGIN <npm|git|path> <source> [REF <ref>]", inst.Line)
    }
    ref := manifest.PluginRef{Kind: inst.Args[0], Source: inst.Args[1]}
    switch ref.Kind {
    case "npm", "git", "path":
    default:
        return manifest.PluginRef{}, fmt.Errorf("line %d: PLUGIN kind must be npm, git, or path", inst.Line)
    }
    for i := 2; i < len(inst.Args); i += 2 {
        if i+1 >= len(inst.Args) {
            return manifest.PluginRef{}, fmt.Errorf("line %d: PLUGIN option %q missing value", inst.Line, inst.Args[i])
        }
        switch inst.Args[i] {
        case "REF":
            ref.Ref = inst.Args[i+1]
        default:
            return manifest.PluginRef{}, fmt.Errorf("line %d: unsupported PLUGIN option %q", inst.Line, inst.Args[i])
        }
    }
    return ref, nil
}

func parseSettings(inst workspacefile.Instruction, m *manifest.WorkspaceManifest) error {
    if len(inst.Args) < 2 {
        return fmt.Errorf("line %d: SETTINGS requires at least key and value", inst.Line)
    }
    // Special case: SETTINGS MCP INJECT / SETTINGS MCP SKIP
    if inst.Args[0] == "MCP" {
        if len(inst.Args) != 2 {
            return fmt.Errorf("line %d: SETTINGS MCP syntax: SETTINGS MCP <INJECT|SKIP>", inst.Line)
        }
        switch inst.Args[1] {
        case "INJECT":
            m.Agent.MCPInject = "auto"
        case "SKIP":
            m.Agent.MCPInject = "skip"
        default:
            return fmt.Errorf("line %d: SETTINGS MCP mode must be INJECT or SKIP", inst.Line)
        }
        return nil
    }
    if m.Settings == nil {
        m.Settings = make(map[string]string)
    }
    m.Settings[inst.Args[0]] = inst.Args[1]
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run "TestBuildParsesAgent|TestBuildRejectsPlugin|TestBuildRejectsSettings" -v`
Expected: PASS

**Step 5: Run all existing tests to check no regressions**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/planner/planner.go internal/planner/planner_test.go
git commit -m "feat(planner): add AGENT gate with PLUGIN and SETTINGS parsing"
```

---

## Task 4: Planner — parse RUN instruction (no gate)

**Files:**
- Modify: `internal/planner/planner.go`
- Test: `internal/planner/planner_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run TestBuildParsesRunWithoutAgent -v`
Expected: FAIL — `RUN` unsupported instruction.

**Step 3: Write minimal implementation**

Add case in `switch inst.Keyword`:

```go
case "RUN":
    if len(inst.Args) < 1 {
        return nil, fmt.Errorf("line %d: RUN requires a command", inst.Line)
    }
    m.RunSteps = append(m.RunSteps, manifest.RunStep{
        Command: strings.Join(inst.Args, " "),
        Line:    inst.Line,
    })
```

Add `"strings"` to imports if not already present.

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run TestBuildParsesRunWithoutAgent -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/planner/planner.go internal/planner/planner_test.go
git commit -m "feat(planner): add RUN instruction parsing (ungated)"
```

---

## Task 5: Planner — parse INCLUDE on FROM/ATTACH

**Files:**
- Modify: `internal/planner/planner.go` (parseSource)
- Test: `internal/planner/planner_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run TestBuildParsesFromWithInclude -v`
Expected: FAIL — `INCLUDE` is not a recognized source option.

**Step 3: Write minimal implementation**

Modify `parseSource` in `internal/planner/planner.go`. Replace the option-parsing loop:

```go
for i := 2; i < len(inst.Args); {
    switch inst.Args[i] {
    case "AS":
        if i+1 >= len(inst.Args) {
            return manifest.RepoRef{}, fmt.Errorf("line %d: %s AS requires a value", inst.Line, inst.Keyword)
        }
        ref.Alias = inst.Args[i+1]
        i += 2
    case "REF":
        if i+1 >= len(inst.Args) {
            return manifest.RepoRef{}, fmt.Errorf("line %d: %s REF requires a value", inst.Line, inst.Keyword)
        }
        ref.Ref = inst.Args[i+1]
        i += 2
    case "AUTH":
        if i+1 >= len(inst.Args) {
            return manifest.RepoRef{}, fmt.Errorf("line %d: %s AUTH requires a value", inst.Line, inst.Keyword)
        }
        ref.AuthStrategy = inst.Args[i+1]
        i += 2
    case "INCLUDE":
        i++
        for i < len(inst.Args) && !isSourceOption(inst.Args[i]) {
            ref.Includes = append(ref.Includes, inst.Args[i])
            i++
        }
    default:
        return manifest.RepoRef{}, fmt.Errorf("line %d: unsupported %s source option %q", inst.Line, inst.Keyword, inst.Args[i])
    }
}
```

Add helper:

```go
func isSourceOption(s string) bool {
	switch s {
	case "AS", "REF", "AUTH", "INCLUDE":
		return true
	default:
		return false
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run TestBuildParsesFromWithInclude -v`
Expected: PASS

**Step 5: Run all planner tests**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -v`
Expected: All PASS (existing `parseSource` tests still work because they use the key-value pairs which now route through the same switch)

**Step 6: Commit**

```bash
git add internal/planner/planner.go internal/planner/planner_test.go
git commit -m "feat(planner): add INCLUDE support for FROM/ATTACH selective paths"
```

---

## Task 6: Planner — parse COPY instruction

**Files:**
- Modify: `internal/planner/planner.go`
- Test: `internal/planner/planner_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run TestBuildParsesCopyInstruction -v`
Expected: FAIL — `COPY` unsupported instruction.

**Step 3: Write minimal implementation**

Add case in `switch inst.Keyword`:

```go
case "COPY":
    if len(inst.Args) != 2 {
        return nil, fmt.Errorf("line %d: COPY syntax: COPY <source> <dest>", inst.Line)
    }
    m.CopyRules = append(m.CopyRules, manifest.CopyRule{
        Source: inst.Args[0],
        Dest:   inst.Args[1],
    })
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run TestBuildParsesCopyInstruction -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/planner/planner.go internal/planner/planner_test.go
git commit -m "feat(planner): add COPY instruction for fine-grained file selection"
```

---

## Task 7: Planner — parse PLUGIN with all three kinds

**Files:**
- Modify: `internal/planner/planner_test.go`

Note: `parsePlugin` was already implemented in Task 3. This task adds comprehensive tests.

**Step 1: Write the tests**

```go
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
```

**Step 2: Run tests**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run "TestBuildParsesPluginAllKinds|TestBuildRejectsPluginInvalidKind" -v`
Expected: PASS (implementation already done in Task 3)

**Step 3: Commit**

```bash
git add internal/planner/planner_test.go
git commit -m "test(planner): add comprehensive PLUGIN parsing tests for all kinds"
```

---

## Task 8: Planner — parse SETTINGS with MCP control

**Files:**
- Modify: `internal/planner/planner_test.go`

Note: `parseSettings` was already implemented in Task 3. This task adds comprehensive tests.

**Step 1: Write the tests**

```go
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
```

**Step 2: Run tests**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/planner/ -run "TestBuildParsesSettingsKeyValue|TestBuildParsesSettingsMCPSkip" -v`
Expected: PASS (implementation already done in Task 3)

**Step 3: Commit**

```bash
git add internal/planner/planner_test.go
git commit -m "test(planner): add SETTINGS key-value and MCP control tests"
```

---

## Task 9: Materializer — support INCLUDE filtering

**Files:**
- Modify: `internal/materializer/materializer.go`
- Test: `internal/materializer/materializer_test.go`

**Step 1: Write the failing test**

Add to `internal/materializer/materializer_test.go`:

```go
func TestMaterializeWithIncludeFilter(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	// Create source structure:
	// src/api/handler.go
	// src/shared/utils.go
	// src/internal/secret.go  (should be excluded)
	// README.md               (should be excluded)
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

	// Included paths should exist
	for _, path := range []string{"src/api/handler.go", "src/shared/utils.go"} {
		if _, err := os.Stat(filepath.Join(wsDir, path)); err != nil {
			t.Fatalf("expected included file %s: %v", path, err)
		}
	}
	// Excluded paths should NOT exist
	for _, path := range []string{"src/internal/secret.go", "README.md"} {
		if _, err := os.Stat(filepath.Join(wsDir, path)); err == nil {
			t.Fatalf("expected excluded file %s to not exist", path)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/materializer/ -run TestMaterializeWithIncludeFilter -v`
Expected: FAIL — all files are copied because INCLUDE filtering doesn't exist yet.

**Step 3: Write minimal implementation**

In `internal/materializer/materializer.go`, modify the base repo copy logic. When `m.BaseRepo.Includes` is non-empty, instead of copying the entire repo, only copy paths that match the include prefixes:

```go
func shouldInclude(relPath string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	for _, inc := range includes {
		if relPath == inc || strings.HasPrefix(relPath, inc+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
```

Then integrate this into the existing `filepath.WalkDir` call for the base repo copy, adding `shouldInclude(rel, m.BaseRepo.Includes)` as an early-return check. Do the same for attached repos using their respective `Includes` field.

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/materializer/ -run TestMaterializeWithIncludeFilter -v`
Expected: PASS

**Step 5: Run all materializer tests**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/materializer/ -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/materializer/materializer.go internal/materializer/materializer_test.go
git commit -m "feat(materializer): support INCLUDE filtering for selective path inclusion"
```

---

## Task 10: Materializer — support COPY rules

**Files:**
- Modify: `internal/materializer/materializer.go`
- Test: `internal/materializer/materializer_test.go`

**Step 1: Write the failing test**

```go
func TestMaterializeWithCopyRules(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	// Base repo with src/utils/helper.go
	utilsDir := filepath.Join(sourceDir, "src", "utils")
	if err := os.MkdirAll(utilsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(utilsDir, "helper.go"), []byte("package utils"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// External file
	externalFile := filepath.Join(t.TempDir(), "config.json")
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/materializer/ -run TestMaterializeWithCopyRules -v`
Expected: FAIL — CopyRules not processed.

**Step 3: Write minimal implementation**

Add a `processCopyRules` function in `internal/materializer/materializer.go`:

```go
func processCopyRules(wsDir string, m *manifest.WorkspaceManifest) error {
	for _, rule := range m.CopyRules {
		src, err := resolveCopySource(rule.Source, m)
		if err != nil {
			return fmt.Errorf("resolve copy source %q: %w", rule.Source, err)
		}
		dest := filepath.Join(wsDir, rule.Dest)
		if err := copyPath(src, dest); err != nil {
			return fmt.Errorf("copy %q -> %q: %w", rule.Source, rule.Dest, err)
		}
	}
	return nil
}

func resolveCopySource(source string, m *manifest.WorkspaceManifest) (string, error) {
	// Check for alias:path format
	if idx := strings.Index(source, ":"); idx > 0 {
		alias := source[:idx]
		relPath := source[idx+1:]
		repoDir, err := findRepoByAlias(alias, m)
		if err != nil {
			return "", err
		}
		return filepath.Join(repoDir, relPath), nil
	}
	// Absolute or relative path (relative to source dir)
	if filepath.IsAbs(source) {
		return source, nil
	}
	return filepath.Join(m.SourceDir, source), nil
}

func findRepoByAlias(alias string, m *manifest.WorkspaceManifest) (string, error) {
	if m.BaseRepo.Alias == alias {
		return filepath.Join(m.SourceDir, m.BaseRepo.Path), nil
	}
	for _, repo := range m.AttachedRepos {
		if repo.Alias == alias {
			return filepath.Join(m.SourceDir, repo.Path), nil
		}
	}
	return "", fmt.Errorf("unknown repo alias %q", alias)
}

// copyPath copies a file or directory from src to dest
func copyPath(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		// Single file copy
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, info.Mode())
	}
	// Directory copy
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, fi.Mode())
	})
}
```

Call `processCopyRules` in `MaterializeWithOptions` after the base repo and overlay copies, before pruning empty directories:

```go
if err := processCopyRules(wsDir, m); err != nil {
    return fmt.Errorf("process copy rules: %w", err)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/materializer/ -run TestMaterializeWithCopyRules -v`
Expected: PASS

**Step 5: Run all materializer tests**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/materializer/ -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/materializer/materializer.go internal/materializer/materializer_test.go
git commit -m "feat(materializer): add COPY rule processing with alias and absolute path support"
```

---

## Task 11: Exporter — claude-code settings.json generation

**Files:**
- Modify: `internal/exporter/exporter.go`
- Test: `internal/exporter/exporter_test.go`

**Step 1: Write the failing test**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/exporter/ -run "TestWriteBundleGeneratesClaudeSettings|TestWriteBundleSkipsMCPInject" -v`
Expected: FAIL — no `.claude/settings.json` generated.

**Step 3: Write minimal implementation**

Add `writeClaudeSettings` function in `internal/exporter/exporter.go`:

```go
func writeClaudeSettings(dir string, m *manifest.WorkspaceManifest) error {
	if m.Agent == nil || m.Agent.Runtime != "claude-code" {
		return nil
	}

	settings := make(map[string]any)

	// Copy all SETTINGS key-values
	for k, v := range m.Settings {
		settings[k] = v
	}

	// MCP auto-inject
	if m.Agent.MCPInject != "skip" {
		servers := filterMCPServersForRuntime(m.MCPServers, "claude")
		if len(servers) > 0 {
			mcpServers := make(map[string]any)
			for _, s := range servers {
				entry := map[string]any{
					"command": s.Command,
				}
				if len(s.Env) > 0 {
					envMap := make(map[string]string)
					for _, e := range s.Env {
						envMap[e] = ""
					}
					entry["env"] = envMap
				}
				mcpServers[s.Name] = entry
			}
			settings["mcpServers"] = mcpServers
		}
	}

	if len(settings) == 0 {
		return nil
	}

	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}
	return writeJSON(filepath.Join(claudeDir, "settings.json"), settings)
}
```

Call it at the end of `writeClaudeExport`:

```go
if err := writeClaudeSettings(dir, m); err != nil {
    return fmt.Errorf("write claude settings: %w", err)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/exporter/ -run "TestWriteBundleGeneratesClaudeSettings|TestWriteBundleSkipsMCPInject" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/exporter/exporter.go internal/exporter/exporter_test.go
git commit -m "feat(exporter): generate .claude/settings.json with MCP auto-inject"
```

---

## Task 12: Exporter — plugins.json and setup.sh generation

**Files:**
- Modify: `internal/exporter/exporter.go`
- Test: `internal/exporter/exporter_test.go`

**Step 1: Write the failing test**

```go
func TestWriteBundleGeneratesPluginsAndSetup(t *testing.T) {
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
		Plugins: []manifest.PluginRef{
			{Kind: "npm", Source: "@anthropic/superpowers"},
			{Kind: "git", Source: "github:user/repo", Ref: "main"},
			{Kind: "path", Source: "./plugins/local"},
		},
		RunSteps: []manifest.RunStep{
			{Command: "echo setup-done", Line: 10},
		},
		RuntimeExports: []manifest.RuntimeExport{{Runtime: "claude"}},
	}

	if err := WriteBundle(dir, m); err != nil {
		t.Fatalf("WriteBundle() error = %v", err)
	}

	// Check plugins.json
	pluginsPath := filepath.Join(dir, "exports", "claude", "plugins.json")
	data, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatalf("read plugins.json: %v", err)
	}
	var plugins []map[string]any
	if err := json.Unmarshal(data, &plugins); err != nil {
		t.Fatalf("parse plugins.json: %v", err)
	}
	if got := len(plugins); got != 3 {
		t.Fatalf("plugins count = %d, want 3", got)
	}

	// Check setup.sh
	setupPath := filepath.Join(dir, "exports", "claude", "setup.sh")
	setupData, err := os.ReadFile(setupPath)
	if err != nil {
		t.Fatalf("read setup.sh: %v", err)
	}
	setupContent := string(setupData)
	if !strings.Contains(setupContent, "echo setup-done") {
		t.Fatalf("setup.sh missing RUN command, got:\n%s", setupContent)
	}
	if !strings.Contains(setupContent, "claude plugin add") {
		t.Fatalf("setup.sh missing plugin install commands, got:\n%s", setupContent)
	}

	// Check setup.sh is executable
	info, err := os.Stat(setupPath)
	if err != nil {
		t.Fatalf("stat setup.sh: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("setup.sh is not executable")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/exporter/ -run TestWriteBundleGeneratesPluginsAndSetup -v`
Expected: FAIL — no `plugins.json` or `setup.sh`.

**Step 3: Write minimal implementation**

Add two functions in `internal/exporter/exporter.go`:

```go
func writePluginsManifest(dir string, m *manifest.WorkspaceManifest) error {
	if len(m.Plugins) == 0 {
		return nil
	}
	return writeJSON(filepath.Join(dir, "plugins.json"), m.Plugins)
}

func writeSetupScript(dir string, m *manifest.WorkspaceManifest) error {
	if len(m.Plugins) == 0 && len(m.RunSteps) == 0 {
		return nil
	}

	var buf bytes.Buffer
	buf.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n\n")

	if len(m.Plugins) > 0 {
		buf.WriteString("# Install plugins\n")
		for _, p := range m.Plugins {
			switch p.Kind {
			case "npm":
				fmt.Fprintf(&buf, "claude plugin add %s\n", p.Source)
			case "git":
				src := p.Source
				if p.Ref != "" {
					src += "@" + p.Ref
				}
				fmt.Fprintf(&buf, "claude plugin add %s\n", src)
			case "path":
				fmt.Fprintf(&buf, "claude plugin add --path %s\n", p.Source)
			}
		}
		buf.WriteString("\n")
	}

	if len(m.RunSteps) > 0 {
		buf.WriteString("# Run steps\n")
		for _, step := range m.RunSteps {
			fmt.Fprintf(&buf, "%s\n", step.Command)
		}
	}

	return os.WriteFile(filepath.Join(dir, "setup.sh"), buf.Bytes(), 0o755)
}
```

Call both at the end of `writeClaudeExport`:

```go
if err := writePluginsManifest(dir, m); err != nil {
    return fmt.Errorf("write plugins manifest: %w", err)
}
if err := writeSetupScript(dir, m); err != nil {
    return fmt.Errorf("write setup script: %w", err)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/exporter/ -run TestWriteBundleGeneratesPluginsAndSetup -v`
Expected: PASS

**Step 5: Run all exporter tests**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/exporter/ -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/exporter/exporter.go internal/exporter/exporter_test.go
git commit -m "feat(exporter): generate plugins.json and setup.sh for claude-code agent"
```

---

## Task 13: Update example Workspacefile

**Files:**
- Modify: `examples/Workspacefile`

**Step 1: Update the example**

```dockerfile
VERSION 1

NAMESPACE moldplan.pm
NAME pm-service

AGENT claude-code

FROM repo "." INCLUDE "src/api" "src/shared" AS main
ATTACH repo "repos/infra-dashboard" AS dashboard
# ATTACH git "git@github.com:your-org/private-repo.git" REF "main" AUTH "ssh-agent" AS private_tools

COPY main:"src/utils" "utils"

OVERLAY namespace "moldplan.pm"
PROMPT file "prompts/pm-service.md"
MCP FILE "mcp/base.json" MERGE
MCP SERVER "github" COMMAND "node ./mcp/github.js" ENV "GITHUB_TOKEN" RUNTIME "codex" "claude" AUTH "gh"

PLUGIN npm "@anthropic/superpowers"
# PLUGIN git "github:user/claude-plugin" REF "main"
# PLUGIN path "./plugins/my-plugin"

RUN "echo workspace ready"

SETTINGS model "claude-sonnet-4-20250514"
SETTINGS allowedTools "Edit,Write,Bash"
# SETTINGS MCP SKIP

TOOLS mcp "linear_*" "nomad_*" "line_*"
TOOLS cli "git" "rg" "sed"

EXPORT runtime "codex"
EXPORT runtime "claude"
```

**Step 2: Commit**

```bash
git add examples/Workspacefile
git commit -m "docs: update example Workspacefile with new AGENT, PLUGIN, RUN, SETTINGS, COPY, INCLUDE instructions"
```

---

## Task 14: End-to-end integration test

**Files:**
- Create: `internal/integration_test.go`

**Step 1: Write the integration test**

```go
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
	srcAPI := filepath.Join(dir, "src", "api")
	srcShared := filepath.Join(dir, "src", "shared")
	srcInternal := filepath.Join(dir, "src", "internal")
	if err := os.MkdirAll(srcAPI, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcShared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcInternal, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(srcAPI, "handler.go"), []byte("api"), 0o644)
	os.WriteFile(filepath.Join(srcShared, "utils.go"), []byte("shared"), 0o644)
	os.WriteFile(filepath.Join(srcInternal, "secret.go"), []byte("secret"), 0o644)

	// Create Workspacefile content as parsed document
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

	// Planner
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

	// Exporter
	outDir := t.TempDir()
	if err := exporter.WriteBundle(outDir, m); err != nil {
		t.Fatalf("WriteBundle: %v", err)
	}

	// Verify INCLUDE filtering: secret.go should not be in workspace
	wsDir := filepath.Join(outDir, "workspace")
	if _, err := os.Stat(filepath.Join(wsDir, "src", "api", "handler.go")); err != nil {
		t.Fatal("included file src/api/handler.go missing")
	}
	if _, err := os.Stat(filepath.Join(wsDir, "src", "internal", "secret.go")); err == nil {
		t.Fatal("excluded file src/internal/secret.go should not exist")
	}

	// Verify COPY
	if _, err := os.Stat(filepath.Join(wsDir, "shared-copy", "utils.go")); err != nil {
		t.Fatal("COPY target shared-copy/utils.go missing")
	}

	// Verify settings.json
	settingsData, err := os.ReadFile(filepath.Join(outDir, "exports", "claude", ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var settings map[string]any
	json.Unmarshal(settingsData, &settings)
	if settings["model"] != "opus" {
		t.Fatalf("settings.model = %v, want opus", settings["model"])
	}

	// Verify plugins.json
	if _, err := os.Stat(filepath.Join(outDir, "exports", "claude", "plugins.json")); err != nil {
		t.Fatal("plugins.json missing")
	}

	// Verify setup.sh
	setupData, err := os.ReadFile(filepath.Join(outDir, "exports", "claude", "setup.sh"))
	if err != nil {
		t.Fatalf("read setup.sh: %v", err)
	}
	if !strings.Contains(string(setupData), "echo done") {
		t.Fatal("setup.sh missing RUN command")
	}
	if !strings.Contains(string(setupData), "claude plugin add @anthropic/superpowers") {
		t.Fatal("setup.sh missing plugin install")
	}
}
```

**Step 2: Run test**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./internal/ -run TestEndToEndAgentConfigLayer -v`
Expected: PASS

**Step 3: Run full test suite**

Run: `cd /Users/weihung/projects/agent-workspace-engine && go test ./... -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/integration_test.go
git commit -m "test: add end-to-end integration test for agent config layer"
```

---

## Summary

| Task | Component | New Instructions |
|------|-----------|-----------------|
| 1-2  | manifest  | Types: AgentConfig, PluginRef, RunStep, CopyRule, Includes |
| 3    | planner   | AGENT (gate), PLUGIN, SETTINGS |
| 4    | planner   | RUN (ungated) |
| 5    | planner   | INCLUDE on FROM/ATTACH |
| 6    | planner   | COPY |
| 7-8  | planner   | PLUGIN/SETTINGS comprehensive tests |
| 9    | materializer | INCLUDE filtering |
| 10   | materializer | COPY rules |
| 11   | exporter  | settings.json generation |
| 12   | exporter  | plugins.json + setup.sh |
| 13   | examples  | Updated Workspacefile |
| 14   | integration | End-to-end test |
