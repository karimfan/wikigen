package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/CONSULT_SKILL.md
var consultSkillTemplate []byte

type cmdFlags struct {
	file     string
	dir      string
	question string
	diff     string
	session  string
	dryRun   bool
	repoRoot string
}

func parseFlags(args []string) cmdFlags {
	var f cmdFlags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if i+1 < len(args) {
				i++
				f.file = args[i]
			}
		case "--dir":
			if i+1 < len(args) {
				i++
				f.dir = args[i]
			}
		case "--question", "-q":
			if i+1 < len(args) {
				i++
				f.question = args[i]
			}
		case "--diff":
			if i+1 < len(args) {
				i++
				f.diff = args[i]
			}
		case "--session":
			if i+1 < len(args) {
				i++
				f.session = args[i]
			}
		case "--dry-run":
			f.dryRun = true
		case "--repo-root":
			if i+1 < len(args) {
				i++
				f.repoRoot = args[i]
			}
		}
	}
	return f
}

func resolveRepoRoot(f cmdFlags) string {
	if f.repoRoot != "" {
		return f.repoRoot
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot determine working directory: %v\n", err)
		os.Exit(1)
	}
	return cwd
}

func targetPath(f cmdFlags) string {
	if f.file != "" {
		return f.file
	}
	return f.dir
}

func printUsage() {
	usage := `consult — find the right person, ask the right question

Usage:
  consult <command> [flags]

Commands:
  who              Identify top experts for a file or directory
  ask              Send a question to an expert via Slack DM
  propose          Share a diff/proposal with an expert for review
  check            Check for replies to a previous ask/propose
  sessions         List active consult sessions
  --update-skills  Update embedded LLM skill files

Flags:
  --file <path>       Target a specific file
  --dir <path>        Target a directory
  --question, -q <s>  Question to ask (for ask command)
  --diff <path>       Diff file to attach (for propose command)
  --session <id>      Session ID (for check command)
  --dry-run           Show what would happen without sending
  --repo-root <path>  Override repository root detection
  --help              Show this help message

Examples:
  consult who --file wikigen.go
  consult who --dir templates/
  consult ask --file wikigen.go -q "Is the heading parser intentionally recursive?"
  consult propose --file wikigen.go --diff changes.patch
  consult check --session 2024-01-15-abc123
  consult sessions
`
	fmt.Print(usage)
}

func cmdWho(args []string) {
	f := parseFlags(args)
	target := targetPath(f)
	if target == "" {
		fmt.Fprintf(os.Stderr, "error: who requires --file or --dir\n")
		os.Exit(1)
	}
	root := resolveRepoRoot(f)
	experts, err := analyzeExperts(root, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(experts) == 0 {
		fmt.Println("No experts found for", target)
		return
	}
	printExperts(experts)
}

func cmdAsk(args []string) {
	f := parseFlags(args)
	target := targetPath(f)
	if target == "" {
		fmt.Fprintf(os.Stderr, "error: ask requires --file or --dir\n")
		os.Exit(1)
	}
	if f.question == "" {
		fmt.Fprintf(os.Stderr, "error: ask requires --question / -q\n")
		os.Exit(1)
	}

	root := resolveRepoRoot(f)
	experts, err := analyzeExperts(root, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(experts) == 0 {
		fmt.Fprintf(os.Stderr, "error: no experts found for %s\n", target)
		os.Exit(1)
	}

	wikiExcerpt := loadWikiExcerpt(root, target)
	message := buildAskMessage(f.question, experts, target, wikiExcerpt)

	if f.dryRun {
		fmt.Fprintln(os.Stderr, "--- Dry Run ---")
		fmt.Fprintln(os.Stderr, "Experts:")
		printExperts(experts)
		fmt.Fprintln(os.Stderr, "Message:")
		fmt.Fprintln(os.Stderr, message)
		return
	}

	token := requireSlackToken()
	experts = resolveExpertSlackIDs(token, root, experts)

	if experts[0].SlackID == "" {
		fmt.Fprintf(os.Stderr, "error: could not resolve Slack ID for top expert %s <%s>\n", experts[0].Name, experts[0].Email)
		fmt.Fprintf(os.Stderr, "hint: add a mapping in .consult.json:\n")
		fmt.Fprintf(os.Stderr, "  {\"user_map\": {\"%s\": \"U01XXXXXX\"}}\n", experts[0].Email)
		os.Exit(1)
	}

	// Use default channel from config if available, otherwise DM the expert.
	cfg := loadConsultConfig(root)
	var channelID string
	if cfg != nil && cfg.DefaultChannel != "" {
		channelID = cfg.DefaultChannel
	} else {
		var err error
		channelID, err = openDM(token, experts[0].SlackID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening DM: %v\n", err)
			os.Exit(1)
		}
	}

	threadTS, err := postMessage(token, channelID, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error sending message: %v\n", err)
		os.Exit(1)
	}

	s, err := createSession(root, target, f.question, "ask", experts, channelID, threadTS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating session: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Contacted %s (%s) via Slack DM\n", experts[0].Name, experts[0].Email)
	fmt.Printf("Session ID: %s\n", s.ID)
	fmt.Printf("Check for a reply with: consult check --session %s\n", s.ID)
}

func cmdPropose(args []string) {
	f := parseFlags(args)
	target := targetPath(f)
	if target == "" {
		fmt.Fprintf(os.Stderr, "error: propose requires --file or --dir\n")
		os.Exit(1)
	}
	if f.question == "" {
		fmt.Fprintf(os.Stderr, "error: propose requires --question / -q\n")
		os.Exit(1)
	}
	if f.diff == "" {
		fmt.Fprintf(os.Stderr, "error: propose requires --diff\n")
		os.Exit(1)
	}

	root := resolveRepoRoot(f)
	experts, err := analyzeExperts(root, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(experts) == 0 {
		fmt.Fprintf(os.Stderr, "error: no experts found for %s\n", target)
		os.Exit(1)
	}

	wikiExcerpt := loadWikiExcerpt(root, target)
	message := buildProposeMessage(f.question, f.diff, experts, target, wikiExcerpt)

	if f.dryRun {
		fmt.Fprintln(os.Stderr, "--- Dry Run ---")
		fmt.Fprintln(os.Stderr, "Experts:")
		printExperts(experts)
		fmt.Fprintln(os.Stderr, "Message:")
		fmt.Fprintln(os.Stderr, message)
		return
	}

	token := requireSlackToken()
	experts = resolveExpertSlackIDs(token, root, experts)

	if experts[0].SlackID == "" {
		fmt.Fprintf(os.Stderr, "error: could not resolve Slack ID for top expert %s <%s>\n", experts[0].Name, experts[0].Email)
		fmt.Fprintf(os.Stderr, "hint: add a mapping in .consult.json:\n")
		fmt.Fprintf(os.Stderr, "  {\"user_map\": {\"%s\": \"U01XXXXXX\"}}\n", experts[0].Email)
		os.Exit(1)
	}

	// Use default channel from config if available, otherwise DM the expert.
	cfg := loadConsultConfig(root)
	var channelID string
	if cfg != nil && cfg.DefaultChannel != "" {
		channelID = cfg.DefaultChannel
	} else {
		var err error
		channelID, err = openDM(token, experts[0].SlackID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening DM: %v\n", err)
			os.Exit(1)
		}
	}

	threadTS, err := postMessage(token, channelID, message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error sending message: %v\n", err)
		os.Exit(1)
	}

	s, err := createSession(root, target, f.question, "propose", experts, channelID, threadTS)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating session: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Contacted %s (%s) via Slack DM\n", experts[0].Name, experts[0].Email)
	fmt.Printf("Session ID: %s\n", s.ID)
	fmt.Printf("Check for a reply with: consult check --session %s\n", s.ID)
}

func cmdCheck(args []string) {
	f := parseFlags(args)
	if f.session == "" {
		fmt.Fprintf(os.Stderr, "error: check requires --session\n")
		os.Exit(1)
	}

	root := resolveRepoRoot(f)
	s, err := loadSession(root, f.session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if s.Status == "complete" {
		fmt.Printf("Session %s is already complete.\n", s.ID)
		fmt.Printf("Response:\n%s\n", s.Response)
		return
	}

	token := requireSlackToken()
	response, found, err := checkSessionResponse(token, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error checking for response: %v\n", err)
		os.Exit(1)
	}

	if found {
		s.Status = "complete"
		s.Response = response
		if err := updateSession(root, s); err != nil {
			fmt.Fprintf(os.Stderr, "error updating session: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Response received for session %s:\n%s\n", s.ID, response)
	} else {
		fmt.Println("No response yet.")
	}
}

func cmdSessions(args []string) {
	f := parseFlags(args)
	root := resolveRepoRoot(f)
	sessions, err := listSessions(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(sessions) == 0 {
		fmt.Println("No consultation sessions found.")
		return
	}
	printSessions(sessions)
}

func cmdUpdateSkills(args []string) {
	f := parseFlags(args)
	root := resolveRepoRoot(f)
	skillContent := string(consultSkillTemplate)

	section := "<!-- consult:start -->\n" + skillContent + "\n<!-- consult:end -->"

	targets := []string{"CLAUDE.md", "AGENTS.md"}
	for _, name := range targets {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			// File doesn't exist — create with section
			if os.IsNotExist(err) {
				if err := os.WriteFile(path, []byte(section+"\n"), 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "error writing %s: %v\n", name, err)
					os.Exit(1)
				}
				fmt.Printf("Created %s with consult skill section\n", name)
				continue
			}
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", name, err)
			os.Exit(1)
		}

		content := string(data)
		startMarker := "<!-- consult:start -->"
		endMarker := "<!-- consult:end -->"
		startIdx := strings.Index(content, startMarker)
		endIdx := strings.Index(content, endMarker)

		if startIdx >= 0 && endIdx >= 0 && endIdx > startIdx {
			// Replace existing section
			content = content[:startIdx] + section + content[endIdx+len(endMarker):]
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing %s: %v\n", name, err)
				os.Exit(1)
			}
			fmt.Printf("Updated consult skill section in %s\n", name)
		} else {
			// Append section
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += "\n" + section + "\n"
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing %s: %v\n", name, err)
				os.Exit(1)
			}
			fmt.Printf("Appended consult skill section to %s\n", name)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

	switch {
	case cmd == "who":
		cmdWho(rest)
	case cmd == "ask":
		cmdAsk(rest)
	case cmd == "propose":
		cmdPropose(rest)
	case cmd == "check":
		cmdCheck(rest)
	case cmd == "sessions":
		cmdSessions(rest)
	case cmd == "--update-skills":
		cmdUpdateSkills(rest)
	case cmd == "--help" || cmd == "-h" || cmd == "help":
		printUsage()
	default:
		if strings.HasPrefix(cmd, "-") {
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", cmd)
		} else {
			fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		}
		printUsage()
		os.Exit(1)
	}
}
