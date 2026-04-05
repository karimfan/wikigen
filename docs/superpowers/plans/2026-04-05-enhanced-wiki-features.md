# Enhanced Wiki Features Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 9 features to wikigen that deepen LLM comprehension of codebases: HTML tree nav fix, dependency graph, file index, modification recipes, entry point traces, boundary annotations, test hints, symbol index, and churn overlay.

**Architecture:** All changes are in `wikigen.go` (single-file convention). The existing post-order traversal pipeline is extended with 2 new prompt sections (boundary, testing) and a new post-processing phase that parses summaries, runs git analysis, makes 2 LLM calls, and emits 6 new root artifact files. wiki.md links to all artifacts. HTML output gains child directory navigation.

**Tech Stack:** Go 1.25, stdlib only + goldmark (existing dependency). Claude Haiku API for 2 new LLM calls.

---

### Task 1: Add Config Fields and Opt-Out Flags

**Files:**
- Modify: `wikigen.go:83-94` (config struct)
- Modify: `wikigen.go:386-452` (parseArgs)
- Modify: `wikigen.go:454-469` (printUsage)

- [ ] **Step 1: Add config fields**

Add 8 boolean fields to the config struct at line 94 (before the closing brace):

```go
type config struct {
	repoRoot          string
	wikiRoot          string
	excludes          map[string]bool
	baseURL           string
	dryRun            bool
	clean             bool
	full              bool
	jsonProgress      bool
	apiKey            string
	noDefaultExcludes bool
	noDepsGraph       bool
	noFileIndex       bool
	noRecipes         bool
	noTraces          bool
	noBoundaries      bool
	noTestHints       bool
	noSymbolIndex     bool
	noChurn           bool
}
```

- [ ] **Step 2: Add flag parsing**

In `parseArgs()`, add cases before the `case strings.HasPrefix(a, "-"):` fallthrough (around line 418):

```go
case a == "--no-deps-graph":
	cfg.noDepsGraph = true
case a == "--no-file-index":
	cfg.noFileIndex = true
case a == "--no-recipes":
	cfg.noRecipes = true
case a == "--no-traces":
	cfg.noTraces = true
case a == "--no-boundaries":
	cfg.noBoundaries = true
case a == "--no-test-hints":
	cfg.noTestHints = true
case a == "--no-symbol-index":
	cfg.noSymbolIndex = true
case a == "--no-churn":
	cfg.noChurn = true
```

- [ ] **Step 3: Update printUsage**

Add to the flags section in `printUsage()`:

```go
fmt.Fprintln(os.Stderr, `Usage: wikigen [flags] <repo>

Generates hierarchical markdown documentation for a codebase.

Flags:
  --output <dir>         Wiki output directory (default: in-place in repo)
  --exclude <pattern>    Exclude directory by base name (repeatable)
  --no-default-excludes  Clear default exclusions (only .git remains)
  --base-url <url>       URL prefix for source file links
  --full                 Force full regeneration (ignore manifest)
  --json                 Emit line-delimited JSON progress events on stderr
  --dry-run              Show what would be summarized without calling the API
  --clean                Remove all generated files and manifest
  --no-deps-graph        Disable dependency graph generation
  --no-file-index        Disable file index generation
  --no-recipes           Disable modification recipes (saves 1 LLM call)
  --no-traces            Disable entry point traces (saves 1 LLM call)
  --no-boundaries        Disable boundary annotations in summaries
  --no-test-hints        Disable test hints in summaries
  --no-symbol-index      Disable symbol index generation
  --no-churn             Disable churn overlay generation
  --help                 Show this help message`)
```

- [ ] **Step 4: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add wikigen.go
git commit -m "feat: add opt-out flags for Sprint 005 enhanced wiki features"
```

---

### Task 2: Modify LLM Prompt for Boundary and Testing Sections

**Files:**
- Modify: `wikigen.go:897-944` (summarize function, system prompt)

- [ ] **Step 1: Refactor system prompt to string builder**

Replace the hardcoded system prompt string in `summarize()` with a builder that conditionally includes sections. The `summarize` function needs access to config, so change its signature:

```go
func summarize(cfg config, b dirBundle) (string, error) {
	prompt := buildPrompt(b)

	var sysPrompt strings.Builder
	sysPrompt.WriteString(`You are writing documentation for an LLM that will use this wiki to understand and modify a codebase. The LLM reads the wiki top-down, so each summary must help it both navigate (which files to open) and comprehend (how things work and connect) without reading every source file.

Write a markdown summary following this structure:

## Overview
1-3 sentences: What this directory is responsible for. State its role in the larger system.

## Key files
Bullet list with relative links and one-line descriptions:
- [filename](./filename) — description

## How it works
Explain the internal data flow and control flow. How do the files in this directory interact? What are the key call chains? If there is a primary entry point, start there and trace the flow. For example: "Requests arrive at handler.go, which validates input using validator.go, then calls store.go to persist. Errors are wrapped by errors.go and returned as HTTP responses."

## Key types and interfaces
List the most important exported types, functions, and interfaces. For interfaces, note who implements them. For key functions, note who calls them and what they return. Group related items rather than listing everything flat.

## Dependencies
Which other packages/directories does this code import from or get called by? List the most important ones with a brief note on the relationship:
- Imports: "policy/ for PolicySet, cedar/ for Compile()"
- Imported by: "runner/ calls LSMManager.Load(), leashd/ calls UpdateRuntimeRules()"

If you can determine this from the code, include it. If you cannot determine the full picture, list what you can see from imports and function signatures.`)

	if !cfg.noBoundaries {
		sysPrompt.WriteString(`

## Boundary
State whether this directory is: **public** (exported API, meant to be imported by other packages), **internal** (implementation detail, not meant for direct external use), or **entry point** (binary/command, not imported). One sentence on what the contract is — what callers can rely on.`)
	}

	if !cfg.noTestHints {
		sysPrompt.WriteString(`

## Testing
Where tests live (colocated _test.go files, separate tests/ directory, or __tests__/). Note the testing pattern: table-driven, fixtures, mocks, integration vs unit. If there are test helpers or shared fixtures, name them. Skip this section if there are no tests in this directory.`)
	}

	sysPrompt.WriteString(`

## Configuration
Note any environment variables, config files, CLI flags, or constants that control behavior in this directory. Skip this section if there are none.

## When to look here
Bullet list of task descriptions that would lead an LLM to this directory. Be specific to what the code actually does. For example: "Modifying how Cedar policies are parsed", "Adding a new CLI subcommand", "Changing container network rules".

## Child directories
(If any) For each child, a bullet with a description of what it owns and what tasks would require looking there.

Guidelines:
- Be concise but not shallow. Prioritize understanding over brevity.
- Do not invent functionality not present in the code.
- Use relative file links for files in this directory.
- Omit any section that has no meaningful content (e.g., skip Configuration if there are no config knobs).`)

	reqBody := apiRequest{
		Model:     llmModel,
		MaxTokens: 2048,
		System:    sysPrompt.String(),
		Messages: []apiMessage{
			{Role: "user", Content: prompt},
		},
	}
```

- [ ] **Step 2: Update the call site**

In `main()` at line ~274, change:
```go
summary, err := summarize(cfg.apiKey, bundle)
```
to:
```go
summary, err := summarize(cfg, bundle)
```

- [ ] **Step 3: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 4: Commit**

```bash
git add wikigen.go
git commit -m "feat: add boundary and testing sections to directory summarization prompt"
```

---

### Task 3: Fix HTML Child Directory Navigation

**Files:**
- Modify: `wikigen.go:1084-1122` (HTML template)
- Modify: `wikigen.go:1124-1128` (htmlPage struct)
- Modify: `wikigen.go:1149-1172` (buildBreadcrumb — add buildChildNav nearby)
- Modify: `wikigen.go:1174-1190` (writeHTMLPage — new parameter)
- Modify: `wikigen.go:231-310` (main loop — pass child dirs)
- Modify: `wikigen.go:344-348` (root index.html — pass empty)

- [ ] **Step 1: Add ChildNav to htmlPage struct and template**

Update the struct:
```go
type htmlPage struct {
	Title      string
	Breadcrumb template.HTML
	ChildNav   template.HTML
	Body       template.HTML
}
```

Update the HTML template — insert child nav and CSS. In the `<style>` block, add after the `.generated` rule:
```css
nav.children { font-size: 0.85rem; color: var(--muted); margin-top: 2rem; padding: 0.5rem 0.75rem; background: var(--nav-bg); border-radius: 6px; }
nav.children a { color: var(--link); text-decoration: none; margin: 0 0.25rem; }
nav.children a:hover { text-decoration: underline; }
```

In the template body, insert `{{if .ChildNav}}{{.ChildNav}}{{end}}` between `{{.Body}}` and the `.generated` div:
```html
{{if .Breadcrumb}}<nav class="breadcrumb">{{.Breadcrumb}}</nav>{{end}}
{{.Body}}
{{if .ChildNav}}{{.ChildNav}}{{end}}
<div class="generated">Generated by wikigen</div>
```

- [ ] **Step 2: Create buildChildNav function**

Add after `buildBreadcrumb`:
```go
// buildChildNav generates an HTML nav block linking to child directory summaries.
func buildChildNav(childDirs []string) template.HTML {
	if len(childDirs) == 0 {
		return ""
	}
	var links []string
	for _, child := range childDirs {
		links = append(links, fmt.Sprintf(`<a href="%s/SUMMARY.html">%s/</a>`, child, child))
	}
	return template.HTML(fmt.Sprintf(`<nav class="children"><strong>Child directories:</strong> %s</nav>`, strings.Join(links, " &middot; ")))
}
```

- [ ] **Step 3: Update writeHTMLPage signature**

Change the function signature and add ChildNav:
```go
func writeHTMLPage(outPath, title, mdContent, rel string, childDirs []string) error {
	body := markdownToHTML(mdContent)
	body = rewriteHTMLLinks(body)

	page := htmlPage{
		Title:      title + " — Codebase Wiki",
		Breadcrumb: buildBreadcrumb(rel),
		ChildNav:   buildChildNav(childDirs),
		Body:       template.HTML(body),
	}

	var buf bytes.Buffer
	if err := htmlTemplate.Execute(&buf, page); err != nil {
		return fmt.Errorf("render HTML template: %w", err)
	}
	return os.WriteFile(outPath, buf.Bytes(), 0644)
}
```

- [ ] **Step 4: Build parentChildren map in main()**

Before the main processing loop (around line 228, after the `newManifest` initialization), build the map:
```go
// Build parent→children map for HTML navigation.
parentChildren := make(map[string][]string)
for _, rel := range dirs {
	parent := filepath.Dir(rel)
	if parent == "." {
		parent = ""
	}
	parentChildren[parent] = append(parentChildren[parent], filepath.Base(rel))
}
// Sort children alphabetically.
for k := range parentChildren {
	sort.Strings(parentChildren[k])
}
```

- [ ] **Step 5: Update all writeHTMLPage call sites**

In the main loop (skip path, ~line 248):
```go
if existing != "" && !cfg.dryRun {
	htmlPath := filepath.Join(cfg.wikiRoot, rel, "SUMMARY.html")
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		writeHTMLPage(htmlPath, filepath.Base(rel), existing, rel, parentChildren[rel])
	}
}
```

In the main loop (dirty path, ~line 299):
```go
htmlPath := filepath.Join(outDir, "SUMMARY.html")
if err := writeHTMLPage(htmlPath, filepath.Base(rel), md, rel, parentChildren[rel]); err != nil {
	fmt.Fprintf(os.Stderr, "  error writing %s: %v\n", htmlPath, err)
}
```

For root index.html (~line 346), pass top-level dirs:
```go
indexPath := filepath.Join(cfg.wikiRoot, "index.html")
if err := writeHTMLPage(indexPath, "Codebase Wiki", wiki, "", parentChildren[""]); err != nil {
	fmt.Fprintf(os.Stderr, "error writing index.html: %v\n", err)
}
```

- [ ] **Step 6: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 7: Commit**

```bash
git add wikigen.go
git commit -m "fix: add child directory navigation links to HTML output"
```

---

### Task 4: Summary Parsing Engine

**Files:**
- Modify: `wikigen.go` — add new functions after the HTML generation section (~line 1190)

- [ ] **Step 1: Add data types for parsed results**

Add after the existing types section:
```go
// Types for parsed summary data.
type depEdge struct {
	Dir  string
	Note string
}

type fileIndexEntry struct {
	Name string
	Dir  string
	Desc string
	Lang string
}

type symbolEntry struct {
	Name     string
	Kind     string
	Location string
	Dir      string
}
```

- [ ] **Step 2: Create extractSection helper**

```go
// extractSection returns the content of a markdown section by heading name.
// It finds "## heading" and returns everything until the next "## " or EOF.
func extractSection(summary, heading string) string {
	marker := "## " + heading
	idx := strings.Index(summary, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	// Skip the rest of the heading line.
	if nl := strings.Index(summary[start:], "\n"); nl >= 0 {
		start += nl + 1
	} else {
		return ""
	}
	// Find next ## heading or end.
	rest := summary[start:]
	end := strings.Index(rest, "\n## ")
	if end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	return strings.TrimSpace(rest)
}
```

- [ ] **Step 3: Create parseDependencies**

```go
// parseDependencies extracts import and imported-by relationships from a summary.
func parseDependencies(summary, dir string) (imports []depEdge, importedBy []depEdge) {
	section := extractSection(summary, "Dependencies")
	if section == "" {
		return
	}
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "imports:") || strings.HasPrefix(lower, "imports ") {
			note := strings.TrimSpace(line[len("imports"):])
			note = strings.TrimPrefix(note, ":")
			note = strings.TrimSpace(note)
			imports = append(imports, depEdge{Dir: dir, Note: note})
		} else if strings.HasPrefix(lower, "imported by:") || strings.HasPrefix(lower, "imported by ") {
			note := strings.TrimSpace(line[len("imported by"):])
			note = strings.TrimPrefix(note, ":")
			note = strings.TrimSpace(note)
			importedBy = append(importedBy, depEdge{Dir: dir, Note: note})
		}
	}
	return
}
```

- [ ] **Step 4: Create parseKeyFiles**

```go
// parseKeyFiles extracts file entries from the Key files section.
var keyFilePattern = regexp.MustCompile(`\[([^\]]+)\]\(\./([^)]+)\)\s*[-—]\s*(.+)`)

func parseKeyFiles(summary, dir string) []fileIndexEntry {
	section := extractSection(summary, "Key files")
	if section == "" {
		return nil
	}
	var results []fileIndexEntry
	for _, line := range strings.Split(section, "\n") {
		m := keyFilePattern.FindStringSubmatch(line)
		if m != nil {
			results = append(results, fileIndexEntry{
				Name: m[1],
				Dir:  dir,
				Desc: strings.TrimSpace(m[3]),
			})
		}
	}
	return results
}
```

- [ ] **Step 5: Create parseSymbols**

```go
// parseSymbols extracts symbol entries from the Key types and interfaces section.
func parseSymbols(summary, dir string) []symbolEntry {
	section := extractSection(summary, "Key types and interfaces")
	if section == "" {
		return nil
	}
	var results []symbolEntry
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || (!strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*")) {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		// Try to extract backtick-wrapped names like `TypeName`
		entry := symbolEntry{Dir: dir}
		if idx := strings.Index(line, "`"); idx >= 0 {
			end := strings.Index(line[idx+1:], "`")
			if end >= 0 {
				name := line[idx+1 : idx+1+end]
				entry.Name = name
				// Infer kind from common patterns.
				lower := strings.ToLower(line)
				switch {
				case strings.Contains(lower, "interface"):
					entry.Kind = "interface"
				case strings.Contains(lower, "struct") || strings.Contains(lower, "type"):
					entry.Kind = "type"
				case strings.Contains(name, "("):
					entry.Kind = "func"
				default:
					entry.Kind = "type"
				}
				// Try to find file reference.
				if fIdx := strings.Index(line, "in "); fIdx >= 0 {
					rest := line[fIdx+3:]
					if bIdx := strings.Index(rest, "`"); bIdx >= 0 {
						bEnd := strings.Index(rest[bIdx+1:], "`")
						if bEnd >= 0 {
							entry.Location = dir + "/" + rest[bIdx+1:bIdx+1+bEnd]
						}
					}
				}
				if entry.Location == "" {
					entry.Location = dir + "/"
				}
				results = append(results, entry)
			}
		}
	}
	return results
}
```

- [ ] **Step 6: Create parseBoundary**

```go
// parseBoundary extracts the boundary classification from a summary.
func parseBoundary(summary string) string {
	section := extractSection(summary, "Boundary")
	if section == "" {
		return ""
	}
	lower := strings.ToLower(section)
	switch {
	case strings.Contains(lower, "entry point"):
		return "entry point"
	case strings.Contains(lower, "internal"):
		return "internal"
	case strings.Contains(lower, "public"):
		return "public"
	default:
		return ""
	}
}
```

- [ ] **Step 7: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 8: Commit**

```bash
git add wikigen.go
git commit -m "feat: add summary parsing engine for deps, files, symbols, boundaries"
```

---

### Task 5: Git Analysis Functions

**Files:**
- Modify: `wikigen.go` — add new functions

- [ ] **Step 1: Add co-change types and function**

```go
// Types for git analysis.
type coChangeCluster struct {
	Files []string
	Count int // minimum co-occurrence count across cluster
}

type churnEntry struct {
	Dir    string
	Count  int
	Level  string // "high", "medium", "low"
}

// analyzeCoChanges finds files that frequently change together in git history.
func analyzeCoChanges(repoRoot string) ([]coChangeCluster, error) {
	// Check if git repo.
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return nil, nil
	}

	// Get recent commit hashes (cap at 500).
	cmd := exec.Command("git", "-C", repoRoot, "log", "--pretty=format:%H", "--since=1y", "-500")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}
	hashes := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(hashes) < 50 {
		return nil, nil // not enough history
	}

	// Build co-occurrence matrix.
	type filePair struct{ a, b string }
	pairCount := make(map[filePair]int)
	fileCommits := make(map[string]int)

	for _, hash := range hashes {
		cmd := exec.Command("git", "-C", repoRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", hash)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		files := strings.Split(strings.TrimSpace(string(out)), "\n")
		// Filter to eligible files.
		var eligible []string
		for _, f := range files {
			if f == "" {
				continue
			}
			base := filepath.Base(f)
			if detectLanguage(base) != "" {
				eligible = append(eligible, f)
			}
		}
		for _, f := range eligible {
			fileCommits[f]++
		}
		// Count pairs (sorted to avoid duplicates).
		for i := 0; i < len(eligible); i++ {
			for j := i + 1; j < len(eligible); j++ {
				a, b := eligible[i], eligible[j]
				if a > b {
					a, b = b, a
				}
				pairCount[filePair{a, b}]++
			}
		}
	}

	// Filter pairs with >= 3 co-occurrences.
	type scoredPair struct {
		a, b  string
		count int
	}
	var pairs []scoredPair
	for p, c := range pairCount {
		if c >= 3 {
			pairs = append(pairs, scoredPair{p.a, p.b, c})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	// Greedy clustering.
	used := make(map[string]bool)
	var clusters []coChangeCluster
	for _, p := range pairs {
		if used[p.a] || used[p.b] {
			continue
		}
		cluster := coChangeCluster{
			Files: []string{p.a, p.b},
			Count: p.count,
		}
		used[p.a] = true
		used[p.b] = true

		// Try to expand: find other files that co-occur with all cluster members.
		for _, p2 := range pairs {
			if len(cluster.Files) >= 8 {
				break
			}
			candidate := ""
			if !used[p2.a] && used[p2.b] {
				candidate = p2.a
			} else if used[p2.a] && !used[p2.b] {
				candidate = p2.b
			}
			if candidate == "" {
				continue
			}
			// Check co-occurrence with all existing cluster members.
			fits := true
			for _, existing := range cluster.Files {
				a, b := candidate, existing
				if a > b {
					a, b = b, a
				}
				if pairCount[filePair{a, b}] < 3 {
					fits = false
					break
				}
			}
			if fits {
				cluster.Files = append(cluster.Files, candidate)
				used[candidate] = true
			}
		}
		clusters = append(clusters, cluster)
		if len(clusters) >= 15 {
			break
		}
	}

	return clusters, nil
}
```

- [ ] **Step 2: Add churn analysis function**

```go
// analyzeChurn computes recent commit frequency per directory.
func analyzeChurn(repoRoot string) ([]churnEntry, error) {
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return nil, nil
	}

	cmd := exec.Command("git", "-C", repoRoot, "log", "--since=90d", "--name-only", "--pretty=format:")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	// Count files per directory.
	dirCount := make(map[string]int)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dir := filepath.Dir(line)
		if dir == "." {
			continue
		}
		// Normalize to top-level or second-level directory.
		parts := strings.SplitN(dir, string(os.PathSeparator), 3)
		if len(parts) >= 2 {
			dir = filepath.Join(parts[0], parts[1])
		} else {
			dir = parts[0]
		}
		dirCount[dir]++
	}

	if len(dirCount) == 0 {
		return nil, nil
	}

	// Build entries.
	var entries []churnEntry
	for dir, count := range dirCount {
		entries = append(entries, churnEntry{Dir: dir, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})

	// Classify: top 20% high, bottom 40% low, rest medium.
	for i := range entries {
		pct := float64(i) / float64(len(entries))
		switch {
		case pct < 0.2:
			entries[i].Level = "high"
		case pct >= 0.6:
			entries[i].Level = "low"
		default:
			entries[i].Level = "medium"
		}
	}

	return entries, nil
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 4: Commit**

```bash
git add wikigen.go
git commit -m "feat: add git co-change and churn analysis functions"
```

---

### Task 6: Recipe and Trace LLM Calls

**Files:**
- Modify: `wikigen.go` — add new functions

- [ ] **Step 1: Create generateRecipes function**

```go
// generateRecipes produces modification recipes from git co-change data.
func generateRecipes(cfg config, clusters []coChangeCluster, summaries map[string]string) (string, error) {
	if len(clusters) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("## Co-change clusters from git history\n\n")
	sb.WriteString("These files frequently change together in commits:\n\n")
	for i, c := range clusters {
		sb.WriteString(fmt.Sprintf("### Cluster %d (co-occurred in %d+ commits)\n", i+1, c.Count))
		for _, f := range c.Files {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Directory summaries (for context)\n\n")
	// Include abbreviated summaries — just Overview and Key files for each.
	for dir, summary := range summaries {
		overview := extractSection(summary, "Overview")
		if overview != "" {
			sb.WriteString(fmt.Sprintf("### %s/\n%s\n\n", dir, overview))
		}
	}

	reqBody := apiRequest{
		Model:     llmModel,
		MaxTokens: 2048,
		System: `You are writing modification recipes for a codebase wiki. Given file co-change clusters from git history and directory summaries, produce 10-15 modification recipes.

Each recipe should have:
1. A descriptive name as an H2 heading (e.g., "## Add a new API endpoint")
2. A bullet list of the files involved, with a one-line note per file on what to change there
3. Use relative paths from the repo root

Focus on the most common and useful modification patterns. Name recipes after the task ("Add a new ...", "Change how ...", "Fix ..."), not the files.

Output markdown only. No preamble.`,
		Messages: []apiMessage{
			{Role: "user", Content: sb.String()},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	var texts []string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}
```

- [ ] **Step 2: Create generateTraces function**

```go
// generateTraces produces entry point execution traces.
func generateTraces(cfg config, summaries map[string]string) (string, error) {
	var sb strings.Builder
	sb.WriteString("## Directory summaries\n\n")
	for dir, summary := range summaries {
		sb.WriteString(fmt.Sprintf("### %s/\n", dir))
		overview := extractSection(summary, "Overview")
		howItWorks := extractSection(summary, "How it works")
		keyFiles := extractSection(summary, "Key files")
		if overview != "" {
			sb.WriteString(overview + "\n\n")
		}
		if howItWorks != "" {
			sb.WriteString("**How it works:** " + howItWorks + "\n\n")
		}
		if keyFiles != "" {
			sb.WriteString("**Key files:**\n" + keyFiles + "\n\n")
		}
	}

	reqBody := apiRequest{
		Model:     llmModel,
		MaxTokens: 2048,
		System: `You are writing entry point traces for a codebase wiki. Given directory summaries, identify the 2-3 most important entry points (main() functions, HTTP server bootstrap, CLI command dispatch, event listeners) and trace their execution end-to-end through the codebase.

For each trace:
1. Use an H2 heading with the trace name (e.g., "## HTTP request → response")
2. Walk through the flow step by step, naming specific files and functions at each step
3. Use → arrows to show the flow direction
4. Keep it concrete — name actual files, not abstract concepts

Output markdown only. No preamble.`,
		Messages: []apiMessage{
			{Role: "user", Content: sb.String()},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	var texts []string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}
```

- [ ] **Step 3: Extract shared API call helper**

The `generateRecipes` and `generateTraces` functions duplicate the API call logic from `summarize()`. Extract a shared helper:

```go
// callLLM sends a request to the Claude API and returns the text response.
func callLLM(apiKey, systemPrompt, userPrompt string) (string, error) {
	reqBody := apiRequest{
		Model:     llmModel,
		MaxTokens: 2048,
		System:    systemPrompt,
		Messages: []apiMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", apiVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	var texts []string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
}
```

Then refactor `summarize()`, `summarizeRoot()`, `generateRecipes()`, and `generateTraces()` to use `callLLM()` instead of duplicating the HTTP/JSON logic. For example, `generateRecipes` becomes:

```go
func generateRecipes(cfg config, clusters []coChangeCluster, summaries map[string]string) (string, error) {
	if len(clusters) == 0 {
		return "", nil
	}
	// ... build userPrompt as before ...
	return callLLM(cfg.apiKey, recipesSystemPrompt, sb.String())
}
```

And `generateTraces`:
```go
func generateTraces(cfg config, summaries map[string]string) (string, error) {
	// ... build userPrompt as before ...
	return callLLM(cfg.apiKey, tracesSystemPrompt, sb.String())
}
```

Store the system prompts as package-level `const` strings for readability.

- [ ] **Step 4: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add wikigen.go
git commit -m "feat: add recipe and trace generation with shared callLLM helper"
```

---

### Task 7: Root Artifact Writers

**Files:**
- Modify: `wikigen.go` — add new functions

- [ ] **Step 1: Create writeDepsGraph**

```go
// writeDepsGraph generates the dependency graph root artifact.
func writeDepsGraph(cfg config, summaries map[string]string) error {
	var sb strings.Builder
	sb.WriteString(generatedTag + "\n")
	sb.WriteString("# Dependency Graph\n\n")
	sb.WriteString("Directory-to-directory dependencies extracted from summaries.\n\n")

	// Collect boundaries.
	boundaries := make(map[string]string)
	for dir, summary := range summaries {
		if b := parseBoundary(summary); b != "" {
			boundaries[dir] = b
		}
	}

	// Collect and write edges.
	sb.WriteString("```\n")
	var dirs []string
	for dir := range summaries {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	edgeCount := 0
	for _, dir := range dirs {
		imports, _ := parseDependencies(summaries[dir], dir)
		if len(imports) == 0 {
			continue
		}
		label := dir
		if b, ok := boundaries[dir]; ok {
			label = fmt.Sprintf("%s (%s)", dir, b)
		}
		sb.WriteString(fmt.Sprintf("%s → %s\n", label, imports[0].Note))
		edgeCount++
	}
	sb.WriteString("```\n")

	md := sb.String()
	mdPath := filepath.Join(cfg.wikiRoot, "deps-graph.md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return err
	}
	htmlPath := filepath.Join(cfg.wikiRoot, "deps-graph.html")
	return writeHTMLPage(htmlPath, "Dependency Graph", md, "", nil)
}
```

- [ ] **Step 2: Create writeFileIndex**

```go
// writeFileIndex generates the file index root artifact.
func writeFileIndex(cfg config, dirs []string, summaries map[string]string) error {
	// Collect descriptions from summaries.
	descMap := make(map[string]map[string]string) // dir → filename → description
	for dir, summary := range summaries {
		for _, fe := range parseKeyFiles(summary, dir) {
			if descMap[dir] == nil {
				descMap[dir] = make(map[string]string)
			}
			descMap[dir][fe.Name] = fe.Desc
		}
	}

	// Walk all eligible source files.
	var entries []fileIndexEntry
	for _, rel := range dirs {
		abs := filepath.Join(cfg.repoRoot, rel)
		dirEntries, err := os.ReadDir(abs)
		if err != nil {
			continue
		}
		for _, e := range dirEntries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lang := detectLanguage(name)
			if lang == "" {
				continue
			}
			desc := ""
			if dm, ok := descMap[rel]; ok {
				desc = dm[name]
			}
			entries = append(entries, fileIndexEntry{
				Name: name,
				Dir:  rel,
				Desc: desc,
				Lang: lang,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Dir < entries[j].Dir
	})

	var sb strings.Builder
	sb.WriteString(generatedTag + "\n")
	sb.WriteString("# File Index\n\n")
	sb.WriteString(fmt.Sprintf("%d source files across %d directories.\n\n", len(entries), len(dirs)))
	sb.WriteString("| File | Directory | Language | Description |\n")
	sb.WriteString("|------|-----------|----------|-------------|\n")
	for _, e := range entries {
		desc := e.Desc
		if desc == "" {
			desc = "—"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", e.Name, e.Dir, e.Lang, desc))
	}

	md := sb.String()
	mdPath := filepath.Join(cfg.wikiRoot, "file-index.md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return err
	}
	htmlPath := filepath.Join(cfg.wikiRoot, "file-index.html")
	return writeHTMLPage(htmlPath, "File Index", md, "", nil)
}
```

- [ ] **Step 3: Create writeSymbolIndex**

```go
// writeSymbolIndex generates the symbol index root artifact.
func writeSymbolIndex(cfg config, summaries map[string]string) error {
	var allSymbols []symbolEntry
	for dir, summary := range summaries {
		allSymbols = append(allSymbols, parseSymbols(summary, dir)...)
	}

	sort.Slice(allSymbols, func(i, j int) bool {
		return allSymbols[i].Name < allSymbols[j].Name
	})

	var sb strings.Builder
	sb.WriteString(generatedTag + "\n")
	sb.WriteString("# Symbol Index\n\n")
	sb.WriteString(fmt.Sprintf("%d key types and functions.\n\n", len(allSymbols)))
	sb.WriteString("| Symbol | Kind | Location |\n")
	sb.WriteString("|--------|------|----------|\n")
	for _, s := range allSymbols {
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", s.Name, s.Kind, s.Location))
	}

	md := sb.String()
	mdPath := filepath.Join(cfg.wikiRoot, "symbol-index.md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return err
	}
	htmlPath := filepath.Join(cfg.wikiRoot, "symbol-index.html")
	return writeHTMLPage(htmlPath, "Symbol Index", md, "", nil)
}
```

- [ ] **Step 4: Create writeChurn**

```go
// writeChurn generates the churn overlay root artifact.
func writeChurn(cfg config, entries []churnEntry) error {
	if len(entries) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(generatedTag + "\n")
	sb.WriteString("# Recent Activity\n\n")
	sb.WriteString("Commit frequency by directory over the last 90 days.\n\n")
	sb.WriteString("| Directory | Commits (90d) | Churn |\n")
	sb.WriteString("|-----------|---------------|-------|\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", e.Dir, e.Count, e.Level))
	}

	md := sb.String()
	mdPath := filepath.Join(cfg.wikiRoot, "churn.md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return err
	}
	htmlPath := filepath.Join(cfg.wikiRoot, "churn.html")
	return writeHTMLPage(htmlPath, "Recent Activity", md, "", nil)
}
```

- [ ] **Step 5: Create writeRecipesFile and writeTracesFile**

```go
// writeRecipesFile writes the recipes root artifact.
func writeRecipesFile(cfg config, content string) error {
	if content == "" {
		return nil
	}
	md := fmt.Sprintf("%s\n# Modification Recipes\n\nCommon change patterns derived from git history.\n\n%s\n", generatedTag, content)
	mdPath := filepath.Join(cfg.wikiRoot, "recipes.md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return err
	}
	htmlPath := filepath.Join(cfg.wikiRoot, "recipes.html")
	return writeHTMLPage(htmlPath, "Modification Recipes", md, "", nil)
}

// writeTracesFile writes the traces root artifact.
func writeTracesFile(cfg config, content string) error {
	if content == "" {
		return nil
	}
	md := fmt.Sprintf("%s\n# Entry Point Traces\n\nEnd-to-end execution flows through the codebase.\n\n%s\n", generatedTag, content)
	mdPath := filepath.Join(cfg.wikiRoot, "traces.md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return err
	}
	htmlPath := filepath.Join(cfg.wikiRoot, "traces.html")
	return writeHTMLPage(htmlPath, "Entry Point Traces", md, "", nil)
}
```

- [ ] **Step 6: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 7: Commit**

```bash
git add wikigen.go
git commit -m "feat: add root artifact writers for all 6 new wiki files"
```

---

### Task 8: Modify buildWiki to Link Root Artifacts

**Files:**
- Modify: `wikigen.go:1286-1340` (buildWiki function)

- [ ] **Step 1: Add artifact metadata parameter**

Change `buildWiki` signature to accept counts for the summary lines:

```go
type wikiArtifacts struct {
	depsEdgeCount   int
	fileCount       int
	dirCount        int
	symbolCount     int
	hasChurn        bool
	hasRecipes      bool
	hasTraces       bool
}

func buildWiki(cfg config, dirs []string, summaries map[string]string, rootOverview string, artifacts wikiArtifacts) string {
```

- [ ] **Step 2: Add artifact link sections**

After the `## Contents` section and the `---` separator, before the per-directory details, insert the new sections:

```go
	sb.WriteString("\n---\n\n")

	// Root artifact links.
	if !cfg.noDepsGraph && artifacts.depsEdgeCount > 0 {
		sb.WriteString(fmt.Sprintf("## Dependency graph\n\n%d directories with dependency edges. [Full graph →](./deps-graph.md)\n\n", artifacts.depsEdgeCount))
	}
	if !cfg.noFileIndex && artifacts.fileCount > 0 {
		sb.WriteString(fmt.Sprintf("## File index\n\n%d source files across %d directories. [Full index →](./file-index.md)\n\n", artifacts.fileCount, artifacts.dirCount))
	}
	if !cfg.noSymbolIndex && artifacts.symbolCount > 0 {
		sb.WriteString(fmt.Sprintf("## Symbol index\n\n%d key types and functions. [Full index →](./symbol-index.md)\n\n", artifacts.symbolCount))
	}
	if !cfg.noChurn && artifacts.hasChurn {
		sb.WriteString("## Recent activity\n\nCommit frequency by directory (90 days). [Full analysis →](./churn.md)\n\n")
	}
	if !cfg.noRecipes && artifacts.hasRecipes {
		sb.WriteString("## Modification recipes\n\nCommon change patterns derived from git history. [Full recipes →](./recipes.md)\n\n")
	}
	if !cfg.noTraces && artifacts.hasTraces {
		sb.WriteString("## Entry point traces\n\nEnd-to-end execution flows through the codebase. [Full traces →](./traces.md)\n\n")
	}

	sb.WriteString("---\n\n")
```

Remove the existing `sb.WriteString("\n---\n\n")` that was after Contents to avoid double separators.

- [ ] **Step 3: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 4: Commit**

```bash
git add wikigen.go
git commit -m "feat: add root artifact links to wiki.md"
```

---

### Task 9: Main Loop Integration

**Files:**
- Modify: `wikigen.go:312-358` (main function, after directory processing loop)

- [ ] **Step 1: Add post-processing after directory summaries**

After the directory processing loop ends (line ~310) and before root overview generation (line ~312), insert the new post-processing pipeline:

```go
	// --- Sprint 005: Post-processing pipeline ---

	// Parse summaries for aggregation.
	var allFileEntries []fileIndexEntry
	var allSymbols []symbolEntry
	depsEdgeCount := 0

	if !cfg.dryRun {
		// Write root artifacts.
		if !cfg.noDepsGraph {
			if err := writeDepsGraph(cfg, summaries); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: deps graph generation failed: %v\n", err)
			} else {
				// Count edges for wiki.md summary.
				for _, s := range summaries {
					imports, _ := parseDependencies(s, "")
					if len(imports) > 0 {
						depsEdgeCount++
					}
				}
			}
		}

		if !cfg.noFileIndex {
			if err := writeFileIndex(cfg, dirs, summaries); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: file index generation failed: %v\n", err)
			}
		}

		// Count files for wiki.md summary.
		fileCount := 0
		for _, rel := range dirs {
			abs := filepath.Join(cfg.repoRoot, rel)
			dirEntries, _ := os.ReadDir(abs)
			for _, e := range dirEntries {
				if !e.IsDir() && detectLanguage(e.Name()) != "" {
					fileCount++
				}
			}
		}

		if !cfg.noSymbolIndex {
			for _, s := range summaries {
				allSymbols = append(allSymbols, parseSymbols(s, "")...)
			}
			if err := writeSymbolIndex(cfg, summaries); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: symbol index generation failed: %v\n", err)
			}
		}

		// Git analysis.
		var churnData []churnEntry
		var clusters []coChangeCluster

		if !cfg.noChurn {
			churnData, _ = analyzeChurn(cfg.repoRoot)
			if err := writeChurn(cfg, churnData); err != nil {
				fmt.Fprintf(os.Stderr, "  warning: churn analysis failed: %v\n", err)
			}
		}

		if !cfg.noRecipes {
			clusters, _ = analyzeCoChanges(cfg.repoRoot)
			if len(clusters) > 0 {
				emitProgress(cfg, progressEvent{Event: "write", Dir: ".", Message: "Generating modification recipes"})
				if !cfg.jsonProgress {
					fmt.Fprintln(os.Stderr, "Generating modification recipes...")
				}
				recipesContent, err := generateRecipes(cfg, clusters, summaries)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  warning: recipe generation failed: %v\n", err)
				} else {
					writeRecipesFile(cfg, recipesContent)
				}
			}
		}

		var hasTraces bool
		if !cfg.noTraces {
			emitProgress(cfg, progressEvent{Event: "write", Dir: ".", Message: "Generating entry point traces"})
			if !cfg.jsonProgress {
				fmt.Fprintln(os.Stderr, "Generating entry point traces...")
			}
			tracesContent, err := generateTraces(cfg, summaries)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warning: trace generation failed: %v\n", err)
			} else {
				writeTracesFile(cfg, tracesContent)
				hasTraces = tracesContent != ""
			}
		}

		// Build wiki artifacts metadata.
		artifacts := wikiArtifacts{
			depsEdgeCount: depsEdgeCount,
			fileCount:     fileCount,
			dirCount:      len(dirs),
			symbolCount:   len(allSymbols),
			hasChurn:      len(churnData) > 0,
			hasRecipes:    len(clusters) > 0,
			hasTraces:     hasTraces,
		}
```

Then update the `buildWiki` call at line ~337 to pass the artifacts:

```go
		wiki := buildWiki(cfg, dirs, summaries, rootOverview, artifacts)
```

- [ ] **Step 2: Update doClean to remove new root files**

In `doClean()`, add the new root files to the cleanup list alongside `wiki.md` and `index.html`:

```go
	for _, name := range []string{
		"wiki.md", "index.html",
		"deps-graph.md", "deps-graph.html",
		"file-index.md", "file-index.html",
		"symbol-index.md", "symbol-index.html",
		"recipes.md", "recipes.html",
		"traces.md", "traces.html",
		"churn.md", "churn.html",
	} {
		p := filepath.Join(cfg.wikiRoot, name)
		if data, err := os.ReadFile(p); err == nil && strings.Contains(string(data), "wikigen") {
			os.Remove(p)
			fmt.Fprintf(os.Stderr, "Removed %s\n", p)
			removed++
		}
	}
```

- [ ] **Step 3: Verify compilation**

Run: `go build wikigen.go`
Expected: Compiles with no errors.

- [ ] **Step 4: Run go vet**

Run: `go vet wikigen.go`
Expected: No issues.

- [ ] **Step 5: Commit**

```bash
git add wikigen.go
git commit -m "feat: integrate Sprint 005 post-processing pipeline into main loop"
```

---

### Task 10: Dry-Run Smoke Test

**Files:** None (testing only)

- [ ] **Step 1: Test compilation**

Run: `go build -o /tmp/wikigen wikigen.go`
Expected: Binary built successfully.

- [ ] **Step 2: Test help output**

Run: `/tmp/wikigen --help`
Expected: All 8 new `--no-*` flags appear in help text.

- [ ] **Step 3: Test dry-run**

Run: `/tmp/wikigen --dry-run .`
Expected: Shows directories that would be summarized, no API calls, no errors.

- [ ] **Step 4: Test dry-run with opt-out flags**

Run: `/tmp/wikigen --dry-run --no-recipes --no-traces .`
Expected: Runs without errors, respects flags.

- [ ] **Step 5: Commit any fixes**

If any issues found, fix and commit:
```bash
git add wikigen.go
git commit -m "fix: address issues found during dry-run smoke test"
```
