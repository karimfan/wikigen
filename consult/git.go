package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type expert struct {
	Name       string
	Email      string
	SlackID    string
	Commits90d int
	Commits1y  int
	BlameLines int
	Score      float64
	LastCommit string
}

func analyzeExperts(repoRoot, target string) ([]expert, error) {
	gitDir := filepath.Join(repoRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository: %s", repoRoot)
	}

	absTarget := target
	if !filepath.IsAbs(target) {
		absTarget = filepath.Join(repoRoot, target)
	}

	info, err := os.Stat(absTarget)
	if err != nil {
		return nil, fmt.Errorf("cannot stat target %s: %v", target, err)
	}

	// Gather files to analyze
	var files []string
	if info.IsDir() {
		files = listSourceFiles(repoRoot, target, 20)
	} else {
		files = []string{target}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no source files found in %s", target)
	}

	// Aggregate commit counts per email
	commits90d := make(map[string]int)
	commits1y := make(map[string]int)
	blameLines := make(map[string]int)

	for _, f := range files {
		c90 := countCommits(repoRoot, f, "90 days ago")
		for email, n := range c90 {
			commits90d[email] += n
		}
		c1y := countCommits(repoRoot, f, "1 year ago")
		for email, n := range c1y {
			commits1y[email] += n
		}

		// Blame only for files (not dirs, and only if the file exists)
		absF := f
		if !filepath.IsAbs(f) {
			absF = filepath.Join(repoRoot, f)
		}
		if fi, err := os.Stat(absF); err == nil && !fi.IsDir() {
			bl := blameFile(repoRoot, f)
			for email, n := range bl {
				blameLines[email] += n
			}
		}
	}

	// Collect all unique emails
	emails := make(map[string]bool)
	for e := range commits90d {
		emails[e] = true
	}
	for e := range commits1y {
		emails[e] = true
	}
	for e := range blameLines {
		emails[e] = true
	}

	// Build expert list
	var experts []expert
	for email := range emails {
		c90 := commits90d[email]
		c1y := commits1y[email]
		bl := blameLines[email]
		score := float64(c90*3) + float64(c1y*1) + float64(bl)*0.5

		name := getAuthorName(repoRoot, email)
		lastCommit := getLastCommitDate(repoRoot, email, target)

		experts = append(experts, expert{
			Name:       name,
			Email:      email,
			Commits90d: c90,
			Commits1y:  c1y,
			BlameLines: bl,
			Score:      score,
			LastCommit: lastCommit,
		})
	}

	// Sort by score descending
	sort.Slice(experts, func(i, j int) bool {
		return experts[i].Score > experts[j].Score
	})

	// Return top 3
	if len(experts) > 3 {
		experts = experts[:3]
	}

	return experts, nil
}

func countCommits(repoRoot, path, since string) map[string]int {
	cmd := exec.Command("git", "log", "--format=%ae", "--since="+since, "--", path)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	counts := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		email := strings.TrimSpace(scanner.Text())
		if email != "" {
			counts[email]++
		}
	}
	return counts
}

func blameFile(repoRoot, filePath string) map[string]int {
	cmd := exec.Command("git", "blame", "--porcelain", "--", filePath)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	counts := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "author-mail ") {
			email := strings.TrimPrefix(line, "author-mail ")
			email = strings.Trim(email, "<>")
			if email != "" && email != "not.committed.yet" {
				counts[email]++
			}
		}
	}
	return counts
}

func getAuthorName(repoRoot, email string) string {
	cmd := exec.Command("git", "log", "--format=%an", "--author="+email, "-1")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return email
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return email
	}
	return name
}

func getLastCommitDate(repoRoot, email, path string) string {
	cmd := exec.Command("git", "log", "--format=%as", "--author="+email, "-1", "--", path)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	date := strings.TrimSpace(string(out))
	if date == "" {
		return "unknown"
	}
	return date
}

func listSourceFiles(repoRoot, dir string, maxFiles int) []string {
	cmd := exec.Command("git", "log", "--all", "--pretty=format:", "--name-only", "--diff-filter=ACMR", "-100", "--", dir)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		f := strings.TrimSpace(scanner.Text())
		if f == "" || seen[f] {
			continue
		}
		// Check file still exists
		absF := filepath.Join(repoRoot, f)
		if _, err := os.Stat(absF); err != nil {
			continue
		}
		seen[f] = true
		files = append(files, f)
		if len(files) >= maxFiles {
			break
		}
	}
	return files
}

func printExperts(experts []expert) {
	fmt.Printf("\n%-4s %-20s %-30s %6s %6s %6s %8s %s\n",
		"Rank", "Name", "Email", "90d", "1yr", "Blame", "Score", "Last Commit")
	fmt.Println(strings.Repeat("-", 100))
	for i, e := range experts {
		fmt.Printf("%-4d %-20s %-30s %6d %6d %6d %8.1f %s\n",
			i+1,
			truncate(e.Name, 20),
			truncate(e.Email, 30),
			e.Commits90d,
			e.Commits1y,
			e.BlameLines,
			e.Score,
			e.LastCommit,
		)
	}
	fmt.Println()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
