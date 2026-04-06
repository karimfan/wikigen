package main

import (
	"fmt"
	"os"
	"strings"
)

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

func notImplemented(name string) {
	fmt.Fprintf(os.Stderr, "%s: not yet implemented\n", name)
	os.Exit(1)
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
		notImplemented("ask")
	case cmd == "propose":
		notImplemented("propose")
	case cmd == "check":
		notImplemented("check")
	case cmd == "sessions":
		notImplemented("sessions")
	case cmd == "--update-skills":
		notImplemented("--update-skills")
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
