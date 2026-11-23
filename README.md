# tmuxizer

A terminal workflow orchestrator. Open a terminal, pick a project with vim keys, and get a [tmux](https://github.com/tmux/tmux) session with your app running, your editor open, and your AI agents ready, with a free shell to work in.

```
 ▌tmuxizer                                                  42 projects · 2 live
 ────────────────────────────────────────────────────────────────────────────
 live  ● tmuxizer  ● web-app
 ▌●go  tmuxizer                    ╭──────────────────────────────────────╮
  ●ts  web-app                     │ tmuxizer ●                           │
   py  ml-lab                      │ ~/code/tmuxizer                      │
   php shop                        │                                      │
   ts  docs                        │ stack    Go · go                     │
                                   │ run      go run ./cmd/tmuxizer       │
                                   │ config   .tmuxizer.toml · split      │
                                   │                                      │
                                   │ git      main ● dirty ↑1             │
                                   │          +1 staged  ~2 modified      │
                                   │ commit   fix auth redirect · 2h ago  │
                                   │                                      │
                                   │ session  ongoing  tmuxizer - Nov 23  │
                                   │ note     “wire the auth middleware”  │
                                   ╰──────────────────────────────────────╯
 j/k move · enter open · ←/→ live sessions · / filter · x close · t shell here · q quit
```

## What it does

- **Finds your projects.** Scans your workspace folders for git repos and Go, TypeScript/JavaScript, Python and PHP projects. No registering by hand.
- **Understands them.** Detects the stack, package manager and framework, and predicts how to run each project (see [Detection](#detection)).
- **Builds the session.** One tmux session per project: a `term` window (focused, for commands), an `app` window running the dev server, plus windows for your editor and chosen agents.
- **Shows what matters.** The preview colours git state (staged, modified, new, conflicts, ahead/behind, last commit) and your last [iwashere](https://github.com/Murchoid/iwashere) note, and a live strip lists the sessions already running.
- **Remembers.** Recents sort first, launch choices are saved per project, and the picker starts instantly from a cache.
- **Tracks your work** through [iwashere](https://github.com/Murchoid/iwashere): a session starts when you open a project and ends when you close the tmux session. After a reboot or crash it asks whether to resume or start fresh.
- **Wears your theme.** It only uses the terminal's own ANSI palette and never paints a background, so it follows whatever theme your terminal or tmux already has.

## Install

Needs [Go](https://go.dev/dl/) 1.26+ and [tmux](https://github.com/tmux/tmux).

From a checkout (recommended, it lets `tmuxizer update` pull and rebuild):

```sh
./install.sh        # builds to ~/.local/bin, then runs `tmuxizer setup`
./install.sh -y     # same, answering yes to every question
```

Or `go install github.com/devalentineomonya/tmuxizer/cmd/tmuxizer@latest` and run `tmuxizer setup` yourself.

`tmuxizer setup` is safe to repeat. It writes a starter config (never overwrites one), offers to add the shell hook so every new terminal opens the picker, and offers to build [iwashere](https://github.com/Murchoid/iwashere) if it is missing. Each step that touches something outside tmuxizer's own folders asks first. `tmuxizer setup --remove` takes the hook out again.

If you prefer to do it by hand: `eval "$(tmuxizer init zsh)"` in your rc (or bash, fish). The picker is skipped inside tmux. Press `q` to drop straight to your normal shell.

| Command | |
| --- | --- |
| `tmuxizer setup [-y] [--remove]` | config, shell hook, iwashere |
| `tmuxizer doctor` | checks tmux (3.1+), git, iwashere, editors, config, folders, hook and PATH. Exits 1 on a failure |
| `tmuxizer update [--check]` | pulls the checkout it was built from (if it tracks a remote), rebuilds and replaces the binary. Without a checkout it runs `go install` |
| `tmuxizer time [today\|week\|all]` | time spent per project |

## Keys

| Picker | |
| --- | --- |
| `j` `k` / arrows | move |
| `ctrl+d` `ctrl+u` | half page |
| `g` `G` | top, bottom |
| `/` | fuzzy filter, matched letters are underlined (`enter` accepts, `esc` clears) |
| `enter` | attach if its session is running, otherwise show the launch options |
| `→` `l` / `←` `h` | jump to the next / previous running session (selects only, never opens) |
| `s` | time spent per project; `h` `l` switch today, this week, all time |
| `x` | close its running session (asks first) |
| `t` | plain shell in the project directory, no tmux |
| `q` `esc` | quit to your shell |

The launch options open as a dialog over the picker. The session name input is focused first and cannot be left empty while iwashere tracking is on.

| Launch options | |
| --- | --- |
| `enter` | in the name input, accept it and move on; on an option row, launch |
| `j` `k` | move |
| `h` `l` | change run command or editor |
| `space` | toggle an agent or iwashere |
| `/` | edit the session name from any row (`esc` cancels the edit) |
| `enter` `e` | on the name row, edit it |
| `esc` `q` `backspace` | back to the project list |

Projects that already have a session attach straight away. Inside tmux, tmuxizer switches client instead of nesting.

## Detection

| Language | Detected from | Run command predicted from |
| --- | --- | --- |
| TypeScript / JavaScript | `package.json`, `tsconfig.json` | first of `dev`, `start:dev`, `develop`, `serve`, `start`, run with the package manager from `packageManager` or the lockfile ([pnpm](https://pnpm.io), [yarn](https://yarnpkg.com), [bun](https://bun.sh), [npm](https://www.npmjs.com)) |
| Go | `go.mod` | `.air.toml` (air), root `main.go`, or `cmd/*/main.go` |
| Python | `pyproject.toml`, `requirements.txt`, `Pipfile` | Django `manage.py`, FastAPI (uvicorn), Flask, or `main.py`; prefixed with `uv run`, `poetry run`, etc. when used |
| PHP | `composer.json` | Laravel `artisan serve`, composer `dev`/`start`, or the built-in server for `public/index.php` |

Every project also lists its Makefile targets and Docker scripts ([`docker compose up`](https://docs.docker.com/compose/) and friends) as options. For a project with only a compose file, `docker compose up` is the run command.

## Per-project config

Drop a `.tmuxizer.toml` in a project to pin how it opens. Everything is optional.

```toml
run    = "pnpm dev"        # command for the app window
editor = "nvim"            # editor to open
agents = ["claude"]        # agents to open (omit to use your remembered choice)
track  = true              # record an iwashere session
session = "{project} {branch}"  # iwashere session title, see below
layout = "split"           # "windows" (default) or "split"

[[windows]]                # extra windows opened with the project
name    = "logs"
command = "docker compose logs -f"

[[windows]]
name    = "api"
command = "air"
dir     = "apps/api"       # relative to the project
```

- `layout = "split"` runs the app in a side pane next to your shell instead of its own window.
- Values set here win over remembered choices and detection, and are marked `pinned` on the launch screen. You can still change them for that launch.
- Extra windows are listed on the launch screen before anything runs. `term` and `app` are reserved names.
- A typo or invalid value is shown on the launch screen and blocks the launch until fixed.
- A manifest runs commands, like `package.json` scripts do. Only open projects you trust.

## Sessions and iwashere

tmuxizer drives the [iwashere](https://github.com/Murchoid/iwashere) CLI, so you need it installed to get tracking. The row is hidden otherwise.

1. Opening a project runs `iwashere init` (only if the project has no `.iwashere` yet) and `iwashere session start`.
2. A tmux `session-closed` hook (installed at its own index, so your hooks are untouched) runs `iwashere session end` when you close the session.
3. If the machine loses power the hook never fires. Next launch, tmuxizer sees a recorded session with no tmux session behind it and asks: **resume** (rebuild the layout and continue the work session), **start new** (end the old one, begin another), or **dismiss**.

### Naming the session and seeing time spent

Each iwashere session gets a title. The default is `{project} - {date} {time}`. Tokens: `{project}`, `{branch}`, `{date}`, `{time}`. The launch screen shows the title with a preview of how it will read, and you can edit it per launch. The order is: what you typed, then the manifest's `session`, then `session_name` in `config.toml`, then the default.

Time comes straight from [iwashere](https://github.com/Murchoid/iwashere)'s session files:

- the preview shows total time, time today and the session count for the highlighted project;
- `s` in the picker (or `tmuxizer time`) shows a bar per project for today, this week (Monday start) or all time;
- a session counts toward the day it stopped, or today while it is running;
- if the machine lost power, the time before iwashere's clean end can include the powered-off gap. This is a limit of the data iwashere records.

## Configuration

Optional: `~/.config/tmuxizer/config.toml`

```toml
roots  = ["~/workspace", "~/code"]   # default: common folders under $HOME that exist
depth  = 3                           # how far below each root to look
ignore = ["archive"]                 # added to node_modules, vendor, dist, ...
editor = "nvim"                      # default editor; otherwise $EDITOR, then zed/cursor/code/nvim/helix/vim
accent = "magenta"                   # ANSI name or index 0-15, or #rrggbb
session_name = "{project} - {date} {time}"   # default iwashere session title
```

State (recents, choices, project cache, live sessions) lives in `~/.local/state/tmuxizer/state.json`.

Editors and agents are detected from `PATH`.

- Editors: [Zed](https://zed.dev), [Cursor](https://cursor.com), [VS Code](https://code.visualstudio.com), [Neovim](https://neovim.io), [Helix](https://helix-editor.com), [Vim](https://www.vim.org).
- Agents: [Claude Code](https://github.com/anthropics/claude-code), [Antigravity](https://antigravity.google), [Gemini CLI](https://github.com/google-gemini/gemini-cli), [Codex](https://github.com/openai/codex), [opencode](https://opencode.ai), [Aider](https://aider.chat).

GUI tools open their own window; terminal tools get a tmux window.

## Architecture

The UI picks, the launcher builds, and tmux, iwashere and git stay external CLIs.

```
 picker ──▶ options ──▶ Result{Plan} ──▶ launcher ──▶ tmux session
   │                                        │            ├─ term   (focused)
 stack · git · theme                   iwashere · store  ├─ app    (dev server)
                                                         ├─ editor
                                                         └─ agents
```

Details, flows and design decisions: [docs/architecture.md](docs/architecture.md). Contributor and agent rules: [AGENTS.md](AGENTS.md) (`CLAUDE.md` and `GEMINI.md` are symlinks to it).

### Layout

```
cmd/tmuxizer/      entrypoint, CLI wiring
internal/
  config/          config.toml and XDG paths
  stack/           language, framework and script detection
  project/         project discovery
  store/           persisted state
  tmux/            tmux command wrapper
  tools/           editor and agent detection
  iwashere/        iwashere CLI integration
  launcher/        builds sessions, handles close and resume
  git/             branch and status
  theme/           ANSI-palette styles
  ui/              [Bubble Tea](https://github.com/charmbracelet/bubbletea) screens
```

## Development

```sh
make test    # go test -race ./...
make lint    # go vet + golangci-lint
make build   # bin/tmuxizer
```

Commits follow [Conventional Commits](https://www.conventionalcommits.org/).

## Built with

- [iwashere](https://github.com/Murchoid/iwashere) by Murchoid for session and time tracking
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles) and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for the interface
- [go-toml](https://github.com/pelletier/go-toml) for config and manifests
- [tmux](https://github.com/tmux/tmux) for the sessions
- Inspired by [devflow-workspace-orchestrator](https://github.com/swaindhruti/devflow-workspace-orchestrator)

## License

MIT
