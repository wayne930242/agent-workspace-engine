package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wayne930242/agent-workspace-engine/internal/manifest"
	"github.com/wayne930242/agent-workspace-engine/internal/workspacefile"
)

func Build(doc *workspacefile.Document) (*manifest.WorkspaceManifest, error) {
	m := &manifest.WorkspaceManifest{
		Version:   "1",
		SourceDir: filepath.Dir(doc.Source),
		PipelineStages: []string{
			"resolve_definition",
			"resolve_base_repo",
			"attach_repos",
			"apply_namespace_overlays",
			"generate_runtime_config",
			"export_runtimes",
		},
	}

	var agentDeclared bool

	for _, inst := range doc.Instructions {
		switch inst.Keyword {
		case "VERSION":
			if len(inst.Args) != 1 {
				return nil, fmt.Errorf("line %d: VERSION requires exactly one argument", inst.Line)
			}
			m.Version = inst.Args[0]
		case "NAMESPACE":
			if len(inst.Args) != 1 {
				return nil, fmt.Errorf("line %d: NAMESPACE requires exactly one argument", inst.Line)
			}
			m.Namespace = inst.Args[0]
		case "NAME":
			if len(inst.Args) != 1 {
				return nil, fmt.Errorf("line %d: NAME requires exactly one argument", inst.Line)
			}
			m.Name = inst.Args[0]
		case "FROM":
			repo, err := parseSource(inst)
			if err != nil {
				return nil, err
			}
			m.BaseRepo = repo
		case "ATTACH":
			repo, err := parseSource(inst)
			if err != nil {
				return nil, err
			}
			m.AttachedRepos = append(m.AttachedRepos, repo)
		case "OVERLAY":
			if len(inst.Args) != 2 || inst.Args[0] != "namespace" {
				return nil, fmt.Errorf("line %d: OVERLAY syntax must be: OVERLAY namespace <name>", inst.Line)
			}
			m.NamespaceOverlays = append(m.NamespaceOverlays, manifest.NamespaceOverlay{Namespace: inst.Args[1]})
		case "PROMPT":
			if len(inst.Args) != 2 {
				return nil, fmt.Errorf("line %d: PROMPT syntax must be: PROMPT <kind> <path>", inst.Line)
			}
			m.Prompt = &manifest.PromptRef{Kind: inst.Args[0], Path: inst.Args[1]}
		case "MCP":
			if err := parseMCP(inst, m); err != nil {
				return nil, err
			}
		case "TOOLS":
			if len(inst.Args) < 2 {
				return nil, fmt.Errorf("line %d: TOOLS syntax must be: TOOLS <kind> <value...>", inst.Line)
			}
			m.ToolPolicies = append(m.ToolPolicies, manifest.ToolPolicy{
				Kind:   inst.Args[0],
				Values: inst.Args[1:],
			})
		case "EXPORT":
			if len(inst.Args) != 2 || inst.Args[0] != "runtime" {
				return nil, fmt.Errorf("line %d: EXPORT syntax must be: EXPORT runtime <name>", inst.Line)
			}
			m.RuntimeExports = append(m.RuntimeExports, manifest.RuntimeExport{Runtime: inst.Args[1]})
		case "RUN":
			if len(inst.Args) < 1 {
				return nil, fmt.Errorf("line %d: RUN requires a command", inst.Line)
			}
			m.RunSteps = append(m.RunSteps, manifest.RunStep{
				Command: strings.Join(inst.Args, " "),
				Line:    inst.Line,
			})
		case "COPY":
			if len(inst.Args) != 2 {
				return nil, fmt.Errorf("line %d: COPY syntax: COPY <source> <dest>", inst.Line)
			}
			m.CopyRules = append(m.CopyRules, manifest.CopyRule{
				Source: inst.Args[0],
				Dest:   inst.Args[1],
			})
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
		default:
			return nil, fmt.Errorf("line %d: unsupported instruction %q", inst.Line, inst.Keyword)
		}
	}

	if m.Namespace == "" {
		return nil, fmt.Errorf("missing NAMESPACE instruction")
	}
	if m.Name == "" {
		return nil, fmt.Errorf("missing NAME instruction")
	}
	if m.BaseRepo.Kind == "" || (m.BaseRepo.Path == "" && m.BaseRepo.URL == "") {
		return nil, fmt.Errorf("missing FROM instruction")
	}
	if err := resolvePrompt(m); err != nil {
		return nil, err
	}
	if err := resolveMCPFiles(m); err != nil {
		return nil, err
	}
	resolveOverlays(m)

	return m, nil
}

func parseSource(inst workspacefile.Instruction) (manifest.RepoRef, error) {
	if len(inst.Args) < 2 {
		return manifest.RepoRef{}, fmt.Errorf("line %d: %s syntax must start with: %s <repo|git> <target>", inst.Line, inst.Keyword, inst.Keyword)
	}

	ref := manifest.RepoRef{Kind: inst.Args[0]}
	switch ref.Kind {
	case "repo":
		ref.Path = inst.Args[1]
	case "git":
		ref.URL = inst.Args[1]
	default:
		return manifest.RepoRef{}, fmt.Errorf("line %d: %s source kind must be repo or git", inst.Line, inst.Keyword)
	}

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

	return ref, nil
}

func isSourceOption(s string) bool {
	switch s {
	case "AS", "REF", "AUTH", "INCLUDE":
		return true
	default:
		return false
	}
}

func parseMCP(inst workspacefile.Instruction, m *manifest.WorkspaceManifest) error {
	if len(inst.Args) < 1 {
		return fmt.Errorf("line %d: MCP requires a subcommand", inst.Line)
	}

	switch inst.Args[0] {
	case "FILE":
		if len(inst.Args) < 2 {
			return fmt.Errorf("line %d: MCP FILE syntax must be: MCP FILE <path> [MERGE]", inst.Line)
		}
		file := manifest.MCPFile{Path: inst.Args[1]}
		for _, arg := range inst.Args[2:] {
			if arg == "MERGE" {
				file.Merge = true
				continue
			}
			return fmt.Errorf("line %d: unsupported MCP FILE option %q", inst.Line, arg)
		}
		m.MCPFiles = append(m.MCPFiles, file)
		return nil
	case "SERVER":
		if len(inst.Args) < 4 || inst.Args[2] != "COMMAND" {
			return fmt.Errorf("line %d: MCP SERVER syntax must be: MCP SERVER <name> COMMAND <command> [ENV <vars...>] [RUNTIME <targets...>] [AUTH <strategy>]", inst.Line)
		}
		server := manifest.MCPServer{
			Name:    inst.Args[1],
			Command: inst.Args[3],
		}
		i := 4
		for i < len(inst.Args) {
			switch inst.Args[i] {
			case "ENV":
				i++
				for i < len(inst.Args) && !isMCPServerOption(inst.Args[i]) {
					server.Env = append(server.Env, inst.Args[i])
					i++
				}
			case "RUNTIME":
				i++
				for i < len(inst.Args) && !isMCPServerOption(inst.Args[i]) {
					server.RuntimeTargets = append(server.RuntimeTargets, inst.Args[i])
					i++
				}
			case "AUTH":
				if i+1 >= len(inst.Args) {
					return fmt.Errorf("line %d: MCP SERVER AUTH requires a value", inst.Line)
				}
				server.AuthStrategy = inst.Args[i+1]
				i += 2
			default:
				return fmt.Errorf("line %d: unsupported MCP SERVER option %q", inst.Line, inst.Args[i])
			}
		}
		m.MCPServers = append(m.MCPServers, server)
		return nil
	default:
		return fmt.Errorf("line %d: unsupported MCP subcommand %q", inst.Line, inst.Args[0])
	}
}

func isMCPServerOption(s string) bool {
	switch s {
	case "ENV", "RUNTIME", "AUTH":
		return true
	default:
		return false
	}
}

func resolvePrompt(m *manifest.WorkspaceManifest) error {
	if m.Prompt == nil || m.Prompt.Kind != "file" {
		return nil
	}

	path := filepath.Join(m.SourceDir, m.Prompt.Path)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read prompt file %q: %w", m.Prompt.Path, err)
	}
	m.PromptContent = string(content)
	return nil
}

func resolveOverlays(m *manifest.WorkspaceManifest) {
	for _, overlay := range m.NamespaceOverlays {
		for _, sourceDir := range overlayCandidates(m.SourceDir, overlay.Namespace) {
			if info, err := os.Stat(sourceDir); err == nil && info.IsDir() {
				m.ResolvedOverlays = append(m.ResolvedOverlays, manifest.ResolvedOverlay{
					Namespace: overlay.Namespace,
					SourceDir: sourceDir,
				})
				break
			}
		}
	}
}

func resolveMCPFiles(m *manifest.WorkspaceManifest) error {
	for i := range m.MCPFiles {
		path := filepath.Join(m.SourceDir, m.MCPFiles[i].Path)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read MCP file %q: %w", m.MCPFiles[i].Path, err)
		}
		m.MCPFiles[i].Content = string(content)
	}
	return nil
}

func overlayCandidates(sourceDir, namespace string) []string {
	return []string{
		filepath.Join(sourceDir, "overlays", "namespaces", namespace),
		filepath.Join(sourceDir, "namespaces", namespace),
		filepath.Join(sourceDir, ".awe", "namespaces", namespace),
	}
}

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
	if len(inst.Args) > 2 {
		return fmt.Errorf("line %d: SETTINGS key-value takes exactly two arguments, got %d", inst.Line, len(inst.Args))
	}
	m.Settings[inst.Args[0]] = inst.Args[1]
	return nil
}
