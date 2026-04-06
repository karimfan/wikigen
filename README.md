<p align="center">
  <img src="logo.svg" width="200" alt="foxhound logo">
</p>

# foxhound

Tools that help LLM agents understand code before they modify it.

An LLM writing code in an unfamiliar repo faces two problems: it doesn't know the codebase's structure, and it can't ask the people who do. foxhound solves both.

| Tool | Problem it solves | How |
|------|------------------|-----|
| [**wikigen**](wikigen/) | "I don't understand this codebase" | Generates a navigable wiki with summaries, dependency graphs, file indexes, and modification recipes |
| [**consult**](consult/) | "I should ask someone before changing this" | Finds code experts via git history and sends them questions via Slack |

## Install

```bash
go build -o wikigen ./wikigen/
go build -o consult ./consult/
```

Requires Go 1.21+ and Git.

## 30-second demo

```bash
# Generate a wiki for any repo
export ANTHROPIC_API_KEY=sk-...
./wikigen --output /tmp/wiki /path/to/repo

# Find who knows about a file
./consult who --file internal/auth/handler.go

# Ask them a question via Slack
export SLACK_BOT_TOKEN=xoxb-...
./consult ask --file internal/auth/handler.go \
    --question "Is there a rate limit middleware I should use?"
```

## How they work together

1. **wikigen** generates a wiki for the repo. The wiki includes skill files (`CLAUDE.md` / `AGENTS.md`) that teach the LLM how to navigate it.
2. The LLM reads the wiki to understand the codebase structure, dependencies, and common modification patterns.
3. When the LLM encounters code it doesn't fully understand — or a change that feels risky — **consult** identifies the right human to ask, based on who has been active in that code recently.
4. The human responds in Slack. The LLM polls for the response and incorporates the feedback.

The wiki handles the 80% case (understanding structure). Consult handles the 20% where human judgment is needed.

## Project structure

```
foxhound/
  wikigen/             → see wikigen/README.md
  consult/             → see consult/README.md
  docs/sprints/        Sprint planning documents
  go.mod               Shared Go module
```

## Tool documentation

Each tool has its own README with full usage, flags, examples, and skill file documentation:

- **[wikigen/README.md](wikigen/README.md)** — Wiki generator
- **[consult/README.md](consult/README.md)** — Expert consultation

## Sample skill files

The [`sample-skills/`](sample-skills/) directory contains example `CLAUDE.md` and `AGENTS.md` files showing what both tools generate when installed on a real repo. These are the skill files that teach LLM agents how to navigate the wiki and consult human experts.

- **[sample-skills/CLAUDE.md](sample-skills/CLAUDE.md)** — Example skill file for Claude Code
- **[sample-skills/AGENTS.md](sample-skills/AGENTS.md)** — Example skill file for Codex
