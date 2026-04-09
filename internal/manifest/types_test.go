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
		Configure: &CLIConfig{Runtime: "claude-code", MCPInject: "auto"},
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

	if decoded.Configure == nil || decoded.Configure.Runtime != "claude-code" {
		t.Fatalf("Configure = %+v, want runtime claude-code", decoded.Configure)
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
