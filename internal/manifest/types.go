package manifest

type WorkspaceManifest struct {
	Version           string             `json:"version"`
	Namespace         string             `json:"namespace"`
	Name              string             `json:"name"`
	SourceDir         string             `json:"source_dir"`
	BaseRepo          RepoRef            `json:"base_repo"`
	AttachedRepos     []RepoRef          `json:"attached_repos,omitempty"`
	NamespaceOverlays []NamespaceOverlay `json:"namespace_overlays,omitempty"`
	ResolvedOverlays  []ResolvedOverlay  `json:"resolved_overlays,omitempty"`
	Prompt            *PromptRef         `json:"prompt,omitempty"`
	PromptContent     string             `json:"prompt_content,omitempty"`
	MCPFiles          []MCPFile          `json:"mcp_files,omitempty"`
	MCPServers        []MCPServer        `json:"mcp_servers,omitempty"`
	AuthChecks        []AuthCheck        `json:"auth_checks,omitempty"`
	ToolPolicies      []ToolPolicy       `json:"tool_policies,omitempty"`
	RuntimeExports    []RuntimeExport    `json:"runtime_exports,omitempty"`
	PipelineStages    []string           `json:"pipeline_stages"`
	Agent             *AgentConfig       `json:"agent,omitempty"`
	Plugins           []PluginRef        `json:"plugins,omitempty"`
	RunSteps          []RunStep          `json:"run_steps,omitempty"`
	Settings          map[string]string  `json:"settings,omitempty"`
	CopyRules         []CopyRule         `json:"copy_rules,omitempty"`
}

type RepoRef struct {
	Kind         string   `json:"kind"`
	Path         string   `json:"path,omitempty"`
	URL          string   `json:"url,omitempty"`
	Alias        string   `json:"alias,omitempty"`
	Ref          string   `json:"ref,omitempty"`
	AuthStrategy string   `json:"auth_strategy,omitempty"`
	Includes     []string `json:"includes,omitempty"`
}

type NamespaceOverlay struct {
	Namespace string `json:"namespace"`
}

type ResolvedOverlay struct {
	Namespace string `json:"namespace"`
	SourceDir string `json:"source_dir"`
}

type PromptRef struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type MCPFile struct {
	Path    string `json:"path"`
	Merge   bool   `json:"merge,omitempty"`
	Content string `json:"content,omitempty"`
}

type MCPServer struct {
	Name           string   `json:"name"`
	Command        string   `json:"command"`
	Env            []string `json:"env,omitempty"`
	RuntimeTargets []string `json:"runtime_targets,omitempty"`
	AuthStrategy   string   `json:"auth_strategy,omitempty"`
}

type AuthCheck struct {
	Scope     string `json:"scope"`
	Target    string `json:"target"`
	Strategy  string `json:"strategy"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

type ToolPolicy struct {
	Kind   string   `json:"kind"`
	Values []string `json:"values"`
}

type RuntimeExport struct {
	Runtime string `json:"runtime"`
}

type AgentConfig struct {
	Runtime   string `json:"runtime,omitempty"`
	MCPInject string `json:"mcp_inject,omitempty"`
}

type PluginRef struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Ref    string `json:"ref,omitempty"`
}

type RunStep struct {
	Command string `json:"command"`
	Line    int    `json:"line,omitempty"`
}

type CopyRule struct {
	Source string `json:"source"`
	Dest   string `json:"dest,omitempty"`
}
