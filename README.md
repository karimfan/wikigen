# wikigen

A Go CLI that generates hierarchical markdown and HTML documentation for any codebase, optimized for LLM navigation. It traverses a repository bottom-up, sends source files to the Claude API for summarization, and produces a navigable wiki with per-directory summaries.

## How it works

1. Walks the directory tree in post-order (deepest directories first)
2. At each directory, reads source files and detects their language
3. Sends file contents + child directory summaries to Claude Haiku for summarization
4. Writes a `SUMMARY.md` and `SUMMARY.html` in the wiki output folder for each directory
5. Generates a root-level project overview via a separate LLM call
6. Generates `wiki.md` and `index.html` linking everything together
7. Writes `CLAUDE.md` and `AGENTS.md` skill files into the repo root to teach LLMs how to use the wiki
8. Saves a `.wikigen-manifest.json` so subsequent runs only regenerate changed directories

## Requirements

- Go 1.21+
- `ANTHROPIC_API_KEY` environment variable set

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

## HTML output

Alongside every markdown file, wikigen generates a styled HTML page:

- `index.html` — browsable version of `wiki.md`
- `{dir}/SUMMARY.html` — browsable version of each `SUMMARY.md`

Features:
- Light/dark mode (follows system preference)
- Breadcrumb navigation
- All `.md` links rewritten to `.html` for seamless browser navigation
- Responsive layout, monospace code blocks

Open the wiki in a browser:

```bash
open /path/to/wiki/index.html
```

## Navigation-oriented summaries

Each summary is designed for LLM task routing, not just documentation:

- **Project overview** (root wiki.md): Architecture snapshot and a navigation guide mapping task categories to directories
- **Directory summaries**: Include a "When to look here" section listing the kinds of tasks that would lead an LLM to that directory
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
  .wikigen-manifest.json           # Incremental state
  cmd/
    SUMMARY.md                     # Summary of cmd/
    SUMMARY.html                   # Browsable HTML version
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
