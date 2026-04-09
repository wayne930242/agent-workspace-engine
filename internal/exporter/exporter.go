package exporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wayne930242/agent-workspace-engine/internal/authcheck"
	"github.com/wayne930242/agent-workspace-engine/internal/manifest"
	"github.com/wayne930242/agent-workspace-engine/internal/materializer"
)

type Options struct {
	StrictAuth bool
}

func WriteBundle(outDir string, m *manifest.WorkspaceManifest) error {
	return WriteBundleWithOptions(outDir, m, Options{})
}

func WriteBundleWithOptions(outDir string, m *manifest.WorkspaceManifest, opts Options) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	collectAuthChecks(m)

	if err := materializer.MaterializeWithOptions(outDir, m, materializer.Options{
		StrictAuth: opts.StrictAuth,
	}); err != nil {
		return fmt.Errorf("materialize workspace: %w", err)
	}

	manifestPath := filepath.Join(outDir, "workspace-manifest.json")
	if err := writeJSON(manifestPath, m); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	exportsDir := filepath.Join(outDir, "exports")
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		return fmt.Errorf("create exports directory: %w", err)
	}

	for _, runtime := range m.RuntimeExports {
		switch runtime.Runtime {
		case "codex":
			if err := writeCodexExport(filepath.Join(exportsDir, "codex"), m); err != nil {
				return err
			}
		case "claude":
			if err := writeClaudeExport(filepath.Join(exportsDir, "claude"), m); err != nil {
				return err
			}
		case "gemini":
			if err := writeGeminiExport(filepath.Join(exportsDir, "gemini"), m); err != nil {
				return err
			}
		case "cursor":
			if err := writeCursorExport(filepath.Join(exportsDir, "cursor"), m); err != nil {
				return err
			}
		case "windsurf":
			if err := writeWindsurfExport(filepath.Join(exportsDir, "windsurf"), m); err != nil {
				return err
			}
		case "amp":
			if err := writeAmpExport(filepath.Join(exportsDir, "amp"), m); err != nil {
				return err
			}
		default:
			if err := writeGenericExport(filepath.Join(exportsDir, runtime.Runtime), runtime.Runtime, m); err != nil {
				return err
			}
		}
	}

	if err := writeControlPlaneMapping(outDir, m); err != nil {
		return fmt.Errorf("write control-plane mapping: %w", err)
	}

	return nil
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeCodexExport(dir string, m *manifest.WorkspaceManifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create codex export directory: %w", err)
	}

	content := agentFileContent("codex", "Codex", "AGENTS.md", m)
	if err := writeIfNotExists(filepath.Join(dir, "AGENTS.md"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write codex AGENTS.md: %w", err)
	}
	if err := writePromptFile(dir, m); err != nil {
		return fmt.Errorf("write codex prompt file: %w", err)
	}
	if err := writeMCPArtifacts(dir, "codex", m); err != nil {
		return fmt.Errorf("write codex MCP artifacts: %w", err)
	}
	return writeRuntimeMetadata(dir, "codex", m)
}

func writeRuntimeMetadata(dir, runtime string, m *manifest.WorkspaceManifest) error {
	metadata := map[string]any{
		"runtime":           runtime,
		"workspace_name":    m.Name,
		"namespace":         m.Namespace,
		"base_repo":         baseRepoDescriptor(m.BaseRepo),
		"workspace_dir":     filepath.Join("..", "..", "workspace"),
		"resolved_overlays": m.ResolvedOverlays,
		"tool_policies":     m.ToolPolicies,
		"mcp_files":         filterMCPFilesForRuntime(m.MCPFiles, runtime),
		"mcp_servers":       filterMCPServersForRuntime(m.MCPServers, runtime),
	}
	return writeJSON(filepath.Join(dir, "runtime.json"), metadata)
}

func writePromptFile(dir string, m *manifest.WorkspaceManifest) error {
	if m.PromptContent == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(dir, "PROMPT.md"), []byte(m.PromptContent), 0o644)
}

func writeClaudeExport(dir string, m *manifest.WorkspaceManifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create claude export directory: %w", err)
	}

	content := agentFileContent("claude", "Claude", "CLAUDE.md", m)
	if err := writeIfNotExists(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write claude CLAUDE.md: %w", err)
	}
	if err := writePromptFile(dir, m); err != nil {
		return fmt.Errorf("write claude prompt file: %w", err)
	}
	if err := writeMCPArtifacts(dir, "claude", m); err != nil {
		return fmt.Errorf("write claude MCP artifacts: %w", err)
	}
	if err := writeRuntimeMetadata(dir, "claude", m); err != nil {
		return fmt.Errorf("write claude runtime metadata: %w", err)
	}
	if err := writeClaudeSettings(dir, m); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}
	if err := writePluginsManifest(dir, m); err != nil {
		return fmt.Errorf("write plugins manifest: %w", err)
	}
	if err := writeSetupScript(dir, m); err != nil {
		return fmt.Errorf("write setup script: %w", err)
	}
	return nil
}

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

func writeClaudeSettings(dir string, m *manifest.WorkspaceManifest) error {
	if m.Configure == nil || m.Configure.Runtime != "claude-code" {
		return nil
	}

	settings := make(map[string]any)

	for k, v := range m.Settings {
		settings[k] = v
	}

	if m.Configure.MCPInject != "skip" {
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

	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}
	return writeJSON(filepath.Join(claudeDir, "settings.json"), settings)
}

// writeIfNotExists writes content to path only if the file does not already exist.
func writeIfNotExists(path string, content []byte, mode os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return nil // file exists, skip
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, mode)
}

func agentFileContent(runtime, runtimeLabel, runtimeFile string, m *manifest.WorkspaceManifest) string {
	content := strings.TrimSpace(fmt.Sprintf(`# Generated By agent-workspace-engine

Workspace: %s
Namespace: %s

Base repo: %s
Workspace dir: ../../workspace

This export is a %s-facing workspace descriptor.
Use workspace-manifest.json as the canonical machine-readable source.
`, m.Name, m.Namespace, m.BaseRepo.Path, runtimeLabel)) + "\n"

	if m.PromptContent != "" {
		content += "\n## Prompt\n\n" + strings.TrimSpace(m.PromptContent) + "\n"
	}

	return content
}

func writeGeminiExport(dir string, m *manifest.WorkspaceManifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create gemini export directory: %w", err)
	}

	content := agentFileContent("gemini", "Gemini CLI", "GEMINI.md", m)
	if err := writeIfNotExists(filepath.Join(dir, "GEMINI.md"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write gemini GEMINI.md: %w", err)
	}
	if err := writePromptFile(dir, m); err != nil {
		return fmt.Errorf("write gemini prompt file: %w", err)
	}
	if err := writeMCPArtifacts(dir, "gemini", m); err != nil {
		return fmt.Errorf("write gemini MCP artifacts: %w", err)
	}
	return writeRuntimeMetadata(dir, "gemini", m)
}

func writeCursorExport(dir string, m *manifest.WorkspaceManifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cursor export directory: %w", err)
	}

	body := agentFileContent("cursor", "Cursor", ".cursor/rules/workspace.mdc", m)
	frontmatter := fmt.Sprintf("---\ndescription: Workspace rules for %s (%s)\nalwaysApply: true\n---\n\n", m.Name, m.Namespace)
	mdcContent := frontmatter + body

	mdcPath := filepath.Join(dir, ".cursor", "rules", "workspace.mdc")
	if err := os.MkdirAll(filepath.Dir(mdcPath), 0o755); err != nil {
		return fmt.Errorf("create cursor rules directory: %w", err)
	}
	if err := writeIfNotExists(mdcPath, []byte(mdcContent), 0o644); err != nil {
		return fmt.Errorf("write cursor workspace.mdc: %w", err)
	}
	if err := writePromptFile(dir, m); err != nil {
		return fmt.Errorf("write cursor prompt file: %w", err)
	}
	if err := writeMCPArtifacts(dir, "cursor", m); err != nil {
		return fmt.Errorf("write cursor MCP artifacts: %w", err)
	}
	return writeRuntimeMetadata(dir, "cursor", m)
}

func writeWindsurfExport(dir string, m *manifest.WorkspaceManifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create windsurf export directory: %w", err)
	}

	content := agentFileContent("windsurf", "Windsurf", ".windsurf/rules/workspace.md", m)
	rulesPath := filepath.Join(dir, ".windsurf", "rules", "workspace.md")
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0o755); err != nil {
		return fmt.Errorf("create windsurf rules directory: %w", err)
	}
	if err := writeIfNotExists(rulesPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write windsurf workspace.md: %w", err)
	}
	if err := writePromptFile(dir, m); err != nil {
		return fmt.Errorf("write windsurf prompt file: %w", err)
	}
	if err := writeMCPArtifacts(dir, "windsurf", m); err != nil {
		return fmt.Errorf("write windsurf MCP artifacts: %w", err)
	}
	return writeRuntimeMetadata(dir, "windsurf", m)
}

func writeAmpExport(dir string, m *manifest.WorkspaceManifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create amp export directory: %w", err)
	}

	content := agentFileContent("amp", "Amp", "AGENTS.md", m)
	if err := writeIfNotExists(filepath.Join(dir, "AGENTS.md"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write amp AGENTS.md: %w", err)
	}
	if err := writePromptFile(dir, m); err != nil {
		return fmt.Errorf("write amp prompt file: %w", err)
	}
	if err := writeMCPArtifacts(dir, "amp", m); err != nil {
		return fmt.Errorf("write amp MCP artifacts: %w", err)
	}
	return writeRuntimeMetadata(dir, "amp", m)
}

func writeGenericExport(dir, runtime string, m *manifest.WorkspaceManifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create generic export directory: %w", err)
	}

	content := strings.TrimSpace(fmt.Sprintf(`
runtime=%s
workspace=%s
namespace=%s
base_repo=%s
workspace_dir=../../workspace
`, runtime, m.Name, m.Namespace, m.BaseRepo.Path)) + "\n"

	if err := os.WriteFile(filepath.Join(dir, "runtime.txt"), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write generic runtime export: %w", err)
	}
	if err := writePromptFile(dir, m); err != nil {
		return fmt.Errorf("write generic prompt file: %w", err)
	}
	if err := writeMCPArtifacts(dir, runtime, m); err != nil {
		return fmt.Errorf("write generic MCP artifacts: %w", err)
	}
	return writeRuntimeMetadata(dir, runtime, m)
}

func writeMCPArtifacts(dir, runtime string, m *manifest.WorkspaceManifest) error {
	files := filterMCPFilesForRuntime(m.MCPFiles, runtime)
	servers := filterMCPServersForRuntime(m.MCPServers, runtime)

	if len(files) == 0 && len(servers) == 0 {
		return nil
	}

	mcpDir := filepath.Join(dir, "mcp")
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		return err
	}

	for _, file := range files {
		target := filepath.Join(mcpDir, normalizeMCPFilePath(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			return err
		}
	}

	merged, mergedSources, err := mergeMCPFiles(files)
	if err != nil {
		return fmt.Errorf("merge MCP files: %w", err)
	}
	if merged != nil {
		mergedBytes, err := json.MarshalIndent(merged, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal merged MCP config: %w", err)
		}
		if err := os.WriteFile(filepath.Join(mcpDir, "merged.json"), append(mergedBytes, '\n'), 0o644); err != nil {
			return err
		}
	}

	manifestData := map[string]any{
		"files":          files,
		"servers":        servers,
		"merged_sources": mergedSources,
	}
	return writeJSON(filepath.Join(mcpDir, "mcp-manifest.json"), manifestData)
}

func filterMCPFilesForRuntime(files []manifest.MCPFile, _ string) []manifest.MCPFile {
	return files
}

func filterMCPServersForRuntime(servers []manifest.MCPServer, runtime string) []manifest.MCPServer {
	if len(servers) == 0 {
		return nil
	}
	var filtered []manifest.MCPServer
	for _, server := range servers {
		if len(server.RuntimeTargets) == 0 {
			filtered = append(filtered, server)
			continue
		}
		for _, target := range server.RuntimeTargets {
			if target == runtime {
				filtered = append(filtered, server)
				break
			}
		}
	}
	return filtered
}

func baseRepoDescriptor(repo manifest.RepoRef) map[string]string {
	result := map[string]string{
		"kind": repo.Kind,
	}
	if repo.Path != "" {
		result["path"] = repo.Path
	}
	if repo.URL != "" {
		result["url"] = repo.URL
	}
	if repo.Ref != "" {
		result["ref"] = repo.Ref
	}
	if repo.AuthStrategy != "" {
		result["auth_strategy"] = repo.AuthStrategy
	}
	return result
}

func normalizeMCPFilePath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	clean = strings.TrimPrefix(clean, "/")
	if strings.HasPrefix(clean, "mcp/") {
		return strings.TrimPrefix(clean, "mcp/")
	}
	if clean == "mcp" {
		return "default.json"
	}
	return clean
}

func mergeMCPFiles(files []manifest.MCPFile) (map[string]any, []string, error) {
	var merged map[string]any
	var sources []string

	for _, file := range files {
		if !file.Merge {
			continue
		}

		var obj map[string]any
		dec := json.NewDecoder(bytes.NewBufferString(file.Content))
		dec.UseNumber()
		if err := dec.Decode(&obj); err != nil {
			return nil, nil, fmt.Errorf("parse merge source %q: %w", file.Path, err)
		}

		if merged == nil {
			merged = obj
		} else {
			deepMerge(merged, obj)
		}
		sources = append(sources, file.Path)
	}

	return merged, sources, nil
}

func deepMerge(dst map[string]any, src map[string]any) {
	for key, srcVal := range src {
		if dstMap, ok := dst[key].(map[string]any); ok {
			if srcMap, ok := srcVal.(map[string]any); ok {
				deepMerge(dstMap, srcMap)
				continue
			}
		}
		dst[key] = srcVal
	}
}

func collectAuthChecks(m *manifest.WorkspaceManifest) {
	var checks []manifest.AuthCheck

	appendRepoCheck := func(scope string, target string, strategy string) {
		if strings.TrimSpace(strategy) == "" {
			return
		}
		result := authcheck.Check(strategy)
		checks = append(checks, manifest.AuthCheck{
			Scope:     scope,
			Target:    target,
			Strategy:  strategy,
			Available: result.Available,
			Detail:    result.Detail,
		})
	}

	appendRepoCheck("base_repo", nonEmpty(m.BaseRepo.URL, m.BaseRepo.Path), m.BaseRepo.AuthStrategy)
	for _, repo := range m.AttachedRepos {
		target := nonEmpty(repo.URL, repo.Path)
		appendRepoCheck("attached_repo", target, repo.AuthStrategy)
	}
	for _, server := range m.MCPServers {
		appendRepoCheck("mcp_server", server.Name, server.AuthStrategy)
	}

	m.AuthChecks = checks
}

func nonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func writeControlPlaneMapping(outDir string, m *manifest.WorkspaceManifest) error {
	workspaceID := fmt.Sprintf("%s/%s", m.Namespace, m.Name)

	var handlers []map[string]any
	var sessions []map[string]any
	for _, runtime := range m.RuntimeExports {
		handlerID := fmt.Sprintf("%s.%s", runtime.Runtime, workspaceID)
		sessionKey := fmt.Sprintf("%s:default", handlerID)
		handlers = append(handlers, map[string]any{
			"handler_id":   handlerID,
			"platform":     runtime.Runtime,
			"workspace_id": workspaceID,
			"mode":         "api",
		})
		sessions = append(sessions, map[string]any{
			"handler_id":     handlerID,
			"workspace_id":   workspaceID,
			"session_policy": "reuse",
			"session_key":    sessionKey,
		})
	}

	mapping := map[string]any{
		"workspace_id":     workspaceID,
		"namespace":        m.Namespace,
		"workspace_name":   m.Name,
		"handlers":         handlers,
		"session_mappings": sessions,
		"auth_checks":      m.AuthChecks,
	}

	return writeJSON(filepath.Join(outDir, "control-plane-mapping.json"), mapping)
}
