---
name: pp-airraro
description: "Live Cook Islands flight fares and the full Air Rarotonga route network, agent-native with offline search. Trigger phrases: `air rarotonga fares`, `flights to aitutaki`, `cook islands flights`, `cheapest flight to aitutaki`, `airraro routes`, `use airraro`."
author: "Ian Tairea"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - airraro-pp-cli
    install:
      - kind: go
        bins: [airraro-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/travel/airraro/cmd/airraro-pp-cli
---

# Air Rarotonga — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `airraro-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install airraro --cli-only
   ```
2. Verify: `airraro-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airraro/cmd/airraro-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Search real flights and fares across the Cook Islands and South Pacific, find the cheapest day to fly, and explore the island route graph offline. Powered by Air Rarotonga's Sabre EzyCommerce API.

## When to Use This CLI

Use for Cook Islands and South Pacific air travel: live fares, schedules, route connectivity, and cheapest-day analysis for Air Rarotonga.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to book or pay for flights — it is search/fares only.
- Do not use for non-Air-Rarotonga carriers.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Fare intelligence
- **`cheapest`** — Find the lowest fare per day across a date window for a route.

  _Agents can answer 'when is it cheapest to fly to Aitutaki' in one call._

  ```bash
  airraro cheapest RAR AIT --date 2026-07-15 --window 7 --agent
  ```
- **`fare-calendar`** — Fares for a route across many dates, persisted for trend tracking.

  _Track price movement the website cannot show historically._

  ```bash
  airraro fare-calendar RAR AIT --date 2026-07-15 --days 14 --agent
  ```

### Local state that compounds
- **`network`** — Full Cook Islands route graph stored locally in SQLite.

  _Answer connectivity questions without a network round-trip._

  ```bash
  airraro network --from RAR --agent
  ```

## Command Reference

**airports** — Air Rarotonga route network and airport connections

- `airraro-pp-cli airports <languageCode>` — Full route network: every origin airport and the destinations it connects to

**config** — Tenant configuration

- `airraro-pp-cli config` — Tenant configuration document

**content** — CMS content (Prismic-backed)

- `airraro-pp-cli content` — Query CMS content blocks by custom type

**flights** — Search live Air Rarotonga flights and fares

- `airraro-pp-cli flights` — Search flights and fares for a route over a date window (Sabre EzyCommerce SearchShop)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
airraro-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `airraro-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export AIRRARO_TENANT_ID="<your-key>"
```

Or persist it in `~/.config/airraro/config.toml`.

Run `airraro-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  airraro-pp-cli config --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
airraro-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
airraro-pp-cli feedback --stdin < notes.txt
airraro-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/airraro-pp-cli/feedback.jsonl`. They are never POSTed unless `AIRRARO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AIRRARO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
airraro-pp-cli profile save briefing --json
airraro-pp-cli --profile briefing config
airraro-pp-cli profile list --json
airraro-pp-cli profile show briefing
airraro-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `airraro-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/travel/airraro/cmd/airraro-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add airraro-pp-mcp -- airraro-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which airraro-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   airraro-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `airraro-pp-cli <command> --help`.
