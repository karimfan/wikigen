# Codebase Navigation via Wiki

This project has an auto-generated codebase wiki at `{{WIKI_PATH}}`. **Always consult the wiki before reading source files.** It will save you time and tokens.

## Wiki location

The wiki root is at `{{WIKI_PATH}}`. All SUMMARY.md files for subdirectories are relative to the directory containing wiki.md.

## How to use the wiki

1. **Start at the root.** Read `{{WIKI_PATH}}` to get:
   - A project overview explaining what this codebase does
   - An architecture snapshot showing how major pieces connect
   - A **navigation guide** mapping task categories to directories
   - An annotated contents listing with one-line descriptions per directory

2. **Match your task to a directory.** Use the navigation guide and "When to look here" sections to identify which directories are relevant. Do NOT grep the entire codebase or read files at random.

3. **Drill into SUMMARY.md files.** Each directory has a `SUMMARY.md` with:
   - What the directory is responsible for
   - Key files with descriptions
   - Important types, functions, and interfaces
   - A "When to look here" section for task-based routing
   - Child directory descriptions with task routing

4. **Read source files last.** Only after the wiki has pointed you to the right files should you open them. The wiki tells you what each file does so you can skip irrelevant ones.

## Navigation pattern

```
{{WIKI_PATH}}                        → Find the right top-level directory
  └─ {dir}/SUMMARY.md               → Find the right subdirectory or file
      └─ {dir}/{subdir}/SUMMARY.md  → Narrow further if needed
          └─ {dir}/{subdir}/file.go → Read the actual source
```

## Rules

- **Never skip the wiki.** Even if you think you know where something is, confirm via the wiki. The codebase may have been reorganized.
- **Read top-down.** Start from `{{WIKI_PATH}}`, not from a random SUMMARY.md. The root page has context that child pages assume you have.
- **Trust the wiki for orientation, trust the code for details.** The wiki tells you where to look and what to expect. The source code is the ground truth for implementation details.
- **If the wiki is missing or stale**, fall back to normal exploration but note it — the wiki may need regeneration via `wikigen`.
