# Air Rarotonga CLI

**Live Cook Islands flight fares and the full Air Rarotonga route network, agent-native with offline search.**

Search real flights and fares across the Cook Islands and South Pacific, find the cheapest day to fly, and explore the island route graph offline. Powered by Air Rarotonga's Sabre EzyCommerce API.

## Install

The recommended path installs both the `airraro-pp-cli` binary and the `pp-airraro` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install airraro
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install airraro --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install airraro --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install airraro --agent claude-code
npx -y @mvanhorn/printing-press-library install airraro --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airraro/cmd/airraro-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airraro-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install airraro --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airraro --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airraro --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install airraro --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airraro-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AIRRARO_TENANT_ID` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airraro/cmd/airraro-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "airraro": {
      "command": "airraro-pp-mcp",
      "env": {
        "AIRRARO_TENANT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# health check, no auth needed
airraro doctor --dry-run

# live flights and fares
airraro flights search ...

```

## Unique Features

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

## Usage

Run `airraro-pp-cli --help` for the full command reference and flag list.

## Commands

### airports

Air Rarotonga route network and airport connections

- **`airraro-pp-cli airports <languageCode>`** - Full route network: every origin airport and the destinations it connects to

### config

Tenant configuration

- **`airraro-pp-cli config`** - Tenant configuration document

### content

CMS content (Prismic-backed)

- **`airraro-pp-cli content`** - Query CMS content blocks by custom type

### flights

Search live Air Rarotonga flights and fares

- **`airraro-pp-cli flights`** - Search flights and fares for a route over a date window (Sabre EzyCommerce SearchShop)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airraro-pp-cli config

# JSON for scripting and agents
airraro-pp-cli config --json

# Filter to specific fields
airraro-pp-cli config --json --select id,name,status

# Dry run — show the request without sending
airraro-pp-cli config --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airraro-pp-cli config --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
airraro-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/airraro/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `AIRRARO_TENANT_ID` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `airraro-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `airraro-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AIRRARO_TENANT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
