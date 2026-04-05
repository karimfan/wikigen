# Sprint 005 Merge Notes

## Claude Draft Strengths
- Clear cost analysis: correctly identified 2 new LLM calls, rest free
- Detailed implementation plan with line-number references to existing code
- Good risk table with concrete mitigations
- Proper integration with existing incremental/manifest system

## Codex Draft Strengths
- Better artifact strategy: standalone root files instead of inlining everything in wiki.md
- More robust data sourcing: file index from direct scan, not just summary parsing
- Consistent naming convention (SCREAMING_SNAKE for root artifacts)
- Better security considerations (minimize source content in LLM prompts)
- Cleaner pipeline taxonomy (4 pipeline types table)

## Valid Critiques Accepted

1. **8-vs-9 flag count inconsistency**: Fixed. HTML tree nav is a bug fix with no opt-out, so there are 8 opt-out flags for 9 features. Made explicit.

2. **Root artifact strategy is muddled**: Accepted. Codex is right that inlining dependency graph, file index, symbol index, and churn in wiki.md will bloat it on larger repos. Changed to: all major indexes become standalone files, wiki.md links to them with one-liner summaries. This matches the hybrid approach the user approved during brainstorming (compact references inline, details in separate files).

3. **Parsing too regex-centric**: Partially accepted. File index SHOULD come from direct repo scan (we already have the file list). Symbol index should be summary-derived though — we'd need language-specific AST parsing to do it from source, which violates the "no new dependencies" constraint. Dependency graph stays summary-derived since import analysis would also need per-language parsing. The fix: enforce stable section headers in the prompt, use section-boundary parsing (find header, read until next header) rather than line-by-line regex.

4. **Artifact naming needs consistency**: Accepted. Adopted lowercase-with-hyphens convention: `file-index.md`, `symbol-index.md`, `deps-graph.md`, `recipes.md`, `traces.md`, `churn.md`. Not SCREAMING_SNAKE — that clashes with the existing `wiki.md` and `SUMMARY.md` convention.

5. **Incremental/manifest integration too shallow**: Accepted. Added explicit regeneration rules per artifact.

6. **HTML fix needs more specificity**: Accepted. Expanded with relative path computation details.

## Critiques Rejected (with reasoning)

1. **Symbol index should be partly scan-derived**: Rejected. Language-specific symbol extraction from source would need per-language regexes or AST parsing, adding significant complexity for marginal gain. The LLM already identifies key symbols in its summary — parsing that is simpler and language-agnostic.

2. **Separate all artifacts into standalone files**: Partially rejected. The user explicitly approved Approach C (hybrid) during brainstorming — compact data inlined, verbose data in separate files. But on reflection, even the "compact" data (file index, symbol index) can get large on bigger repos. Compromise: put them in separate files with summary counts inline in wiki.md.

3. **`--no-html-tree-nav` flag**: Rejected. This is a bug fix, not a feature. It should always be on. No opt-out.

## Interview Refinements Applied
- User confirmed Approach C (hybrid inline + separate files) — adjusted to lean more toward separate files per Codex's valid critique about wiki.md bloat
- User confirmed all features on by default with opt-out flags
- User explicitly requested HTML child directory linking as a bug fix

## Final Decisions

1. **All 9 features get standalone root files** (except boundary/test hints which live in SUMMARY.md). wiki.md links to each with a count/summary line.
2. **File index sourced from direct repo scan**, not summary parsing. Language and directory come from the scan; description comes from summary parsing (best of both).
3. **Section-boundary parsing** for extracting data from summaries: find `## Header`, read lines until next `##`. More robust than line-level regex.
4. **8 opt-out flags** (HTML fix has no opt-out). Enumerated explicitly.
5. **Artifact regeneration rules**: file-index/symbol-index/deps-graph regenerate when any summary changes. recipes regenerate when git HEAD changes. churn regenerates every run. traces regenerate when any summary changes.
6. **File naming**: `file-index.md`, `symbol-index.md`, `deps-graph.md`, `recipes.md`, `traces.md`, `churn.md` (all lowercase-hyphenated, matching wiki.md convention).
