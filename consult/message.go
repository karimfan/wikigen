package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func buildAskMessage(question string, experts []expert, filePath, wikiExcerpt string) string {
	var b strings.Builder

	b.WriteString(":rotating_light: *Consult Request*\n")
	b.WriteString(fmt.Sprintf("*File:* `%s`\n", filePath))
	b.WriteString("*Type:* Question\n")
	b.WriteString("*From:* LLM Agent\n\n")
	b.WriteString(fmt.Sprintf("*Question:*\n%s\n\n", question))

	b.WriteString("*Why you:*\n")
	for _, e := range experts {
		b.WriteString(fmt.Sprintf("• %s — %d commits (90d), %d commits (1yr)\n",
			e.Name, e.Commits90d, e.Commits1y))
	}
	b.WriteString("\n")

	if wikiExcerpt != "" {
		b.WriteString("*Wiki context:*\n")
		for _, line := range strings.Split(wikiExcerpt, "\n") {
			b.WriteString("> " + line + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("_Reply in this thread. The agent will poll for your response._")
	return b.String()
}

func buildProposeMessage(question, diff string, experts []expert, filePath, wikiExcerpt string) string {
	var b strings.Builder

	b.WriteString(":rotating_light: *Consult Request*\n")
	b.WriteString(fmt.Sprintf("*File:* `%s`\n", filePath))
	b.WriteString("*Type:* Proposed Change\n")
	b.WriteString("*From:* LLM Agent\n\n")
	b.WriteString(fmt.Sprintf("*Question:*\n%s\n\n", question))

	// Truncate diff to 20 lines
	lines := strings.Split(diff, "\n")
	if len(lines) > 20 {
		truncated := strings.Join(lines[:20], "\n")
		omitted := len(lines) - 20
		b.WriteString(fmt.Sprintf("*Diff:*\n```\n%s\n```\n_%d lines omitted_\n\n", truncated, omitted))
	} else {
		b.WriteString(fmt.Sprintf("*Diff:*\n```\n%s\n```\n\n", diff))
	}

	b.WriteString("*Why you:*\n")
	for _, e := range experts {
		b.WriteString(fmt.Sprintf("• %s — %d commits (90d), %d commits (1yr)\n",
			e.Name, e.Commits90d, e.Commits1y))
	}
	b.WriteString("\n")

	if wikiExcerpt != "" {
		b.WriteString("*Wiki context:*\n")
		for _, line := range strings.Split(wikiExcerpt, "\n") {
			b.WriteString("> " + line + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("_Reply in this thread. The agent will poll for your response._")
	return b.String()
}

func loadWikiExcerpt(repoRoot, filePath string) string {
	dir := filepath.Dir(filePath)

	// Try SUMMARY.md in the file's directory, then under docs/
	candidates := []string{
		filepath.Join(repoRoot, dir, "SUMMARY.md"),
		filepath.Join(repoRoot, "docs", dir, "SUMMARY.md"),
	}

	for _, path := range candidates {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		overview := extractOverview(string(content))
		if overview != "" {
			return firstNSentences(overview, 2)
		}
	}
	return ""
}

func extractOverview(content string) string {
	lines := strings.Split(content, "\n")
	inOverview := false
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Overview" {
			inOverview = true
			continue
		}
		if inOverview {
			// Stop at the next heading
			if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "# ") {
				break
			}
			result = append(result, line)
		}
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func firstNSentences(text string, n int) string {
	count := 0
	for i, ch := range text {
		if ch == '.' || ch == '!' || ch == '?' {
			count++
			if count >= n {
				return strings.TrimSpace(text[:i+1])
			}
		}
	}
	return strings.TrimSpace(text)
}
