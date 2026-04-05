# Sprint 005 Intent: Enhanced Wiki Features for LLM Comprehension

## Seed

Add 9 features to wikigen that deepen an LLM's ability to navigate AND understand a codebase:
1. Fix HTML tree navigation (child directories aren't linked)
2. Dependency graph (aggregated from per-directory summaries)
3. File index (flat lookup table)
4. Modification recipes (git co-change analysis + 1 LLM call)
5. Entry point traces (1 LLM call)
6. Interface/boundary annotations (added to existing LLM prompt)
7. Test location and pattern hints (added to existing LLM prompt)
8. Symbol index (aggregated from summaries)
9. Churn overlay (git log analysis)

All features on by default, opt-out via `--no-{feature}` flags.

## Context

- wikigen is a single-file Go CLI (~1500 lines in wikigen.go) that generates hierarchical markdown/HTML documentation for codebases
- It uses Claude Haiku for summarization, processes directories in post-order
- Sprint 003 built the core generator; Sprint 004 added --output, incremental mode, manifest system, HTML output
- The tool currently produces directory-level SUMMARY.md files and a root wiki.md
- HTML output has breadcrumbs for upward navigation but no downward child links
- External dependency: only goldmark (markdown → HTML)

## Recent Sprint Context

- **Sprint 003**: Built the core wiki generator — post-order traversal, Claude Haiku summarization, SUMMARY.md per directory, wiki.md at root
- **Sprint 004**: Added --output flag, .wikigen-manifest.json for incremental mode, git-diff and hash-based change detection, HTML output with light/dark mode and breadcrumbs, CLAUDE.md/AGENTS.md skill file generation
- **Sprint 001-002**: Unrelated (sprint tooling rewrite, GTM strategy)

## Relevant Codebase Areas

- `wikigen.go` lines 897-940: Directory summarization prompt (will be modified for boundary + test hints)
- `wikigen.go` lines 1196-1235: Root summarization prompt
- `wikigen.go` lines 1286-1340: buildWiki() — assembles wiki.md (will gain new sections)
- `wikigen.go` lines 1083-1190: HTML generation (writeHTMLPage, buildBreadcrumb — will gain child nav)
- `wikigen.go` lines 386-452: parseArgs() — will gain 8 new opt-out flags
- `wikigen.go` lines 83-107: config and dirBundle types — config gains new booleans

## Constraints

- Must remain a single-file Go program (project convention)
- Only stdlib + goldmark dependency
- Incremental mode must still work — new features integrate with manifest/dirty detection
- New LLM calls limited to 2 (recipes + traces) to keep cost reasonable
- Aggregation features (deps graph, file index, symbol index) parse LLM-generated markdown — must be tolerant of format variations

## Success Criteria

- HTML wiki is fully navigable up and down the directory tree
- An LLM reading wiki.md can determine: which dirs depend on which, look up any file by name, find key symbol definitions, see which areas are actively changing
- An LLM can read recipes.md to know which files to touch for common tasks
- An LLM can read traces.md to understand end-to-end execution flows
- Per-directory summaries include boundary classification and test patterns
- All features can be individually disabled via --no-{feature} flags

## Open Questions

- How robust will the summary parsing be? (regex on LLM-generated markdown)
- Should recipes.md be regenerated on every run or only when git history changes?
- How should incremental mode handle the new separate files (recipes.md, traces.md)?
