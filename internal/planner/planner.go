package planner

import (
	"fmt"
	"os"
	"path/filepath"

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

	for i := 2; i < len(inst.Args); i += 2 {
		if i+1 >= len(inst.Args) {
			return manifest.RepoRef{}, fmt.Errorf("line %d: invalid %s source options", inst.Line, inst.Keyword)
		}
		switch inst.Args[i] {
		case "AS":
			ref.Alias = inst.Args[i+1]
		case "REF":
			ref.Ref = inst.Args[i+1]
		case "AUTH":
			ref.AuthStrategy = inst.Args[i+1]
		default:
			return manifest.RepoRef{}, fmt.Errorf("line %d: unsupported %s source option %q", inst.Line, inst.Keyword, inst.Args[i])
		}
	}

	return ref, nil
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
