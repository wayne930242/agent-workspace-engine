# Architecture Notes

## Intent

This project is meant to become a workspace build engine, not an agent orchestration backend.

The engine should answer:

- how a workspace is defined
- how it is built
- what artifacts it exports for different AI runtimes
- how it can later be detached, archived, and restored

## Core Model

1. `Workspacefile`
   The declarative source.

2. `Build plan`
   The normalized internal representation produced by parsing.

3. `Workspace manifest`
   The portable output that downstream runtimes or orchestration systems consume.

4. `Runtime exporters`
   Adapters that translate the manifest into Codex- or Claude-compatible workspace surfaces.

## Separation

- This engine should not know business workflows such as PM sessions or Linear flows.
- This engine should not embed runtime-specific prompt policy in the parser.
- This engine should focus on build semantics and artifact generation.

## Near-Term Next Steps

- add tests for the parser and planner
- formalize the `Workspacefile` grammar
- define exporter output layout for Codex and Claude
- add namespace overlay resolution from local directories
