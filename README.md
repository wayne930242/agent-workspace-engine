# agent-workspace-engine

A workspace engine for AI runtimes.

This project explores a `Dockerfile`-like authoring model for building detachable AI workspaces from a declarative `Workspacefile`.

The initial goal is not to run Claude or Codex directly. The initial goal is to define:

- a stable `Workspacefile` syntax
- a pipeline model for building an agent workspace
- a portable workspace manifest that different runtimes can consume
- a clear separation between base repo content, attached repos, overlays, and runtime-specific outputs
- a safe way to describe private git sources and MCP runtime inputs without embedding secrets

## Why

Current agent systems tend to grow through ad hoc setup paths:

- one path for API-triggered work
- one path for bot-triggered work
- one path for long-lived sessions
- one path for project-specific overlays

This repo treats workspace construction as a first-class build problem.

## Concepts

- `Workspacefile`: declarative build script, inspired by Dockerfile
- `Namespace overlay`: local files layered into the workspace when the namespace matches
- `Build pipeline`: normalized stages such as `FROM`, `ATTACH`, `OVERLAY`, `PROMPT`, `TOOLS`, `EXPORT`
- `Workspace manifest`: machine-readable output describing the built workspace
- `Runtime export`: adapters for Codex, Claude, or future AI runtimes

## DSL

```dockerfile
VERSION 1

NAMESPACE moldplan.pm
NAME pm-service

AGENT claude-code

FROM repo "." INCLUDE "src/api" "src/shared" AS main
ATTACH repo "../infra-dashboard" AS dashboard
# ATTACH git "git@github.com:org/private-repo.git" REF "main" AUTH "ssh-agent" AS private_tools

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

## Planned Scope

Phase 1:

- parse `Workspacefile`
- normalize instructions into a build plan
- output a `workspace-manifest.json`
- produce runtime export directories for Codex and Claude

Phase 2:

- support namespace overlay resolution from local directories
- support attached repo declarations
- support private git repo declarations via `AUTH` strategy metadata
- support MCP file and MCP server declarations in the manifest and runtime exports
- support variable interpolation and environment inputs

Phase 3:

- execute real workspace materialization
- support detachable archive / restore flows
- expose the engine as a library and CLI

## Layout

```text
cmd/awe/                     CLI entrypoint
internal/workspacefile/      parser and AST
internal/planner/            normalized build plan
internal/manifest/           runtime-neutral manifest model
examples/                    sample Workspacefiles
docs/                        design notes
```

## Status

This is an exploratory scaffold, but it now supports:

- parsing a `Workspacefile` with full DSL including `AGENT`, `PLUGIN`, `RUN`, `SETTINGS`, `COPY`, `INCLUDE`
- building a normalized manifest
- recording private git source metadata (`git + REF + AUTH`)
- recording MCP file and MCP server declarations
- selective path inclusion via `INCLUDE` on `FROM`/`ATTACH`
- fine-grained file copying via `COPY` (alias:path or absolute path)
- writing a bundle with:
  - `workspace-manifest.json`
  - `workspace/` materialized content (with INCLUDE filtering and COPY rules applied)
  - `exports/codex/AGENTS.md`
  - `exports/claude/CLAUDE.md`
  - `exports/claude/.claude/settings.json` (MCP auto-inject + SETTINGS)
  - `exports/claude/plugins.json`
  - `exports/claude/setup.sh` (plugin install + RUN steps, executable)
  - runtime metadata and MCP artifacts per runtime

## Private Repos

Private repos are supported at the definition level through `git` sources:

```dockerfile
FROM git "git@github.com:org/private-repo.git" REF "main" AUTH "ssh-agent" AS main
```

Important:

- `Workspacefile` describes the auth mechanism only
- secrets and tokens are never stored in the file
- the actual credential must already exist in the build environment

Examples of `AUTH` values:

- `ssh-agent`
- `gh`
- `glab`
- `inherit`

The current engine will pass through to the local `git` command. If the environment can already clone the private repo, the engine can use it too.

## MCP

Two MCP-related instructions are supported:

```dockerfile
MCP FILE "mcp/base.json" MERGE
MCP SERVER "github" COMMAND "node ./mcp/github.js" ENV "GITHUB_TOKEN" RUNTIME "codex" "claude" AUTH "gh"
```

- `MCP FILE` includes local MCP config files into the manifest and runtime export bundle
- `MCP SERVER` records runtime MCP server definitions in the manifest and per-runtime metadata

The engine currently exports MCP artifacts into each runtime directory under `mcp/`.

## Usage

Install the CLI from a tagged release:

```bash
go install github.com/wayne930242/agent-workspace-engine/cmd/awe@v0.2.0
```

Run locally from source:

Print the normalized manifest:

```bash
go run ./cmd/awe -file examples/Workspacefile
```

Write a bundle to disk:

```bash
go run ./cmd/awe -file examples/Workspacefile -write -out ./build/demo
```

Fail fast when declared auth strategies are unavailable:

```bash
go run ./cmd/awe -file examples/Workspacefile -write -out ./build/demo --strict-auth
```

Bundle output now includes:

- `workspace-manifest.json`
- `control-plane-mapping.json`
- `workspace/` materialized content
- runtime exports for each target
- `mcp/merged.json` when `MCP FILE ... MERGE` is declared
