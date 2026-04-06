# wikigen

Generates hierarchical markdown and HTML documentation for any codebase, optimized for LLM navigation and comprehension. Traverses a repository bottom-up, sends source files to the Claude API for summarization, and produces a navigable wiki with per-directory summaries, cross-repository indexes, and git-derived analysis.

## How it works

1. Walks the directory tree in post-order (deepest directories first)
2. At each directory, reads source files and detects their language
3. Sends file contents + child directory summaries to Claude Haiku for summarization
4. Writes a `SUMMARY.md` and `SUMMARY.html` for each directory
5. Generates root-level artifacts: dependency graph, file index, symbol index, churn overlay
6. Analyzes git co-change history and generates modification recipes (1 LLM call)
7. Generates entry point traces showing end-to-end execution flows (1 LLM call)
8. Generates a root-level project overview via a separate LLM call
9. Generates `wiki.md` and `index.html` linking everything together
10. Writes `CLAUDE.md` and `AGENTS.md` skill files into the repo root
11. Saves a `.wikigen-manifest.json` for incremental runs

## Requirements

- Go 1.21+
- `ANTHROPIC_API_KEY` environment variable
- Git (optional, for co-change recipes, churn overlay, and fast incremental detection)

## Usage

```bash
# Full scan — write wiki from /path/to/repo into /path/to/wiki
go run ./wikigen/ --output /path/to/wiki /path/to/repo

# Preview what would be scanned (no API calls, no files written)
go run ./wikigen/ --dry-run --output /path/to/wiki /path/to/repo

# Incremental run (automatic when manifest exists)
go run ./wikigen/ --output /path/to/wiki /path/to/repo

# Force full regeneration
go run ./wikigen/ --full --output /path/to/wiki /path/to/repo

# Remove all generated files
go run ./wikigen/ --clean --output /path/to/wiki /path/to/repo

# Skip expensive features (saves 2 LLM calls)
go run ./wikigen/ --no-recipes --no-traces --output /path/to/wiki /path/to/repo

# Minimal run — directory summaries only, no extras
go run ./wikigen/ --no-deps-graph --no-file-index --no-recipes --no-traces \
                  --no-boundaries --no-test-hints --no-symbol-index --no-churn \
                  --output /path/to/wiki /path/to/repo

# Update skill files only (no scan, no API key needed)
go run ./wikigen/ --update-skills /path/to/repo
```

If `--output` is omitted, the wiki is written in-place inside the repo.

## Flags

| Flag | Description |
|------|-------------|
| `--output <dir>` | Wiki output directory (default: in-place in repo) |
| `--exclude <name>` | Exclude a directory by base name (repeatable) |
| `--no-default-excludes` | Clear default exclusions (only `.git` is still skipped) |
| `--base-url <url>` | URL prefix for source file links |
| `--full` | Force full regeneration, ignore manifest |
| `--json` | Emit line-delimited JSON progress events on stderr |
| `--dry-run` | Show what would be summarized without calling the API |
| `--clean` | Remove all generated files and manifest |
| `--update-skills` | Update CLAUDE.md and AGENTS.md without scanning |
| `--no-deps-graph` | Skip dependency graph generation (`deps-graph.md`) |
| `--no-file-index` | Skip file index generation (`file-index.md`) |
| `--no-recipes` | Skip modification recipes (`recipes.md`, saves 1 LLM call) |
| `--no-traces` | Skip entry point traces (`traces.md`, saves 1 LLM call) |
| `--no-boundaries` | Omit `## Boundary` section from directory summaries |
| `--no-test-hints` | Omit `## Testing` section from directory summaries |
| `--no-symbol-index` | Skip symbol index generation (`symbol-index.md`) |
| `--no-churn` | Skip churn overlay generation (`churn.md`) |

## Generated artifacts

All on by default. Each can be disabled with its `--no-*` flag.

### Per-directory: `SUMMARY.md` / `SUMMARY.html`

Each directory gets a summary containing:

| Section | What it provides |
|---------|-----------------|
| **Overview** | What the directory is responsible for (1-3 sentences) |
| **Key files** | Every file with a one-line description and relative link |
| **How it works** | Internal data flow, call chains, entry points |
| **Key types and interfaces** | Important exports, who implements/calls them |
| **Dependencies** | What it imports, what imports it |
| **Boundary** | Public API / internal / entry point classification |
| **Testing** | Where tests live, testing pattern, helpers |
| **Configuration** | Env vars, config files, CLI flags |
| **When to look here** | Task descriptions that lead to this directory |
| **Child directories** | What each child owns |

### Root-level artifacts

| Artifact | Description |
|----------|-------------|
| `wiki.md` / `index.html` | Root overview with architecture, navigation guide, and links to all artifacts |
| `deps-graph.md` | Directory dependency adjacency list with boundary annotations |
| `file-index.md` | Flat file lookup table (name, directory, language, description) |
| `symbol-index.md` | Key types, functions, interfaces and their locations |
| `recipes.md` | Common modification patterns derived from git co-change history |
| `traces.md` | 2-3 critical end-to-end execution paths from entry points |
| `churn.md` | Recent activity by directory (90 days), classified high/medium/low |

## HTML output

Every markdown file gets a styled HTML companion:

- Light/dark mode (follows system preference)
- Breadcrumb navigation (upward through the tree)
- Child directory navigation (downward links to children)
- All `.md` links rewritten to `.html` for seamless browser navigation
- Responsive layout, monospace code blocks

```bash
open /path/to/wiki/index.html
```

## LLM skill files

wikigen writes `CLAUDE.md` and `AGENTS.md` into the target repo root. These teach LLM agents a three-layer navigation approach:

**Layer 1 — Project-level**: Read `wiki.md` for architecture and navigation guide. Consult root artifacts for specific questions:

| Question | Read this |
|----------|-----------|
| What files do I change for this task? | `recipes.md` |
| Where is this file or type? | `file-index.md` / `symbol-index.md` |
| What depends on this package? | `deps-graph.md` |
| How does data flow end-to-end? | `traces.md` |
| Is this area actively changing? | `churn.md` |

**Layer 2 — Directory-level**: Read `SUMMARY.md` for relevant directories. Check boundary (public API?), testing patterns, and dependencies.

**Layer 3 — Source code**: Read the specific files the wiki pointed to.

**Safe for existing files.** If `CLAUDE.md` or `AGENTS.md` already exists, wikigen appends a delimited section between `<!-- wikigen:start -->` and `<!-- wikigen:end -->` markers. `--clean` removes only the wikigen section.

## LLM cost

| Feature | LLM calls |
|---------|-----------|
| Directory summaries | 1 per dirty directory |
| Root overview | 1 |
| Modification recipes | 1 (opt-out: `--no-recipes`) |
| Entry point traces | 1 (opt-out: `--no-traces`) |
| All other features | 0 (local aggregation or git analysis) |

For a repo with 50 directories: ~52 LLM calls on full scan. Incremental runs only regenerate changed directories.

## Incremental mode

wikigen saves `.wikigen-manifest.json` with SHA-256 hashes of every source file and summary. On subsequent runs:

- Uses `git diff` to detect changes (fast), or hash comparison (portable)
- Only dirty directories and their ancestors are re-summarized
- Root artifacts regenerate when any summary changes

## Default exclusions

Excluded by default (plus all dot-prefixed directories): `.claude`, `.codex`, `docs`

```bash
go run ./wikigen/ --no-default-excludes --exclude vendor --exclude node_modules --output ./wiki /repo
```

## Supported languages

Go, Swift, TypeScript, JavaScript, C, Python, Shell, Make, Docker, YAML, TOML, JSON.

Binary files, generated files, lock files, and files over 100KB are automatically skipped.

## CI/CD integration

```bash
go run ./wikigen/ --json --output ./wiki /repo 2>progress.jsonl
```

Each line is a JSON event. Exit codes: `0` success, `1` error, `2` partial failure.

## Output structure

```
wiki-output/
  wiki.md / index.html           Root overview
  deps-graph.md / .html          Dependency graph
  file-index.md / .html          File lookup table
  symbol-index.md / .html        Symbol index
  recipes.md / .html             Modification recipes
  traces.md / .html              Entry point traces
  churn.md / .html               Recent activity
  .wikigen-manifest.json         Incremental state
  cmd/
    SUMMARY.md / .html           Per-directory summary
  internal/
    SUMMARY.md / .html
    server/
      SUMMARY.md / .html
    db/
      SUMMARY.md / .html

target-repo/
  CLAUDE.md                      Skill file for Claude Code
  AGENTS.md                      Skill file for Codex
```
