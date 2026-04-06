# consult

Identifies code experts via git blame and log analysis, then lets LLM agents ask them questions or propose changes via Slack. The tool finds who has been most active in the code being modified and sends them a structured message with context — the same way a junior developer would ask a senior for help.

## How it works

1. You point it at a file or directory
2. It analyzes `git log` (commit frequency) and `git blame` (line ownership) to rank experts
3. It looks up the top expert on Slack via their git commit email
4. It sends a structured message with the question, file context, and a wiki excerpt (if available)
5. The expert replies in a Slack thread
6. You poll for the response with `consult check`

## Requirements

- Go 1.21+
- Git (required)
- `SLACK_BOT_TOKEN` environment variable (for `ask`, `propose`, `check` only)
  - Required Slack bot scopes: `chat:write`, `users:read.email`, `im:write`

No Slack token needed for `who`, `sessions`, or `--dry-run`.

## Commands

### `consult who` — Identify experts

Find who knows about a file or directory. No Slack needed, no side effects.

```bash
consult who --file internal/auth/handler.go
consult who --dir internal/auth/
```

Output:
```
Name                      Email                          90d       1y    Blame    Score  Last Commit
-------------------------------------------------------------------------------------------------------------------
Alice Smith               alice@company.com               12       47      180    123.0  2026-04-01
Bob Jones                 bob@company.com                  5       23       90     83.0  2026-03-15
Carol Lee                 carol@company.com                2        8       45     36.5  2026-02-20
```

### `consult ask` — Ask a question

Send a question to the top expert via Slack DM.

```bash
consult ask --file internal/auth/handler.go \
    --question "I'm adding rate limiting. Is there an existing middleware I should use?"
```

Output:
```
Consultation sent to Alice Smith (alice@company.com)
Session ID: a1b2c3d4
Check for response: consult check --session a1b2c3d4
```

### `consult propose` — Propose a change

Send a proposed change with a diff for review.

```bash
consult propose --file internal/auth/handler.go \
    --diff "$(git diff HEAD)" \
    --question "Does this change look safe? Worried about session handling."
```

The diff is included in the Slack message (truncated to 20 lines).

### `consult check` — Check for a response

Poll Slack for a reply to a previous consultation.

```bash
consult check --session a1b2c3d4
```

Output (if replied):
```
Response received:
Yes, use the middleware in pkg/middleware/ratelimit.go — it already handles per-user rate limiting.
```

Output (if not yet):
```
No response yet.
```

### `consult sessions` — List all sessions

```bash
consult sessions
```

Output:
```
ID         File                                     Type       Status     Created
-----------------------------------------------------------------------------------------------
a1b2c3d4   internal/auth/handler.go                 ask        pending    2026-04-06T10:30:00
e5f6g7h8   internal/db/store.go                     propose    complete   2026-04-05T14:15:00
```

### `consult --update-skills` — Update skill files

Write the consult skill section into `CLAUDE.md` and `AGENTS.md` in the repo root.

```bash
consult --update-skills
```

## Flags

| Flag | Description |
|------|-------------|
| `--file <path>` | Target file to analyze |
| `--dir <path>` | Target directory to analyze |
| `--question <text>` | Question or context for the expert |
| `--diff <text>` | Diff content to include (for `propose`) |
| `--session <id>` | Session ID (for `check`) |
| `--repo <path>` | Repository root (default: current directory) |
| `--dry-run` | Show who would be contacted and the message, without sending |
| `--update-skills` | Update CLAUDE.md and AGENTS.md with consult skill |

## Dry run

Preview the message and experts without a Slack token:

```bash
consult ask --dry-run --file internal/auth/handler.go \
    --question "Is this the right place to add OAuth2?"
```

Shows the formatted Slack message and expert table to stderr. No message is sent, no session is created, no token is required.

## Expert ranking

Experts are scored with a weighted formula:

```
Score = (commits in last 90 days * 3) + (commits in last year * 1) + (blame lines * 0.5)
```

- **Recent commits** (3x weight): People who touched the code recently are most likely to have context
- **Total commits** (1x): Longer history means deeper understanding
- **Blame lines** (0.5x): Current line ownership signals ongoing responsibility

The top 3 experts are returned. For directories, the tool analyzes up to 20 recently-modified files and merges scores across them.

## Identity resolution

**Primary**: Slack `users.lookupByEmail` — looks up the expert's git commit email in Slack. Zero config when emails match.

**Fallback**: Optional `.consult.json` at repo root for overrides when emails don't match:

```json
{
  "user_map": {
    "alice-old@personal.com": "U12345",
    "bob@contractor.io": "U67890"
  },
  "default_channel": "C_TEAM_CHANNEL"
}
```

If neither lookup nor config resolves a user, consult reports who it found in git but couldn't map to Slack, and suggests adding them to `.consult.json`.

## Session storage

Sessions are stored in `.consult/` under the repo root:

```
.consult/
  a1b2c3d4.json
  e5f6g7h8.json
```

Each session tracks the question, experts contacted, Slack thread, and response status. Add `.consult/` to your `.gitignore`.

## Slack message format

Messages are structured for quick human scanning:

```
🔍 Code Consultation Request

File: internal/auth/handler.go
Type: Question
From: LLM Agent (via consult)

Question:
I'm adding rate limiting to the login handler. Is there an existing
middleware I should use instead of implementing it inline?

Why you: You've made 12 commits to this file in the last 90 days (47 in the last year).

File summary (from wiki):
> HTTP request handlers for authentication. Handles login, logout, session validation.

━━━
Reply in this thread. The agent will poll for your response.
```

## LLM skill file

`consult --update-skills` writes a skill section into `CLAUDE.md` and `AGENTS.md` that teaches LLM agents when and how to consult humans. The skill uses `<!-- consult:start -->` / `<!-- consult:end -->` markers for safe updates.

### Decision framework

The skill teaches a three-layer approach:

1. **Wiki sufficient?** Check the wiki for architecture, API contracts, design rationale. If documented, use it.
2. **Source sufficient?** Read the source and git history. If intent is clear, proceed.
3. **Consult a human.** When neither wiki nor source resolves the question.

### When to consult

- Modifying **unfamiliar code in a high-churn area**
- Changing a **public API** that other packages depend on
- Code has **warning comments** (`HACK`, `XXX`, `DO NOT CHANGE`, `FRAGILE`)
- Need to understand **why** something was built a certain way (tribal knowledge)
- The cost of being wrong is **high** and you're not confident

### When NOT to consult

- Adding tests, fixing typos, formatting changes
- Well-documented code with clear patterns
- Changes within an internal directory that nothing depends on
- Straightforward feature additions following existing patterns

### Tips for good questions

- **State what you already know**: "I read the wiki entry and git history, but I'm unclear on..."
- **Be specific**: Name the file and the concern, not "how does auth work?"
- **Explain your proposed change**: So the expert can flag risks you don't see
- **One question per consult**: Multiple unrelated questions reduce response quality
