# Critique of `SPRINT-005-CLAUDE-DRAFT.md`

## Overall Assessment

Claude's draft is strong. It captures the core intent of Sprint 005, stays aligned with the project's constraints (single-file Go CLI, no new external dependencies beyond `goldmark`), and proposes a practical implementation shape that mostly fits the current `wikigen.go` architecture. It is especially good at distinguishing between features that can be derived locally versus the two features that should consume one additional LLM call each.

The draft also shows a better-than-average understanding of the likely implementation surface in `wikigen.go`: prompt extension, post-processing after directory summaries, git-backed analysis, and root wiki assembly are all sensible places to integrate the requested work.

That said, there are several issues worth correcting before this becomes the execution sprint. The biggest problems are:

1. it under-specifies one of the nine flags in places,
2. it assumes a parsing-heavy strategy that may be more brittle than necessary,
3. it overcommits some outputs to `wiki.md` inline instead of separate root artifacts,
4. it references at least one non-repo design document not established by the user's request,
5. it misses some opportunities to leverage Sprint 004's artifact/output model more cleanly.

## What Claude's Draft Gets Right

### 1. Correctly frames the sprint

The overview is crisp and accurately explains the shift from directory-level navigation to repo-level comprehension. That matches the intent document well.

### 2. Correctly preserves cost discipline

Claude explicitly identifies:
- prompt enrichments as "free" in terms of extra LLM calls,
- modification recipes as one extra LLM call,
- entry point traces as one extra LLM call,
- the HTML tree fix as a bug fix.

This is an important planning strength because the intent is explicit about call budgeting.

### 3. Good implementation instinct: post-processing after summaries

The proposed data flow is sensible: let the existing post-order traversal generate/enrich summaries first, then aggregate repo-wide artifacts afterward. That fits the current architecture much better than trying to compute everything during traversal.

### 4. Good attention to git-optional behavior

The draft repeatedly treats git-derived features as optional/failable without breaking the run. That is the right product behavior.

### 5. Good acknowledgment of fragility and risk

The draft openly calls out summary parsing fragility as a major risk. That is honest and important.

## Main Issues

### 1. Flag accounting is inconsistent

The sprint intent names nine features, so there should be nine `--no-*` flags. Claude's narrative understands there are nine features, but the Definition of Done says "All 8 `--no-*` flags parse correctly," which is incorrect.

This matters because the sprint intent is very explicit: all features are on by default, opt-out via `--no-{feature}` flags. The execution sprint should enumerate all nine flags unambiguously.

### 2. Root artifact strategy is muddled

Claude's draft sometimes treats the new outputs as inline sections inside `wiki.md`, and sometimes as standalone generated pages like `recipes.md` and `traces.md`.

That split is not necessarily wrong, but it is under-argued. For LLM usability and consistency, Sprint 005 should decide more clearly whether these features are:
- standalone root artifacts linked from `wiki.md`, or
- inline root sections inside `wiki.md`, or
- a deliberate mix with explicit rationale.

I think the cleaner direction is:
- `wiki.md` stays as the top-level navigation document,
- major generated analyses/indexes become standalone root files,
- `wiki.md` links to them with short summaries.

That keeps `wiki.md` from becoming overly long and makes downstream consumption more modular.

### 3. The parsing strategy is too regex-centric

Claude leans heavily on parsing generated summaries using regexes to recover dependencies, key files, symbols, and boundaries.

This is likely workable for a first pass, but it is brittle in exactly the areas the sprint wants to make more reliable. Since we control the prompt additions, a better plan would be to enforce stable subsection headers and structured bullet conventions in the generated markdown, then parse those sections conservatively. Even better, some artifacts should come from direct repository scanning rather than summary recovery:
- file index should come directly from the scan set,
- symbol index should be at least partly scan-derived,
- churn should come directly from git,
- dependency graph can be summary-derived if the prompt format is disciplined.

The draft hints at this, but it still centers parsing too much as the universal mechanism.

### 4. File naming is only partially thought through

Claude proposes `recipes.md` and `traces.md`, but the rest of the sprint's outputs are described more conceptually than concretely. The sprint should choose a naming convention for all root artifacts up front.

For example, one consistent scheme could be:
- `FILE_INDEX.md`
- `SYMBOL_INDEX.md`
- `DEPENDENCY_GRAPH.md`
- `MODIFICATION_RECIPES.md`
- `ENTRY_POINT_TRACES.md`
- `CHURN.md`

The exact names are less important than consistency. Without that, implementation and clean-up logic get noisier.

### 5. It may overfit to `wiki.md` growth

The draft proposes adding dependency graph, file index, symbol index, and churn directly into `wiki.md`. That could make the root document very large on medium or large repos.

For a tool intended to help LLM comprehension, document chunkability matters. Separate root artifacts are often better than one monolith because they make retrieval and targeted prompting easier.

### 6. Incremental/manifest integration is too shallow

Claude mentions tracking `recipes.md` and `traces.md` hashes in the manifest, but Sprint 005 likely needs a more general rule for derived artifacts.

Questions the execution sprint should answer more explicitly:
- When should each root artifact regenerate?
- Are they all regenerated if any directory summary changes?
- Can some be skipped when only non-git, non-symbol, or non-boundary data changed?
- Does the manifest need per-artifact hashes or just a root-level derived-artifacts hash?

Claude's draft touches this area but doesn't fully plan it.

### 7. The HTML child-navigation fix needs more specificity

Claude correctly includes the bug fix, but the plan does not say enough about *where* the child directories should appear in HTML output and how the links should be derived.

Because this is one of the few fully deterministic tasks in the sprint, the plan should be sharper here:
- which pages get child-nav blocks,
- whether the source is markdown transformation or HTML template injection,
- how relative paths are computed,
- how leaf directories behave.

### 8. External reference is questionable

The References section cites `docs/superpowers/specs/2026-04-05-enhanced-wiki-features-design.md`. That may exist, but it was not part of the user's requested source set. If that file is not committed or canonical, citing it in a sprint draft weakens the document.

The safer references are the ones the user explicitly asked to ground against:
- `docs/sprints/drafts/SPRINT-005-INTENT.md`
- `docs/sprints/README.md`
- `README.md`
- `wikigen.go`

## Recommended Improvements

### 1. Enumerate all nine features and flags explicitly

The draft should include a single authoritative list mapping each feature to its `--no-*` flag. That removes ambiguity and prevents the current "8 vs 9" inconsistency.

### 2. Prefer standalone root artifacts over large inline root sections

Claude should move toward a model where `wiki.md` links to the generated analyses instead of fully embedding all of them. This better matches modular retrieval and keeps the root artifact readable.

### 3. Use structured summary sections, not loose regex extraction

Where parsing summary output is required, the sprint should require stable headings and constrained bullet formats in the prompt contract. Regex should be the fallback parser, not the design center.

### 4. Separate sources of truth by artifact

The sprint should state explicitly:
- file index: direct scan,
- symbol index: direct scan plus summary help if needed,
- dependency graph: summary-derived,
- churn: git-derived,
- modification recipes: git-derived seed + one LLM call,
- entry traces: summary/file-derived seed + one LLM call.

This would make the implementation more robust and easier to test.

### 5. Tighten artifact naming and cleanup expectations

The sprint should specify all root filenames and ensure `--clean` removes all generated markdown/html artifacts consistently.

### 6. Expand validation criteria slightly

Claude's testing section is decent, but the sprint would benefit from a few more specific validations:
- non-git repo still generates non-git artifacts,
- root artifacts render to HTML correctly,
- nested child-directory links resolve correctly,
- missing boundary/testing sections do not break aggregation.

## Bottom Line

Claude's draft is directionally very good and probably executable with moderate revision. It understands the spirit of Sprint 005, respects the codebase's constraints, and proposes a realistic architecture for fitting the work into `wikigen.go`.

My main recommendation is not to replace it, but to tighten it:
- fix the 8-vs-9 feature inconsistency,
- adopt a more explicit root-artifact strategy,
- reduce reliance on free-form regex parsing,
- specify artifact naming and regeneration behavior more clearly.

With those changes, Claude's draft would be a strong execution-ready sprint plan.
