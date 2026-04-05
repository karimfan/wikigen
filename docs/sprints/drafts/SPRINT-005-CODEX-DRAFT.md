# Sprint 005: Enhanced Wiki Features for LLM Comprehension

## Overview

Sprint 005 expands `wikigen` from a directory-summary generator into a more navigable and analysis-rich codebase wiki for LLMs. The current output gives strong per-directory summaries, but it still leaves several high-value comprehension tasks to the model at query time: locating files and symbols globally, following downward navigation in HTML, inferring architectural boundaries, tracing entry points, identifying test conventions, and reasoning about likely edit surfaces.

This sprint adds nine complementary features that make the generated wiki easier for both humans and LLMs to traverse and reason over. The common theme is aggregation: reuse the existing post-order summarization pipeline, enrich the per-directory prompt where it is already strongest, and synthesize repository-wide indexes and analyses at the root. The result should remain a single-binary Go CLI with minimal new dependencies, preserving `wikigen`'s current simplicity while substantially improving the quality of downstream code understanding.

The sprint also preserves the product philosophy established in Sprints 003 and 004: sensible defaults, deterministic file generation, and optionality through CLI flags rather than separate modes. All new features should be enabled by default and individually disabled with `--no-{feature}` flags so advanced users can trade completeness for runtime or cost.

## Use Cases

1. **LLM navigation in large repositories**: A model can jump directly from a concept to the relevant file, symbol, or child directory without re-scanning the whole wiki
2. **Architectural comprehension**: A model can inspect dependency structure, boundaries, and entry flows from generated root-level artifacts instead of inferring them ad hoc
3. **Edit planning assistance**: A model can use churn and co-change recipes to predict which files should be reviewed together before proposing a modification
4. **Testing-aware code changes**: A model can quickly find likely test locations and infer repository testing patterns from summary hints and aggregated indexes
5. **Human browsing of HTML output**: A user can move downward into child directories directly from rendered HTML summaries, not just upward via breadcrumbs
6. **Incremental understanding with low prompt cost**: Existing directory summaries become richer in one LLM pass, while repo-wide artifacts are assembled mostly from local summaries and git analysis

## Architecture

Sprint 005 adds two classes of output on top of the existing wiki structure:

1. **Enhanced per-directory summaries** generated during the current traversal
2. **Root-level aggregated artifacts** built after traversal from directory summaries, repository scans, and git history

High-level flow:

```text
Repository scan
├── Existing file discovery and filtering
├── Existing post-order directory traversal
│   ├── Summarize leaf directories
│   ├── Summarize parent directories using child summaries
│   └── Enrich prompt with boundaries + testing hints
├── Aggregate repository-wide indexes
│   ├── File index
│   ├── Symbol index
│   ├── Dependency graph
│   ├── Churn overlay
│   └── HTML child-navigation metadata
├── Run focused repo-level analyses
│   ├── Modification recipes (git co-change + 1 LLM call)
│   └── Entry point traces (1 LLM call)
└── Emit markdown + HTML artifacts
    ├── Root wiki.md / index.html
    ├── Per-directory SUMMARY.md / SUMMARY.html
    └── Root-level generated index pages
```

Conceptually, the sprint introduces four new data pipelines:

| Pipeline | Inputs | Output | Cost Profile |
|----------|--------|--------|--------------|
| Summary enrichment | Existing directory bundle + prompt | Boundary and testing annotations inside `SUMMARY.md` | No extra LLM calls |
| Static aggregation | Directory summaries + repo file scan | File index, symbol index, dependency graph | Local compute only |
| Git history analysis | `git log` / co-change data | Churn overlay, modification recipes seed data | Local compute + 1 LLM call for recipes |
| Repo-level synthesis | Existing wiki + selected summary excerpts | Entry point traces | 1 LLM call |

### Output Shape

Expected new root-level artifacts:

```text
<wiki-root>/
├── wiki.md
├── index.html
├── FILE_INDEX.md
├── SYMBOL_INDEX.md
├── DEPENDENCY_GRAPH.md
├── MODIFICATION_RECIPES.md
├── ENTRY_POINT_TRACES.md
├── CHURN.md
└── ...existing mirrored directory summaries...
```

If HTML output remains a rendering of markdown artifacts, these pages should also render to matching `.html` files using the existing goldmark-based pipeline.

## Implementation Plan

### Phase 1: Output model and feature flags (~15%)

**Files:**
- `wikigen.go` - Add feature configuration, CLI flags, artifact generation hooks
- `README.md` - Document new generated artifacts and opt-out flags

**Tasks:**
- [ ] Define internal feature toggles for all nine Sprint 005 capabilities
- [ ] Add CLI flags: `--no-html-tree-nav`, `--no-dependency-graph`, `--no-file-index`, `--no-modification-recipes`, `--no-entry-point-traces`, `--no-boundary-annotations`, `--no-test-hints`, `--no-symbol-index`, `--no-churn`
- [ ] Ensure all new features default to enabled
- [ ] Add a root-artifact generation stage after directory summaries complete
- [ ] Establish naming conventions for new root markdown/html outputs
- [ ] Decide and implement failure semantics: if one optional artifact fails, continue generating the rest and report partial failure clearly

### Phase 2: Fix downward HTML tree navigation (~10%)

**Files:**
- `wikigen.go` - Update HTML rendering/template logic for child-directory links

**Tasks:**
- [ ] Inspect current HTML generation path for `SUMMARY.html` and root `index.html`
- [ ] Identify why child directories are not currently linked from rendered pages
- [ ] Add child-directory navigation blocks to rendered HTML using known tree structure from traversal
- [ ] Ensure links resolve correctly for nested directories and custom `--output` locations
- [ ] Keep existing breadcrumbs for upward navigation intact
- [ ] Preserve graceful behavior for leaf directories with no children

### Phase 3: Enrich directory summaries in the existing LLM pass (~15%)

**Files:**
- `wikigen.go` - Extend summarization prompt and parsing expectations

**Tasks:**
- [ ] Extend the existing Claude prompt to ask for interface/boundary annotations at directory scope
- [ ] Extend the prompt to ask for test location and repository test-pattern hints when evidence exists
- [ ] Keep prompt additions concise so token growth remains bounded
- [ ] Ensure the generated markdown uses stable section names that later aggregation can parse or locate reliably
- [ ] Preserve compatibility with older directories that may lack explicit interfaces or tests
- [ ] Update root wiki assembly so enriched sections survive unchanged into downstream HTML rendering

### Phase 4: Build static repository indexes (~20%)

**Files:**
- `wikigen.go` - Add repository scan helpers and root artifact writers

**Tasks:**
- [ ] Implement a flat file index artifact containing path, file type/language, and owning directory summary reference
- [ ] Implement a symbol index artifact aggregated from per-directory summaries and direct file scans where needed
- [ ] Define lightweight symbol extraction heuristics by language using existing extension knowledge and conservative regexes
- [ ] Aggregate dependency notes from directory summaries into a root dependency graph artifact
- [ ] Choose a markdown representation for the dependency graph that is LLM-friendly (adjacency lists or layered sections rather than only prose)
- [ ] Cross-link generated indexes back to relevant directory summaries and source-relative paths

### Phase 5: Add git-history derived analyses (~20%)

**Files:**
- `wikigen.go` - Add git analysis helpers and artifact generation

**Tasks:**
- [ ] Implement churn analysis from `git log` to surface frequently modified files/directories over a bounded recent history window
- [ ] Emit a churn overlay artifact that highlights hotspots and likely unstable areas
- [ ] Implement co-change analysis from git history to identify files that are often edited together
- [ ] Distill co-change results into structured inputs for a single LLM call that produces modification recipes
- [ ] Ensure recipe output emphasizes probable change sets, validation steps, and caution areas rather than speculative certainty
- [ ] Handle non-git repositories or git-command failures gracefully by skipping churn/recipe artifacts with clear messaging

### Phase 6: Generate entry-point traces (~10%)

**Files:**
- `wikigen.go` - Add entry-point candidate detection and repo-level LLM synthesis

**Tasks:**
- [ ] Detect likely entry points using conventional filenames and language-aware heuristics (`main`, CLI commands, server bootstrap files, test runners, package entry modules)
- [ ] Gather the most relevant summary excerpts and file references for each candidate entry point
- [ ] Use one LLM call to synthesize concise entry-point traces that explain startup flow and major handoff boundaries
- [ ] Keep traces grounded in observed files and summaries rather than broad architectural speculation
- [ ] Support multiple entry points when a repo has CLI, server, worker, or library surfaces

### Phase 7: Root wiki integration and rendering (~5%)

**Files:**
- `wikigen.go` - Integrate new artifacts into root wiki and HTML generation

**Tasks:**
- [ ] Link all generated root artifacts from `wiki.md`
- [ ] Ensure corresponding `.html` pages are emitted and reachable from `index.html`
- [ ] Keep root wiki readable by summarizing the purpose of each artifact before linking it
- [ ] Maintain consistent markdown formatting so artifacts are easy for both humans and LLMs to parse

### Phase 8: Validation and documentation (~5%)

**Tasks:**
- [ ] Run `go test ./...` if tests exist and are applicable
- [ ] Run `go run wikigen.go --help` and verify all new flags are documented
- [ ] Validate markdown-only generation and HTML generation paths
- [ ] Validate behavior on a git repository and on a non-git directory
- [ ] Sanity-check that generated links work for nested child summaries and root-level artifacts
- [ ] Update `README.md` with examples of the new outputs and recommended usage for LLM-oriented workflows

## API Endpoints (if applicable)

Not applicable.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `wikigen.go` | Modify | Add feature flags, summary enrichment, aggregation pipelines, git analyses, new root artifacts, and HTML child navigation |
| `README.md` | Modify | Document Sprint 005 outputs, behavior, and opt-out flags |

## Definition of Done

- [ ] HTML output includes working downward child-directory navigation links
- [ ] Root wiki links to all enabled Sprint 005 artifacts
- [ ] File index is generated by default and covers the repository scan set
- [ ] Symbol index is generated by default and is materially useful for lookup
- [ ] Dependency graph is generated by default from aggregated summary data
- [ ] Directory summaries include boundary annotations unless the feature is disabled
- [ ] Directory summaries include test hints when supported by available evidence unless the feature is disabled
- [ ] Modification recipes are generated with exactly one additional LLM call when git history is available
- [ ] Entry point traces are generated with exactly one additional LLM call when candidate entry points are found
- [ ] Churn overlay is generated from git history when available
- [ ] Each new feature can be disabled independently via `--no-{feature}` flags
- [ ] Non-git repositories still generate all non-git-dependent artifacts successfully
- [ ] Existing markdown and HTML generation behavior remains backward compatible for repos that do not use the new artifacts
- [ ] `README.md` documents the new outputs and flags

## Security Considerations

- Git history analysis must only inspect local repository metadata and should not transmit commit history verbatim unless required for concise synthesis
- New LLM prompts should minimize inclusion of unnecessary source content, especially for modification recipes and entry traces
- Symbol extraction should use conservative parsing heuristics and avoid executing repository code or build steps
- Generated dependency and recipe artifacts should present uncertainty clearly to avoid overconfident architectural claims
- HTML navigation additions must preserve correct relative-link handling and avoid introducing unsafe raw HTML content

## Dependencies

- Sprint 003: Core post-order traversal and summarization pipeline
- Sprint 004: Output-path handling, HTML rendering, manifest/incremental infrastructure, and repository-aware generation flow
- Local `git` availability for churn and co-change features
- Existing Claude summarization integration for prompt enrichment and the two new repo-level synthesis calls

## References

- `README.md`
- `wikigen.go`
- `docs/sprints/SPRINT-003.md`
- `docs/sprints/SPRINT-004.md`
- `docs/sprints/drafts/SPRINT-005-INTENT.md`
