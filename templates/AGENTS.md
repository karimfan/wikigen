# Codebase Navigation via Wiki

This project has an auto-generated codebase wiki. **Read the wiki before reading source files.** It tells you where to look so you don't waste time searching.

## Wiki location

- **Root**: `{{WIKI_PATH}}`
- **Directory summaries**: `{{WIKI_DIR}}/{dir}/SUMMARY.md` (one per directory in the codebase)
- **Root artifacts** (all in `{{WIKI_DIR}}/`):
  - `deps-graph.md` — directory-to-directory dependency graph
  - `file-index.md` — flat file lookup (name → directory → description)
  - `symbol-index.md` — key types, functions, interfaces and their locations
  - `recipes.md` — common modification patterns (which files change together)
  - `traces.md` — end-to-end execution flows from entry points
  - `churn.md` — recently active directories (last 90 days)
- All paths inside the wiki are relative to `{{WIKI_DIR}}/`

## How to navigate

### Step 1: Read the root

Read `{{WIKI_PATH}}`. It contains:
- A **project overview** (what this codebase does)
- An **architecture snapshot** (how major pieces connect)
- A **navigation guide** that maps task categories to directories
- An **annotated contents** listing with one-line descriptions
- Links to root artifacts (dependency graph, file index, symbol index, etc.)

### Step 2: Match your task to directories

Use the navigation guide and "When to look here" sections to find relevant directories. Pick the 1-3 most relevant, not all of them.

### Step 3: Read the SUMMARY.md for those directories

Each `SUMMARY.md` contains:
- What the directory is responsible for
- Key files with one-line descriptions
- Important types, functions, and interfaces
- Dependencies (what it imports, what imports it)
- Boundary classification (public API, internal, or entry point)
- Testing patterns (where tests live, what style they use)
- A "When to look here" section
- Child directory descriptions

If a child directory looks relevant, read its SUMMARY.md too. Keep drilling until you've identified the specific files you need.

### Step 4: Read source files

Only now open the actual source files the wiki pointed you to.

## Worked example

Task: "Add a new CLI subcommand that lists active policies"

```
1. Read {{WIKI_PATH}}
   → Navigation guide says: "To modify CLI commands → cmd/"
   → Also says: "Policy management → internal/policy/"

2. Read {{WIKI_DIR}}/cmd/SUMMARY.md
   → Shows cmd/leash/main.go handles command routing
   → Shows cmd/leash/commands.go defines subcommands

3. Read {{WIKI_DIR}}/internal/policy/SUMMARY.md
   → Shows policy.Manager type with ListActive() method
   → Shows policy/store.go handles persistence

4. Now read the actual files:
   → cmd/leash/commands.go (to add the new subcommand)
   → internal/policy/store.go (to understand the ListActive API)
```

Total files read: 4 (wiki) + 2 (source) = 6, instead of searching the entire codebase.

## When to use root artifacts

Use the root artifacts for specific tasks before diving into source:

- **"What files do I change for X?"** → Read `{{WIKI_DIR}}/recipes.md` for common modification patterns
- **"Where is this file/type?"** → Read `{{WIKI_DIR}}/file-index.md` or `{{WIKI_DIR}}/symbol-index.md` for instant lookup
- **"What depends on this package?"** → Read `{{WIKI_DIR}}/deps-graph.md` to assess blast radius
- **"How does a request flow through the system?"** → Read `{{WIKI_DIR}}/traces.md` for end-to-end paths
- **"Is this area actively changing?"** → Read `{{WIKI_DIR}}/churn.md` to check recent activity
- **"Is this a public API or internal?"** → Check the `## Boundary` section in the relevant SUMMARY.md
- **"How should I write tests here?"** → Check the `## Testing` section in the relevant SUMMARY.md

## Multi-directory tasks

If your task spans multiple directories (e.g., adding an API endpoint that touches `cmd/`, `internal/`, and `controlui/`):

1. Read the root wiki.md once
2. Read each relevant directory's SUMMARY.md
3. Build a mental map of which files need changes before opening any source
4. Read source files in dependency order (data layer first, then API, then UI)

## What NOT to do

- **Don't grep the entire repo** for a keyword. The wiki's "When to look here" sections are faster and more accurate.
- **Don't read files speculatively.** If the wiki doesn't mention a file as relevant, you probably don't need it.
- **Don't skip the root wiki.md** and jump straight to a SUMMARY.md. The root has architecture context that child pages assume you know.
- **Don't assume file paths from memory.** Files get moved. The wiki reflects the current structure.

## When the wiki isn't enough

The wiki covers structure and purpose. For these, use other tools:
- **Recent changes**: `git log`, `git blame`
- **Runtime behavior**: test output, logs
- **Exact implementation details**: read the source (after the wiki points you to it)
- **If the wiki is missing or stale**: fall back to normal exploration. The wiki may need regeneration via `wikigen`.
