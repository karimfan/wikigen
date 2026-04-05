# Sprint 005: Enhanced Wiki Features for LLM Comprehension

## Overview

Sprint 004 delivered a working incremental wiki generator with HTML output, but the wiki only helps an LLM *navigate* to the right directory — it doesn't help the LLM *work* in the codebase once it gets there. An LLM that finds `pkg/api/` still doesn't know which files change together for common tasks, how directories depend on each other, or what the critical execution paths are through the system.

This sprint adds 9 features that bridge the gap from navigation to comprehension. Six features are free — they either modify the existing LLM prompt (boundary annotations, test hints) or aggregate data already present in generated summaries (dependency graph, file index, symbol index, churn overlay). Two features require one additional LLM call each (modification recipes, entry point traces). One feature is a bug fix (HTML child directory linking). All features are on by default with opt-out flags.

The tool remains a single-file Go program with only goldmark as an external dependency.

## Use Cases

1. **Blast radius assessment**: LLM reads the dependency graph to understand which directories are affected before making changes to a shared package
2. **File lookup from stack traces**: LLM encounters `handler.go` in an error trace, looks it up in the file index to find its directory and purpose without tree traversal
3. **Common task execution**: LLM reads recipes.md to learn "adding a new API endpoint requires touching routes.go, handlers/, queries/, and tests" — derived from actual git history
4. **Flow understanding**: LLM reads traces.md to understand "a request enters at main.go, routes through router.go, hits handlers, calls the database" before modifying request handling
5. **API contract awareness**: LLM sees a directory is marked "public" with a defined contract, knows to be careful about breaking changes
6. **Test authoring**: LLM writing a new test reads the test hints to match existing patterns (table-driven, fixtures, etc.)
7. **Symbol navigation**: LLM encounters `PolicySet` type, looks up symbol index to find it's defined in `pkg/policy/types.go`
8. **Change context**: LLM sees `pkg/api` has high churn (47 commits in 90 days), knows the area is actively changing and may have in-flight work
9. **Human HTML browsing**: Human opens index.html, clicks through to any directory, navigates up via breadcrumbs and down via child links

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
                                          │
  New post-processing pipeline
  ────────────────────────────
  After all summaries exist:

  ┌─────────────────────┐     ┌──────────────────────┐
  │  Parse all summaries │────►│ Extract:             │
  │  (regex-based)       │     │  - Dependencies      │
  │                      │     │  - Key files         │
  └──────────────────────┘     │  - Symbols           │
                               │  - Boundaries        │
                               └──────────┬───────────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │                     │                     │
              ┌─────▼──────┐    ┌────────▼────────┐    ┌──────▼───────┐
              │ Inline in   │    │ Git analysis:   │    │ LLM calls:   │
              │ wiki.md:    │    │  - co-change    │    │  - recipes   │
              │  - deps     │    │    clusters     │    │  - traces    │
              │  - files    │    │  - churn stats  │    │              │
              │  - symbols  │    │                 │    │              │
              └─────────────┘    └────────┬────────┘    └──────┬───────┘
                                         │                     │
                                  ┌──────▼───────┐    ┌───────▼──────┐
                                  │ Inline in    │    │ Separate:    │
                                  │ wiki.md:     │    │  recipes.md  │
                                  │  - churn     │    │  traces.md   │
                                  └──────────────┘    └──────────────┘

  HTML generation (modified)
  ──────────────────────────
  writeHTMLPage() now receives childDirs []string
  → generates <nav class="children"> block with links
```

### wiki.md Final Structure

```markdown
# Codebase Wiki

{LLM-generated root overview}

## Contents
- [dir/](./dir/SUMMARY.md) — one-liner

## Dependency graph
dir_a → dir_b, dir_c
dir_b (internal) → dir_d (public)

## File index
| File | Directory | Description |
|------|-----------|-------------|
| ... | ... | ... |

## Symbol index
| Symbol | Kind | Location |
|--------|------|----------|
| ... | ... | ... |

## Recent activity
| Directory | Commits (90d) | Churn |
|-----------|---------------|-------|
| ... | ... | ... |

## Modification recipes
Common change patterns derived from git history. [Full recipes →](./recipes.md)

## Entry point traces
End-to-end flows through the codebase. [Full traces →](./traces.md)

---

## {top-level-dir}
{abbreviated summary}
[Full details →](./dir/SUMMARY.md)

---
```

### Modified Directory Summary Structure

Each SUMMARY.md gains two new optional sections:

```markdown
## Overview           (existing)
## Key files          (existing)
## How it works       (existing)
## Key types          (existing)
## Dependencies       (existing)
## Boundary           (NEW - public/internal/entry point)
## Testing            (NEW - patterns, locations, helpers)
## Configuration      (existing)
## When to look here  (existing)
## Child directories  (existing)
```

## Implementation Plan

### Phase 1: Config & Flag Parsing (~10%)

**Files:**
- `wikigen.go` — config struct, parseArgs()

**Tasks:**
- [ ] Add 8 boolean fields to config struct: `noDepsGraph`, `noFileIndex`, `noRecipes`, `noTraces`, `noBoundaries`, `noTestHints`, `noSymbolIndex`, `noChurn`
- [ ] Add flag parsing for `--no-deps-graph`, `--no-file-index`, `--no-recipes`, `--no-traces`, `--no-boundaries`, `--no-test-hints`, `--no-symbol-index`, `--no-churn`
- [ ] Update printUsage() with new flags

### Phase 2: LLM Prompt Modifications (~10%)

**Files:**
- `wikigen.go` — summarize() system prompt

**Tasks:**
- [ ] Add `## Boundary` section to directory summarization system prompt (conditional on `!cfg.noBoundaries`)
- [ ] Add `## Testing` section to directory summarization system prompt (conditional on `!cfg.noTestHints`)
- [ ] Build the system prompt dynamically (string builder) rather than a single string literal, so sections can be conditionally included

### Phase 3: HTML Tree Navigation Fix (~10%)

**Files:**
- `wikigen.go` — writeHTMLPage(), HTML template, main loop

**Tasks:**
- [ ] Add `childDirs []string` parameter to `writeHTMLPage()`
- [ ] Create `buildChildNav(childDirs []string, rel string)` function that generates `<nav class="children">` HTML with links to `{child}/SUMMARY.html`
- [ ] Add CSS for `.children` nav (similar styling to `.breadcrumb`)
- [ ] Insert child nav after body content, before "Generated by wikigen" footer in HTML template
- [ ] Update all call sites of `writeHTMLPage()` to pass child directory lists
- [ ] Build a `parentChildren map[string][]string` from the dirs list before the main loop

### Phase 4: Summary Parsing Engine (~15%)

**Files:**
- `wikigen.go` — new functions

**Tasks:**
- [ ] Create `parseDependencies(summary string) (imports []string, importedBy []string)` — extracts from `## Dependencies` section
- [ ] Create `parseKeyFiles(summary string) []fileIndexEntry` — extracts from `## Key files` section, returns `{name, dir, description}`
- [ ] Create `parseSymbols(summary string) []symbolEntry` — extracts from `## Key types and interfaces` section, returns `{name, kind, location}`
- [ ] Create `parseBoundary(summary string) string` — extracts from `## Boundary` section, returns "public"/"internal"/"entry point"
- [ ] All parsers must be tolerant of markdown format variations (extra whitespace, missing sections, alternate bullet styles)

### Phase 5: Git Analysis (~10%)

**Files:**
- `wikigen.go` — new functions

**Tasks:**
- [ ] Create `analyzeCoChanges(repoRoot string) ([]coChangeCluster, error)` — runs `git log --pretty=format:"%H" --since=1y`, then `git show --name-only --pretty="" {hash}` per commit, builds co-occurrence matrix, clusters top 10-15 groups via simple greedy clustering
- [ ] Create `analyzeChurn(repoRoot string) ([]churnEntry, error)` — runs `git log --since=90d --name-only`, counts commits per directory, classifies high/medium/low
- [ ] Both functions return empty results (not errors) if not a git repo or insufficient history
- [ ] Co-change analysis: require >= 50 commits, files must appear together in >= 3 commits to count

### Phase 6: New LLM Calls — Recipes & Traces (~15%)

**Files:**
- `wikigen.go` — new functions

**Tasks:**
- [ ] Create `generateRecipes(apiKey string, clusters []coChangeCluster, summaries map[string]string) (string, error)` — sends co-change clusters + directory summaries to LLM, gets back named recipes
- [ ] Recipe prompt: "Given these file co-change clusters from git history and directory summaries, produce 10-15 modification recipes. Each recipe: a name, the files involved, and a one-line note per file on what to do there."
- [ ] Create `generateTraces(apiKey string, summaries map[string]string) (string, error)` — sends all directory summaries to LLM, gets back 2-3 end-to-end traces
- [ ] Trace prompt: "Given these directory summaries, identify the 2-3 most important entry points and trace their execution end-to-end through the codebase, naming specific files and functions."
- [ ] Write `recipes.md` + `recipes.html` at wiki root
- [ ] Write `traces.md` + `traces.html` at wiki root
- [ ] Skip recipes if `cfg.noRecipes` or co-change analysis returned empty
- [ ] Skip traces if `cfg.noTraces`

### Phase 7: wiki.md Assembly (~15%)

**Files:**
- `wikigen.go` — buildWiki()

**Tasks:**
- [ ] Refactor `buildWiki()` to accept aggregated data structs (deps, files, symbols, churn, recipe/trace paths)
- [ ] Add `## Dependency graph` section — adjacency list with boundary annotations
- [ ] Add `## File index` section — markdown table sorted by filename
- [ ] Add `## Symbol index` section — markdown table sorted by symbol name
- [ ] Add `## Recent activity` section — markdown table sorted by commit count desc
- [ ] Add `## Modification recipes` section — one-liner + link to recipes.md
- [ ] Add `## Entry point traces` section — one-liner + link to traces.md
- [ ] Each section conditional on its opt-out flag
- [ ] Sections appear after Contents, before per-directory details

### Phase 8: Integration & Main Loop (~10%)

**Files:**
- `wikigen.go` — main()

**Tasks:**
- [ ] After all directory summaries are generated, run the parsing engine to extract deps/files/symbols/boundaries
- [ ] Run git analysis (co-change + churn) in parallel with parsing if possible (or sequentially — single file, no goroutines needed for MVP)
- [ ] Run recipe and trace LLM calls (sequential — 2 API calls)
- [ ] Pass all aggregated data to buildWiki()
- [ ] Update manifest to track recipes.md and traces.md hashes for incremental support
- [ ] Clean mode (`--clean`): also remove recipes.md, traces.md, recipes.html, traces.html

### Phase 9: Testing & Validation (~5%)

**Tasks:**
- [ ] `go vet wikigen.go` passes
- [ ] `go build wikigen.go` compiles
- [ ] Dry-run mode works with new flags
- [ ] Test each `--no-*` flag individually disables its feature
- [ ] Test HTML child navigation renders correct links
- [ ] Test summary parsers handle missing sections gracefully
- [ ] Test git analysis functions handle non-git repos gracefully

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `wikigen.go` | Modify | All changes — config, flags, prompts, HTML nav, parsing, git analysis, LLM calls, wiki assembly |

## Definition of Done

- [ ] `go vet` and `go build` pass
- [ ] All 8 `--no-*` flags parse correctly and disable their respective features
- [ ] HTML SUMMARY pages have clickable child directory links
- [ ] HTML breadcrumbs + child nav allow full up/down tree traversal
- [ ] wiki.md contains: dependency graph, file index, symbol index, churn overlay
- [ ] recipes.md generated with git co-change derived recipes (when git available)
- [ ] traces.md generated with LLM-traced execution paths
- [ ] Directory summaries include Boundary and Testing sections
- [ ] Incremental mode still works — manifest tracks new file hashes
- [ ] `--clean` removes all new generated files
- [ ] Dry-run mode works with all new features
- [ ] No new external dependencies (stdlib + goldmark only)

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Summary parsing fragile against LLM format variations | High | Medium | Use permissive regexes, test against multiple LLM outputs, fail gracefully (empty results, not errors) |
| Co-change analysis slow on large repos | Medium | Low | Cap at 500 most recent commits; only needed once per run |
| recipes.md/traces.md quality depends on LLM | Medium | Medium | Prompt engineering; recipes grounded in real git data; traces grounded in summaries |
| wiki.md becomes too long with all sections | Low | Medium | Tables are compact; recipes/traces are links only; sections are individually opt-out-able |
| Git analysis fails in non-git repos | Low | Low | All git functions return empty results (not errors) when git unavailable |
| Incremental mode confused by new files | Medium | Medium | recipes.md/traces.md tracked in manifest; regenerated when any summary changes |

## Security Considerations

- No new API key handling (reuses existing)
- Git commands are read-only (`log`, `show`, `rev-parse`)
- No user input used in shell commands (all paths are programmatically derived)
- New LLM prompts contain only code summaries and git data, no secrets

## Dependencies

- Sprint 004 (completed) — incremental mode, HTML output, manifest system
- Go stdlib + goldmark (no new deps)
- Git (optional, for co-change and churn analysis)

## References

- Design spec: `docs/superpowers/specs/2026-04-05-enhanced-wiki-features-design.md`
- Current implementation: `wikigen.go` (1,519 lines)
