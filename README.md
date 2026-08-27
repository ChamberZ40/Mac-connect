<p align="center">
  <img src="./docs/images/banner.svg" alt="CC-Connect Banner" width="800"/>
</p>

<p align="center">
  <a href="./README.md">English</a> | <a href="./README.zh-CN.md">中文</a>
</p>

cc-connect is a bridge between AI coding agents running on your machine and the
messaging apps you already use. You send a message in Feishu or WeChat; the
agent runs locally in your project directory; its output comes back to the chat.

This is a trimmed deployment of [chenhg5/cc-connect](https://github.com/chenhg5/cc-connect)
carrying only the agents and platforms listed below. This README covers
deployment and configuration; everything else lives in [docs/](docs/).

<p align="center">
  <img src="docs/images/connector.png" alt="CC-Connect Architecture" width="90%"/>
</p>


## What's included

| Agent | Config |
|-------|--------|
| Claude Code | `type = "claudecode"` |
| Codex (OpenAI) | `type = "codex"` |
| Cursor Agent | `type = "cursor"` |
| GitHub Copilot CLI | `type = "copilot"` |
| ACP | `type = "acp"` — any [ACP-compatible agent](https://agentclientprotocol.com/get-started/agents), e.g. OpenClaw or Hermes |

| Platform | Connection | Public IP needed? | Setup guide |
|----------|------------|-------------------|-------------|
| Feishu (Lark) | WebSocket | No | [docs/feishu.md](docs/feishu.md) |
| WeChat Work | WebSocket / Webhook | No (WS) / Yes (Webhook) | [docs/wecom.md](docs/wecom.md) |
| Weixin (personal, ilink) | HTTP long polling | No | [docs/weixin.md](docs/weixin.md) |

Per-platform capabilities:

| Capability | Feishu | WeCom | Weixin *(personal)* |
|------------|:------:|:-----:|:-------------------:|
| Text & slash commands | ✅ | ✅ | ✅ |
| Markdown / cards | ✅ | ⚠️ | ✅ |
| Streaming / chunked replies | ✅ | ✅ | ✅ |
| Images & files | ✅ | ✅ | ✅ |
| Voice / STT / TTS | ⚠️ | ⚠️ | ✅ |
| Private (DM) | ✅ | ✅ | ✅ |
| Group / channel | ✅ | ✅ | ✅ |

⚠️ means partial, or needs extra configuration — the voice row in particular
requires `[speech]` / TTS providers in `config.toml`.


## Deploy

> **Install in this order.** cc-connect is a bridge for *local* agent CLIs, so
> the agent must be installed and authenticated **before** cc-connect starts.
> Skipping ahead makes cc-connect exit with `claudecode: claude CLI not found in
> PATH` (or the equivalent for your agent), and the Web UI on `:9820` never
> comes up.

### 1. Install an agent CLI

You need at least one.

```bash
# Claude Code
brew install --cask claude-code            # macOS / Linux Homebrew
npm install -g @anthropic-ai/claude-code   # or, any platform via npm

# OpenAI Codex
npm install -g @openai/codex

# GitHub Copilot CLI
npm install -g @github/copilot
```

Cursor Agent: follow <https://docs.cursor.com/agent>.
Any other ACP-speaking agent is configured as `type = "acp"`.

Confirm the binary is on your `PATH`:

```bash
claude --version       # or: codex / copilot / cursor-agent
```

### 2. Authenticate the agent

Run the agent once interactively so it stores credentials in your home
directory:

```bash
claude login           # opens a browser
codex login            # or: copilot / cursor-agent — see the agent's docs
```

Skip this and cc-connect still starts, but the agent rejects every prompt with
an auth error.

### 3. Install cc-connect

```bash
npm install -g cc-connect     # any platform
brew install cc-connect       # macOS / Linux
```

Or build from source (Go 1.22+, plus Node.js — `make build` also rebuilds the
embedded Web UI):

```bash
git clone https://github.com/ChamberZ40/Mac-connect.git
cd Mac-connect
make build                    # produces ./cc-connect
```

### 4. First run

```bash
cc-connect                    # auto-creates ~/.cc-connect/config.toml on first run
```

It prints the admin URL:

```
Web admin:  http://localhost:9820
```

If `9820` is taken, pass `--web-port 9821` or set `web_port` in `config.toml`.

> `cc-connect web` **only** opens the browser and the config UI — it does not
> start the bridge. Keep `cc-connect` running separately.

### 5. Add platform credentials

In the Web UI, create a project, add a platform (Feishu / WeChat Work /
Weixin), and paste the credentials from that platform's developer console.
Save; cc-connect hot-reloads. Send a message to your bot to confirm.

### Run as a service

```bash
cc-connect daemon install --config ~/.cc-connect/config.toml
cc-connect daemon start
cc-connect daemon status
cc-connect daemon restart
cc-connect daemon stop
cc-connect daemon uninstall
```

This installs a launchd agent on macOS, a systemd unit on Linux, and a Task
Scheduler task named `cc-connect` on Windows. On Linux, run
`loginctl enable-linger $USER` so the unit survives logout — `daemon install`
warns when linger is off.

### Upgrade

```bash
npm install -g cc-connect     # npm
brew upgrade cc-connect       # Homebrew
cc-connect update             # binary self-update, stable only
cc-connect update --pre       # include pre-releases
```

Self-update compares your build against upstream release tags. A locally built
binary stamped with SemVer build metadata (`v1.5.0+trim.1`) compares equal to
`v1.5.0`; a version that cannot be parsed at all makes `cc-connect update`
refuse rather than guess, so an unknown build is never talked into a downgrade.


## Configure

Config lives at `~/.cc-connect/config.toml`. The Web UI (`cc-connect web`)
edits it visually — projects, platforms, providers — with no TOML editing. To do
it by hand:

```bash
mkdir -p ~/.cc-connect
cp config.example.toml ~/.cc-connect/config.toml
vim ~/.cc-connect/config.toml
```

[config.example.toml](config.example.toml) is the annotated reference for every
option. The minimum viable shape is one project = one agent + one platform:

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "claudecode"          # or codex, cursor, copilot, acp

[projects.agent.options]
work_dir = "/path/to/project"
mode = "default"

[[projects.platforms]]
type = "feishu"              # or wecom, weixin

[projects.platforms.options]
app_id = "your-feishu-app-id"
app_secret = "your-feishu-app-secret"
```

One process can run many projects, each with its own agent + platform pair.

### Keep secrets out of the file

Any option value may reference an environment variable, which is how to avoid
committing credentials:

```toml
app_secret = "${FEISHU_APP_SECRET}"
```

### Privileged commands

`admin_from` lists the user IDs allowed to run privileged commands such as
`/dir` and `/shell`. It belongs under `[[projects]]` — **not** under
`[projects.platforms.options]`:

```toml
[[projects]]
admin_from = "alice,bob"
```

Use `/whoami` or `/status` in chat to find your own user ID.

### Session reset on idle

Projects rotate to a fresh session after inactivity. This prevents context
drift, where stale history (failed commands, debugging noise) is repeatedly
re-ingested via `--continue` and starts to dominate the model's attention. The
previous session is preserved and stays reachable via `/list` and `/switch`.

```toml
[[projects]]
reset_on_idle_mins = 30   # default when unset; 0 disables rotation
```

### Permission mode

```toml
[projects.agent.options]
mode = "default"
```

Switchable at runtime with `/mode`. Values are agent-specific: Claude Code takes
`default` / `acceptEdits` / `auto` / `plan` / `bypassPermissions`, Codex takes
`suggest` / `auto-edit` / `full-auto` / `yolo`, Cursor takes `default` / `force`
/ `plan` / `ask`, Copilot takes `default` / `bypassPermissions`.

### Attachment send-back

Agents can push generated files back into the chat:

```toml
attachment_send = "on"          # default "on"; "off" blocks image/file send-back
max_attachment_size_mb = 50     # default 50 MiB; or CC_MAX_ATTACHMENT_SIZE_MB
```

```bash
cc-connect send --image /absolute/path/to/chart.png
cc-connect send --file /absolute/path/to/report.pdf
cc-connect send --tts "Hello from cc-connect"
```

Currently delivered on Feishu. Absolute paths are safest; `--image` and `--file`
may both be repeated. This switch is independent of the agent's `/mode` — it
only gates `cc-connect send`, and ordinary text replies keep working when it is
`off`. Voice send-back uses the `[speech]` TTS config instead.

If your agent does not natively inject the system prompt, run `/bind setup` (or
`/cron setup`) once in chat after upgrading, to refresh the cc-connect
instructions in the project memory file.

### Scheduled tasks

```bash
/cron add 0 6 * * * Summarize GitHub trending
```

### OS-user isolation (`run_as_user`)

On Linux/macOS a project can spawn its agent under a different Unix user, for
file-system isolation from the supervisor user running cc-connect. Currently
supported by Claude Code.

```toml
[[projects]]
name = "claude-sandboxed"
run_as_user = "partseeker-coder"
run_as_env = ["PGSSLROOTCERT"]
```

The target user needs passwordless sudo from the supervisor, no sudo of its own,
read+write on `work_dir`, and its own `~/.claude/settings.json` with whatever
credentials the agent uses. Under `claude.ai` OAuth, symlink the target user's
`~/.claude/.credentials.json` to the supervisor's copy so token refresh stays in
sync — see the
[environment propagation checklist](./docs/usage.md#environment-propagation-what-moves-into-the-target-users-home).

Audit before starting:

```bash
cc-connect doctor user-isolation
```

Three go/no-go preflight gates plus an isolation probe reporting what the target
user can and cannot read. cc-connect refuses to start if a gate fails or the
probe finds a cross-user leak.


## Selective builds

Every agent and platform is imported through its own `plugin_*.go` file behind a
build tag, so a build can carry a subset. All of them are included by default.

```bash
make build AGENTS=claudecode PLATFORMS_INCLUDE=feishu
make build AGENTS=claudecode,codex PLATFORMS_INCLUDE=feishu,wecom
make build EXCLUDE=weixin,wecom

go build -tags 'no_weixin no_wecom' ./cmd/cc-connect   # without Make
```

Available tags: `no_acp`, `no_claudecode`, `no_codex`, `no_copilot`,
`no_cursor`, `no_feishu`, `no_wecom`, `no_weixin`.


## Runtime commands

Typed in chat, not in a shell.

```
/new [name]                 Start a new session
/list                       List sessions
/switch <id>                Switch session
/current                    Show current session
/dir [path|reset]           Show, switch, or reset the work directory
/dir <number> | /dir -      Jump through directory history
/mode [name]                Show or switch permission mode
/model [switch <alias>]     List or switch model
/provider [switch <name>]   List or switch API provider
/cron, /timer               Recurring and one-shot scheduled tasks
/cancel                     Interrupt the current turn
/whoami, /status            Identity and session state
```

`/dir reset` restores the configured `work_dir` and clears the persisted
override in `data_dir/projects/<project>.state.json`.

Full reference: [docs/usage.md](docs/usage.md).


## Documentation

- [docs/usage.md](docs/usage.md) — complete feature and command reference
- [INSTALL.md](INSTALL.md) — step-by-step install guide written for an AI agent to follow
- [config.example.toml](config.example.toml) — annotated configuration template
- [docs/management-api.md](docs/management-api.md) — HTTP management API
- [docs/bridge-protocol.md](docs/bridge-protocol.md) — WebSocket protocol for third-party platform adapters
- [CLAUDE.md](CLAUDE.md) / [AGENTS.md](AGENTS.md) — architecture and contribution rules


## License

[MIT](LICENSE) — use it however you like, commercially included; the only
condition is keeping the copyright and permission notice.

Upstream [chenhg5/cc-connect](https://github.com/chenhg5/cc-connect) declares MIT
in `npm/package.json` but ships no `LICENSE` file, so [LICENSE](LICENSE) carries
both its notice and this fork's.
