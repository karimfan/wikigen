# wikigen

A Go CLI that generates hierarchical markdown and HTML documentation for any codebase, optimized for LLM navigation and comprehension. It traverses a repository bottom-up, sends source files to the Claude API for summarization, and produces a navigable wiki with per-directory summaries, cross-repository indexes, and git-derived analysis.

## How it works

1. Walks the directory tree in post-order (deepest directories first)
2. At each directory, reads source files and detects their language
3. Sends file contents + child directory summaries to Claude Haiku for summarization
4. Writes a `SUMMARY.md` and `SUMMARY.html` in the wiki output folder for each directory
5. Generates root-level artifacts: dependency graph, file index, symbol index, churn overlay
6. Analyzes git co-change history and generates modification recipes (1 LLM call)
7. Generates entry point traces showing end-to-end execution flows (1 LLM call)
8. Generates a root-level project overview via a separate LLM call
9. Generates `wiki.md` and `index.html` linking everything together
10. Writes `CLAUDE.md` and `AGENTS.md` skill files into the repo root to teach LLMs how to use the wiki
11. Saves a `.wikigen-manifest.json` so subsequent runs only regenerate changed directories

## Requirements

- Go 1.21+
- `ANTHROPIC_API_KEY` environment variable set
- Git (optional, for co-change recipes, churn overlay, and fast incremental detection)

## Usage

```bash
# Full scan — read from <repo>, write wiki to <output>
go run . --output /path/to/wiki /path/to/repo

# Preview what would be scanned (no API calls, no files written)
go run . --dry-run --output /path/to/wiki /path/to/repo

# Incremental run (automatic when manifest exists from a previous run)
go run . --output /path/to/wiki /path/to/repo

# Force full regeneration, ignoring the manifest
go run . --full --output /path/to/wiki /path/to/repo

# Remove all generated files from the wiki folder
go run . --clean --output /path/to/wiki /path/to/repo

# Skip expensive features (saves 2 LLM calls)
go run . --no-recipes --no-traces --output /path/to/wiki /path/to/repo

# Minimal run — only directory summaries, no extras
go run . --no-deps-graph --no-file-index --no-recipes --no-traces \
         --no-boundaries --no-test-hints --no-symbol-index --no-churn \
         --output /path/to/wiki /path/to/repo
```

If `--output` is omitted, the wiki is written in-place inside the repo.

## Flags

| Flag | Description |
|------|-------------|
| `--output <dir>` | Wiki output directory (default: in-place in repo) |
| `--exclude <name>` | Exclude a directory by base name (repeatable) |
| `--no-default-excludes` | Clear default exclusions (only `.git` is still skipped) |
| `--base-url <url>` | URL prefix for source file links (e.g. `https://github.com/org/repo/blob/main`) |
| `--full` | Force full regeneration, ignore manifest |
| `--json` | Emit line-delimited JSON progress events on stderr |
| `--dry-run` | Show what would be summarized without calling the API |
| `--clean` | Remove all generated files and manifest from the wiki folder |
| `--no-deps-graph` | Skip dependency graph generation (`deps-graph.md`) |
| `--no-file-index` | Skip file index generation (`file-index.md`) |
| `--no-recipes` | Skip modification recipes (`recipes.md`, saves 1 LLM call) |
| `--no-traces` | Skip entry point traces (`traces.md`, saves 1 LLM call) |
| `--no-boundaries` | Omit `## Boundary` section from directory summaries |
| `--no-test-hints` | Omit `## Testing` section from directory summaries |
| `--no-symbol-index` | Skip symbol index generation (`symbol-index.md`) |
| `--no-churn` | Skip churn overlay generation (`churn.md`) |

## Enhanced wiki features

All features below are **on by default**. Use the `--no-*` flags to disable individual features.

### Dependency graph (`deps-graph.md`)

A text-based adjacency list showing how directories depend on each other. Extracted from the `## Dependencies` section of each directory summary and annotated with boundary classifications (public/internal/entry point).

```
pkg/auth (public) → pkg/db, pkg/config
pkg/api (public) → pkg/auth, pkg/db, pkg/middleware
cmd/server (entry point) → pkg/api, pkg/config
```

Useful for blast radius assessment — an LLM can see which directories are affected before changing a shared package.

Disable: `--no-deps-graph`

### File index (`file-index.md`)

A flat table mapping every source file to its directory, language, and a one-liner description. Built from a direct repo scan (file paths and languages) enriched with descriptions from directory summaries.

| File | Directory | Language | Description |
|------|-----------|----------|-------------|
| handler.go | pkg/api | Go | HTTP request handlers for the REST API |
| store.go | pkg/db | Go | Database persistence layer |

Useful for stack trace lookup — an LLM can find any file's location and purpose without tree traversal.

Disable: `--no-file-index`

### Symbol index (`symbol-index.md`)

Key exported types, interfaces, and functions with their locations. Extracted from the `## Key types and interfaces` section of each directory summary.

| Symbol | Kind | Location |
|--------|------|----------|
| `PolicySet` | type | pkg/policy/types.go |
| `Compile()` | func | pkg/cedar/compiler.go |
| `Store` | interface | pkg/db/store.go |

Disable: `--no-symbol-index`

### Modification recipes (`recipes.md`)

"To add a new X, touch files A, B, C." Derived from git co-change history — files that appear in the same commits frequently are clustered, then an LLM call turns the clusters into named recipes.

Requires: Git repo with 50+ commits in the last year.

Costs: 1 additional LLM call.

Disable: `--no-recipes`

### Entry point traces (`traces.md`)

2-3 critical end-to-end execution paths traced from entry points through the codebase, naming specific files and functions at each step.

Costs: 1 additional LLM call.

Disable: `--no-traces`

### Boundary annotations

Each directory summary includes a `## Boundary` section classifying the directory as **public** (exported API), **internal** (implementation detail), or **entry point** (binary/command). Helps an LLM avoid coupling to internals or breaking public contracts.

Disable: `--no-boundaries`

### Test hints

Each directory summary includes a `## Testing` section noting where tests live (colocated, separate directory), the testing pattern (table-driven, fixtures, mocks), and any test helpers. Helps an LLM write tests that match the project's existing style.

Disable: `--no-test-hints`

### Churn overlay (`churn.md`)

Directories ranked by recent commit frequency over the last 90 days, classified as high/medium/low churn.

| Directory | Commits (90d) | Churn |
|-----------|---------------|-------|
| pkg/api | 47 | high |
| pkg/auth | 23 | medium |
| pkg/config | 2 | low |

Requires: Git repo with 30+ days of history.

Disable: `--no-churn`

## LLM cost

| Feature | LLM calls |
|---------|-----------|
| Directory summaries | 1 per dirty directory |
| Root overview | 1 |
| Modification recipes | 1 (opt-out: `--no-recipes`) |
| Entry point traces | 1 (opt-out: `--no-traces`) |
| All other features | 0 (local aggregation or git analysis) |

For a repo with 50 directories on a full scan, expect ~52 LLM calls (50 dirs + 1 root + 1 recipes). Incremental runs only regenerate changed directories plus recipes/traces.

## HTML output

Alongside every markdown file, wikigen generates a styled HTML page:

- `index.html` — browsable version of `wiki.md`
- `{dir}/SUMMARY.html` — browsable version of each `SUMMARY.md`
- `deps-graph.html`, `file-index.html`, `symbol-index.html`, `recipes.html`, `traces.html`, `churn.html` — browsable versions of all root artifacts

Features:
- Light/dark mode (follows system preference)
- Breadcrumb navigation (upward through the tree)
- Child directory navigation (downward links to children)
- All `.md` links rewritten to `.html` for seamless browser navigation
- Responsive layout, monospace code blocks

Open the wiki in a browser:

```bash
open /path/to/wiki/index.html
```

## LLM skill files

wikigen automatically writes `CLAUDE.md` and `AGENTS.md` into the **target repo root** (not the wiki output directory). These files teach Claude Code and Codex how to navigate the wiki.

The skill files contain:
- The concrete path to `wiki.md` relative to the repo root
- Step-by-step navigation instructions (read wiki.md first, drill into SUMMARY.md, read source last)
- A visual navigation pattern showing the top-down hierarchy

**Safe for existing files.** If a `CLAUDE.md` or `AGENTS.md` already exists in the repo, wikigen **appends** a clearly delimited section using marker comments:

```markdown
<!-- wikigen:start -->
# Codebase Navigation via Wiki
...
<!-- wikigen:end -->
```

- On subsequent runs, only the content between these markers is updated
- `--clean` removes just the wikigen section, leaving the rest of the file intact
- If the file was entirely wikigen-generated and `--clean` is run, the file is deleted

## Navigation-oriented summaries

Each summary is designed for LLM task routing, not just documentation:

- **Project overview** (root wiki.md): Architecture snapshot, navigation guide mapping task categories to directories, and links to all root artifacts
- **Directory summaries**: Include "When to look here" (task routing), "Boundary" (API contract), "Testing" (test patterns), "Dependencies" (imports/imported by), and "How it works" (internal flow)
- **Annotated contents**: Every directory listing includes one-line descriptions, not just bare links

This lets an LLM match a task description to the right code location in 2-3 file reads instead of searching the entire codebase.

## Default exclusions

These directories are excluded by default (in addition to all dot-prefixed directories):

- `.claude`
- `.codex`
- `docs`

Override with `--no-default-excludes` and add your own with `--exclude`:

```bash
go run . --no-default-excludes --exclude vendor --exclude node_modules --output ./wiki /path/to/repo
```

## Incremental mode

On the first run, wikigen scans the entire tree and writes `.wikigen-manifest.json` to the wiki folder. This manifest records SHA-256 hashes of every source file and generated summary.

On subsequent runs:

- If the repo is a git repository, wikigen uses `git diff` to detect changed files (fast)
- Otherwise, it compares file hashes against the manifest (portable)
- Only directories with changed files are re-summarized
- Changes propagate upward — modifying a leaf file regenerates its directory, all ancestor directories, and `wiki.md`
- Root artifacts (deps-graph, file-index, etc.) are regenerated when any summary changes

## Output structure

Given a repo like:

```
myrepo/
  cmd/
    main.go
  internal/
    server/
      handler.go
      router.go
    db/
      store.go
```

wikigen produces:

```
wiki-output/
  wiki.md                          # Top-level overview with navigation guide
  index.html                       # Browsable HTML version of wiki.md
  deps-graph.md                    # Directory dependency adjacency list
  deps-graph.html
  file-index.md                    # Flat file lookup table
  file-index.html
  symbol-index.md                  # Key types/functions and locations
  symbol-index.html
  recipes.md                       # Git co-change derived modification patterns
  recipes.html
  traces.md                        # Entry point execution traces
  traces.html
  churn.md                         # Recent activity by directory
  churn.html
  .wikigen-manifest.json           # Incremental state
  cmd/
    SUMMARY.md                     # Summary of cmd/
    SUMMARY.html                   # Browsable HTML with child nav
  internal/
    SUMMARY.md                     # Summary of internal/ (references children)
    SUMMARY.html
    server/
      SUMMARY.md                   # Summary of server package
      SUMMARY.html
    db/
      SUMMARY.md                   # Summary of db package
      SUMMARY.html

myrepo/
  CLAUDE.md                        # Wiki navigation instructions for Claude Code
  AGENTS.md                        # Wiki navigation instructions for Codex
```

## CI/CD integration

wikigen is designed to run in CI pipelines. Use `--json` for machine-readable progress:

```bash
go run . --json --output ./wiki /path/to/repo 2>progress.jsonl
```

Each line is a JSON event:

```json
{"event":"regenerate","dir":"internal/server","status":"dirty","files_changed":2}
{"event":"skip","dir":"internal/db","status":"unchanged"}
{"event":"done","message":"Regenerated 3/12 directories (5 skipped)"}
```

Exit codes: `0` = success, `1` = error, `2` = partial failure (some directories failed).

## Supported languages

Go, Swift, TypeScript, JavaScript, C, Python, Shell, Make, Docker, YAML, TOML, JSON.

Binary files, generated files (containing "Code generated" or "DO NOT EDIT"), lock files (`go.sum`, `pnpm-lock.yaml`, `package-lock.json`), and files over 100KB are automatically skipped.
