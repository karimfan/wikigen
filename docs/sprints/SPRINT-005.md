# Sprint 005: Enhanced Wiki Features for LLM Comprehension

## Overview

Sprint 004 delivered a working incremental wiki generator with HTML output, but the wiki only helps an LLM *navigate* to the right directory — it doesn't help the LLM *work* in the codebase once it gets there. An LLM that finds `pkg/api/` still doesn't know which files change together for common tasks, how directories depend on each other, or what the critical execution paths are through the system.

This sprint adds 9 features that bridge the gap from navigation to comprehension. The features fall into four categories:

| Category | Features | Cost |
|----------|----------|------|
| Summary enrichment | Boundary annotations, test hints | No extra LLM calls (prompt additions) |
| Static aggregation | File index, symbol index, dependency graph | Local compute only |
| Git history analysis | Churn overlay, modification recipes | Local compute + 1 LLM call |
| Repo-level synthesis | Entry point traces | 1 LLM call |

Plus one bug fix: HTML child directory navigation.

All features are on by default with opt-out flags. The tool remains a single-file Go program with only goldmark as an external dependency.

## Use Cases

1. **Blast radius assessment**: LLM reads the dependency graph to understand which directories are affected before changing a shared package
2. **File lookup from stack traces**: LLM encounters `handler.go` in an error, looks it up in the file index to find its directory and purpose instantly
3. **Common task execution**: LLM reads recipes to learn "adding a new API endpoint requires touching routes.go, handlers/, queries/, and tests" — derived from actual git co-change history
4. **Flow understanding**: LLM reads traces to understand end-to-end request flow before modifying request handling
5. **API contract awareness**: LLM sees a directory is marked "public" and knows to be careful about breaking changes
6. **Test authoring**: LLM reads test hints to match existing patterns (table-driven, fixtures, etc.)
7. **Symbol navigation**: LLM encounters `PolicySet` type, looks up symbol index to find `pkg/policy/types.go`
8. **Change context**: LLM sees `pkg/api` has high churn (47 commits in 90 days), knows the area is actively changing
9. **Human HTML browsing**: Human navigates up via breadcrumbs AND down via child directory links

## Architecture

### Data Flow

```
  Existing pipeline (unchanged)
  ─────────────────────────────
  collectDirs → computeHashes → detectChanges → propagateDirty
  → for each dirty dir: buildBundle → summarize(LLM) → write SUMMARY.md/html
                                          │
                                          │ prompt now includes:
                                          │ + ## Boundary section
                                          │ + ## Testing section

  New post-processing pipeline (after all summaries)
  ──────────────────────────────────────────────────

  ┌──────────────────────┐     ┌──────────────────────┐
  │ Parse all summaries  │     │ Direct repo scan     │
  │ (section-boundary    │     │ (file list from      │
  │  extraction)         │     │  collectDirs)        │
  └──────────┬───────────┘     └──────────┬───────────┘
             │                            │
  Extract:   │                  Extract:  │
  - Dependencies                - File paths
  - Symbols                     - Languages
  - Boundaries                  - Directories
  - Descriptions (Key files)
             │                            │
             └──────────┬─────────────────┘
                        │
         ┌──────────────┼──────────────────┐
         │              │                  │
   ┌─────▼──────┐  ┌───▼────────┐  ┌─────▼───────��──┐
   │ Standalone  │  │ Git        │  │ LLM calls (2)  │
   │ root files: │  │ analysis:  │  │                │
   │  deps-graph │  │  co-change │  │  recipes       │
   │  file-index │  │  churn     │  │  traces        │
   │  symbol-idx │  │            │  │                │
   └─────────────┘  └────────────┘  └────────────────┘
         │              │                  │
         └──────────────┼──────────────────┘
                        │
                  ┌─────▼──────────┐
                  │ wiki.md links  │
                  │ to all root    │
                  │ artifacts      │
                  └────────────────┘
```

### Output Shape

```
<wiki-root>/
├── wiki.md              (root nav — links to all artifacts)
├── index.html
├── deps-graph.md        (NEW — directory dependency adjacency list)
├── deps-graph.html
├── file-index.md        (NEW — flat file→directory→description table)
├── file-index.html
├── symbol-index.md      (NEW — key types/functions and locations)
├── symbol-index.html
├── recipes.md           (NEW — git co-change derived modification patterns)
├── recipes.html
├── traces.md            (NEW — entry point execution traces)
├── traces.html
├── churn.md             (NEW — recent activity by directory)
├── churn.html
├── .wikigen-manifest.json
└── {dir}/
    ├── SUMMARY.md       (enhanced with Boundary + Testing sections)
    └── SUMMARY.html     (enhanced with child nav links)
```

### Feature → Flag → Data Source Mapping

| # | Feature | Opt-out flag | Data source | Output |
|---|---------|-------------|-------------|--------|
| 1 | HTML tree nav | *(none — bug fix)* | Dir tree from traversal | Child nav in SUMMARY.html |
| 2 | Dependency graph | `--no-deps-graph` | Summary parsing (## Dependencies) | `deps-graph.md` |
| 3 | File index | `--no-file-index` | Direct repo scan + summary parsing (## Key files) | `file-index.md` |
| 4 | Modification recipes | `--no-recipes` | Git co-change + 1 LLM call | `recipes.md` |
| 5 | Entry point traces | `--no-traces` | All summaries + 1 LLM call | `traces.md` |
| 6 | Boundary annotations | `--no-boundaries` | LLM prompt addition | ## Boundary in SUMMARY.md |
| 7 | Test hints | `--no-test-hints` | LLM prompt addition | ## Testing in SUMMARY.md |
| 8 | Symbol index | `--no-symbol-index` | Summary parsing (## Key types) | `symbol-index.md` |
| 9 | Churn overlay | `--no-churn` | Git log analysis | `churn.md` |

### Incremental Regeneration Rules

| Artifact | Regenerate when |
|----------|----------------|
| `deps-graph.md` | Any directory summary changed |
| `file-index.md` | Any directory summary changed |
| `symbol-index.md` | Any directory summary changed |
| `recipes.md` | Git HEAD changed (new commits since last run) |
| `traces.md` | Any directory summary changed |
| `churn.md` | Every run (cheap git command) |

The manifest gains a `root_artifacts` map tracking hashes for each generated root file.

### Modified Directory Summary Structure

Each SUMMARY.md gains two new optional sections in the LLM prompt:

```markdown
## Overview           (existing)
## Key files          (existing)
## How it works       (existing)
## Key types          (existing)
## Dependencies       (existing)
## Boundary           (NEW)
## Testing            (NEW)
## Configuration      (existing)
## When to look here  (existing)
## Child directories  (existing)
```

**Boundary prompt addition:**
```
## Boundary
State whether this directory is: **public** (exported API, meant to be imported
by other packages), **internal** (implementation detail, not meant for direct
external use), or **entry point** (binary/command, not imported).
One sentence on what the contract is — what callers can rely on.
```

**Testing prompt addition:**
```
## Testing
Where tests live (colocated _test.go, separate tests/ dir, or __tests__/).
Note the testing pattern: table-driven, fixtures, mocks, integration vs unit.
If there are test helpers or shared fixtures, name them.
Skip this section if there are no tests in this directory.
```

### wiki.md Final Structure

```markdown
# Codebase Wiki

{LLM-generated root overview}

## Contents
- [dir/](./dir/SUMMARY.md) — one-liner

## Dependency graph
N directories, M dependency edges. [Full graph →](./deps-graph.md)

## File index
N source files across M directories. [Full index →](./file-index.md)

## Symbol index
N key types and functions. [Full index →](./symbol-index.md)

## Recent activity
[Churn analysis (90 days) →](./churn.md)

## Modification recipes
N common change patterns derived from git history. [Full recipes →](./recipes.md)

## Entry point traces
N end-to-end flows. [Full traces →](./traces.md)

---

## {top-level-dir}
{abbreviated summary}
[Full details →](./dir/SUMMARY.md)
```

### HTML Child Navigation

Every SUMMARY.html gets a child navigation block:

```html
<nav class="children">
  <strong>Child directories:</strong>
  <a href="child1/SUMMARY.html">child1/</a>
  <a href="child2/SUMMARY.html">child2/</a>
</nav>
```

- Derived from the directory tree (deterministic, not LLM-dependent)
- Placed after body content, before "Generated by wikigen" footer
- Leaf directories: nav block omitted
- Relative paths computed from current directory depth
- Styled consistently with breadcrumb nav (`.children` CSS class)

## Implementation Plan

### Phase 1: Config & Flag Parsing (~8%)

**Files:**
- `wikigen.go` — config struct (line ~83), parseArgs() (line ~386)

**Tasks:**
- [ ] Add 8 boolean fields to config: `noDepsGraph`, `noFileIndex`, `noRecipes`, `noTraces`, `noBoundaries`, `noTestHints`, `noSymbolIndex`, `noChurn`
- [ ] Add flag parsing for all 8 `--no-*` flags
- [ ] Update `printUsage()` with new flags and descriptions

### Phase 2: LLM Prompt Modifications (~8%)

**Files:**
- `wikigen.go` — summarize() (line ~897)

**Tasks:**
- [ ] Refactor system prompt from string literal to string builder so sections can be conditionally included
- [ ] Add `## Boundary` section (conditional on `!cfg.noBoundaries`)
- [ ] Add `## Testing` section (conditional on `!cfg.noTestHints`)
- [ ] Ensure section headers are stable and parseable (exact `## Boundary` and `## Testing` format)

### Phase 3: HTML Tree Navigation Fix (~10%)

**Files:**
- `wikigen.go` — writeHTMLPage() (line ~1175), HTML template (line ~1084), main loop (line ~231)

**Tasks:**
- [ ] Build `parentChildren map[string][]string` from the dirs list before the main processing loop
- [ ] Add `childDirs []string` parameter to `writeHTMLPage()`
- [ ] Create `buildChildNav(childDirs []string) template.HTML` — generates `<nav class="children">` with relative links to `{child}/SUMMARY.html`
- [ ] Add `.children` CSS class (styled like `.breadcrumb` — muted background, rounded, small text, with `a` tags spaced by ` · ` separator)
- [ ] Modify HTML template to insert child nav after `{{.Body}}`, before `.generated` div
- [ ] Update all call sites of `writeHTMLPage()` to pass child dirs (main loop line ~300, skip path line ~248, root index line ~346)
- [ ] Leaf directories: pass empty slice, nav block omitted

### Phase 4: Summary Parsing Engine (~12%)

**Files:**
- `wikigen.go` — new functions

**Tasks:**
- [ ] Create `extractSection(summary, heading string) string` — finds `## {heading}` and returns content until next `##` or EOF. This is the core parser, used by all extractors.
- [ ] Create `parseDependencies(summary string) (imports []depEdge, importedBy []depEdge)` — extracts from `## Dependencies`, parses `Imports:` and `Imported by:` lines
- [ ] Create `parseKeyFiles(summary, dir string) []fileIndexEntry` — extracts from `## Key files`, parses `- [name](./name) — description` bullets
- [ ] Create `parseSymbols(summary, dir string) []symbolEntry` — extracts from `## Key types and interfaces`, parses type/func/interface mentions with file references
- [ ] Create `parseBoundary(summary string) string` — extracts from `## Boundary`, returns "public"/"internal"/"entry point"/""
- [ ] All parsers return empty results (not errors) on missing/malformed sections

### Phase 5: Direct Repo File Index (~5%)

**Files:**
- `wikigen.go` — new function

**Tasks:**
- [ ] Create `buildFileIndex(cfg config, dirs []string, summaries map[string]string) []fileIndexEntry` — walks all source files from the existing scan, gets language from extLang, gets description from summary parsing when available
- [ ] This is the primary data source for file-index.md; summary parsing provides descriptions only

### Phase 6: Git Analysis (~12%)

**Files:**
- `wikigen.go` — new functions

**Tasks:**
- [ ] Create `analyzeCoChanges(repoRoot string) ([]coChangeCluster, error)`:
  - Run `git log --pretty=format:"%H" --since=1y -500` (cap at 500 commits)
  - For each commit: `git diff-tree --no-commit-id --name-only -r {hash}`
  - Build co-occurrence matrix (file pairs appearing in same commit)
  - Filter: pairs must co-occur in >= 3 commits
  - Greedy clustering: pick highest-frequency pair, expand cluster with files that co-occur with all cluster members
  - Return top 10-15 clusters
- [ ] Create `analyzeChurn(repoRoot string) ([]churnEntry, error)`:
  - Run `git log --since=90d --name-only --pretty=format:""`
  - Count commits per directory
  - Classify: top 20% = high, bottom 40% = low, rest = medium
  - Return sorted by count descending
- [ ] Both return empty results (not errors) if not git repo, git fails, or insufficient history
- [ ] Co-change requires >= 50 commits; churn requires >= 30 days history

### Phase 7: New LLM Calls — Recipes & Traces (~12%)

**Files:**
- `wikigen.go` — new functions

**Tasks:**
- [ ] Create `generateRecipes(apiKey string, clusters []coChangeCluster, summaries map[string]string) (string, error)`:
  - System prompt: "Given file co-change clusters from git history and directory summaries, produce 10-15 modification recipes. Each recipe: a descriptive name (e.g. 'Add a new API endpoint'), the files involved as a bulleted list, and a one-line note per file on what to change there."
  - User prompt: serialized clusters + top-level summary excerpts
  - Skip if clusters empty or `cfg.noRecipes`
- [ ] Create `generateTraces(apiKey string, summaries map[string]string) (string, error)`:
  - System prompt: "Given directory summaries for a codebase, identify the 2-3 most important entry points (main(), HTTP handlers, CLI commands, event listeners) and trace their execution end-to-end, naming specific files and functions at each step."
  - User prompt: all directory summaries
  - Skip if `cfg.noTraces`
- [ ] Write `recipes.md` + `recipes.html` to wiki root
- [ ] Write `traces.md` + `traces.html` to wiki root

### Phase 8: Root Artifact Assembly (~12%)

**Files:**
- `wikigen.go` — new functions, modify buildWiki()

**Tasks:**
- [ ] Create `writeDepsGraph(cfg, summaries) error` — aggregates parseDependencies() results, annotates with boundary info, writes `deps-graph.md` + `deps-graph.html`
- [ ] Create `writeFileIndex(cfg, dirs, summaries) error` — calls buildFileIndex(), writes markdown table sorted by filename to `file-index.md` + `file-index.html`
- [ ] Create `writeSymbolIndex(cfg, summaries) error` — aggregates parseSymbols(), writes markdown table sorted by symbol to `symbol-index.md` + `symbol-index.html`
- [ ] Create `writeChurn(cfg, churnData) error` — writes markdown table to `churn.md` + `churn.html`
- [ ] Modify `buildWiki()` to add one-liner summary + link for each root artifact (conditional on opt-out flags)
- [ ] Each write function returns the content hash for manifest tracking

### Phase 9: Main Loop Integration (~8%)

**Files:**
- `wikigen.go` — main() (line ~144)

**Tasks:**
- [ ] After all directory summaries are generated (line ~310), run parsing/aggregation:
  1. Build parentChildren map (for HTML)
  2. Parse summaries for deps/files/symbols/boundaries
  3. Run git analysis (co-change + churn)
  4. Generate recipes (1 LLM call, if enabled)
  5. Generate traces (1 LLM call, if enabled)
  6. Write root artifacts
  7. Build wiki.md with links to artifacts
- [ ] Update manifest with `root_artifacts` hash map
- [ ] `--clean` mode: remove all new root files (deps-graph, file-index, symbol-index, recipes, traces, churn — both .md and .html)
- [ ] Dry-run mode: skip LLM calls and writes, print what would be generated
- [ ] Failure semantics: if one artifact fails, continue with the rest, report partial failure

### Phase 10: Validation (~5%)

**Tasks:**
- [ ] `go vet wikigen.go` passes
- [ ] `go build wikigen.go` compiles
- [ ] Dry-run mode works with all new flags
- [ ] Each `--no-*` flag individually disables its feature
- [ ] HTML child navigation renders correct relative links
- [ ] HTML child nav omitted for leaf directories
- [ ] Summary parsers handle missing sections gracefully (empty results)
- [ ] Git analysis functions handle non-git repos gracefully (empty results)
- [ ] Non-git repos generate all non-git-dependent artifacts successfully
- [ ] `--clean` removes all new generated files
- [ ] Root artifact links in wiki.md/index.html resolve correctly

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `wikigen.go` | Modify | Config, flags, prompts, HTML nav, parsing, git analysis, LLM calls, root artifacts, wiki assembly |

## Definition of Done

- [ ] `go vet` and `go build` pass
- [ ] All 8 `--no-*` flags parse and disable their respective features
- [ ] HTML SUMMARY pages have clickable child directory links (nav block)
- [ ] Full up/down tree traversal works in HTML (breadcrumbs + child nav)
- [ ] `deps-graph.md` generated with directory dependency adjacency list
- [ ] `file-index.md` generated with all source files, directories, and descriptions
- [ ] `symbol-index.md` generated with key types/functions and locations
- [ ] `churn.md` generated with recent activity by directory
- [ ] `recipes.md` generated with git co-change derived recipes (when git available, >= 50 commits)
- [ ] `traces.md` generated with LLM-traced execution paths
- [ ] Directory summaries include `## Boundary` and `## Testing` sections
- [ ] wiki.md links to all root artifacts with summary counts
- [ ] Incremental mode works — manifest tracks root artifact hashes
- [ ] `--clean` removes all generated files (summaries + root artifacts)
- [ ] Non-git repos generate all non-git-dependent artifacts
- [ ] No new external dependencies (stdlib + goldmark only)

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Summary section parsing fragile | High | Medium | Use section-boundary extraction (## to ##), not line-level regex. Fail gracefully (empty, not error). |
| Co-change analysis slow on large repos | Medium | Low | Cap at 500 recent commits. Only runs once per generation. |
| recipes.md/traces.md quality varies | Medium | Medium | Recipes grounded in real git data. Traces grounded in summaries with "How it works" call chains. |
| wiki.md grows too large | Low | Low | All data in separate files; wiki.md has only one-liner links with counts. |
| Git analysis fails in non-git repos | Low | Low | All git functions return empty (not errors). Non-git artifacts unaffected. |
| Incremental mode confused by new files | Medium | Medium | Manifest gains root_artifacts hash map. Explicit regeneration rules per artifact type. |
| HTML child nav breaks with --output | Low | Medium | Relative paths computed from directory depth, same as breadcrumbs. Test with --output. |

## Security Considerations

- No new API key handling (reuses existing env var)
- Git commands are read-only (`log`, `diff-tree`, `rev-parse`)
- No user input used in shell commands (paths programmatically derived)
- New LLM prompts contain summaries and git metadata, not raw source content
- Generated artifacts are static markdown/HTML, no injection vectors

## Dependencies

- Sprint 004 (completed) — incremental mode, HTML output, manifest system
- Go stdlib + goldmark (no new dependencies)
- Git (optional, for co-change, churn, and recipe features)

## References

- Design spec: `docs/superpowers/specs/2026-04-05-enhanced-wiki-features-design.md`
- Intent: `docs/sprints/drafts/SPRINT-005-INTENT.md`
- Claude draft: `docs/sprints/drafts/SPRINT-005-CLAUDE-DRAFT.md`
- Codex draft: `docs/sprints/drafts/SPRINT-005-CODEX-DRAFT.md`
- Codex critique: `docs/sprints/drafts/SPRINT-005-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- Merge notes: `docs/sprints/drafts/SPRINT-005-MERGE-NOTES.md`
- Current implementation: `wikigen.go` (1,519 lines)
