# AGENTS.md

Guidance for AI coding agents working on tmuxizer. `CLAUDE.md` and `GEMINI.md` are symlinks to this file; edit only `AGENTS.md`.

## What this is

A Go terminal app: pick a project with vim keys, get a tmux session with the app running, an editor and optional AI agents open, and a free shell. Sessions are tracked with [iwashere](https://github.com/Murchoid/iwashere). Looks matter as much as function.

Full design: [docs/architecture.md](docs/architecture.md). User docs: [README.md](README.md).

## Commands

```sh
make test     # go test -race ./...
make lint     # go vet + golangci-lint
make build    # bin/tmuxizer
make run      # go run ./cmd/tmuxizer
gofmt -w .    # before every commit
```

Go 1.26, Bubble Tea v1 (`charmbracelet/bubbletea`), Lip Gloss v1. Pin `charmbracelet/x/ansi` to the version Bubble Tea selects; a newer one breaks `cellbuf`.

## Layout

```
cmd/tmuxizer/   entrypoint and wiring only
internal/
  config/ stack/ project/ store/ tmux/ tools/ iwashere/ git/ manifest/   leaf packages
  report/ setup/ doctor/ update/   back the CLI commands (time, setup, doctor, update)
  launcher/   composes tmux + iwashere + store into a session
  theme/      ANSI-palette styles
  ui/         Bubble Tea screens (picker, launch options, resume)
```

Leaf packages must not import each other. Composition happens in `launcher`, `ui` and `cmd`.

## Conventions

- Follow standard Go style: small packages, errors wrapped with `%w`, no panics, `context` only where something can block.
- Comments are short and say why, not what. Exported identifiers get a one-line doc comment that starts with the name.
- Inject side effects (tmux runner, iwashere runner, `LookPath`, clocks) so tests need no real tmux.
- New detection goes in `internal/stack` as a `detector` plus a table test case in `stack_test.go`.
- Commits use Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`).
- Only Go, TypeScript/JavaScript, Python and PHP are supported languages. Do not add others without being asked.

## Rules that are easy to break

1. **Colours are ANSI indexes 0-15 only.** No RGB, no hex defaults, no painted backgrounds. The terminal's theme must drive the look. User-set `accent` may be a hex value, the default may not.
2. **The `session-closed` hook uses foreground `run-shell`.** With `-b` the job is lost when the last session closes. Install it at array index 742 so user hooks are untouched.
3. **`display-message -t` needs `=name:`** (trailing colon) to return `#{session_path}`.
4. **Never call `iwashere init --force`**; it deletes the store. `Init` skips when a parent already has `.iwashere`.
5. **Tracking is best effort.** An iwashere failure must not stop a project from opening.
6. **The UI returns a `Result`; it does not launch anything.** Keep it testable by key messages.
7. **A manifest runs commands.** Anything from `.tmuxizer.toml` that will execute (run, windows) must be visible on the launch screen before launch. Keep `DisallowUnknownFields` so typos fail loudly, and keep `dir` local to the project.
8. **Keys can arrive coalesced** (`"jj"` as one rune message). New key handling goes through `Update`, which splits bursts; do not match multi-rune strings.
9. **Path filtering is substring, name filtering is fuzzy.** Do not fuzzy-match paths.
10. **State writes go through `store.Update`** (locked, atomic) because the TUI and the tmux hook can write at once.

11. **`setup` only edits its own marked block** in a shell rc file, resolves symlinks, writes atomically and never overwrites an existing config.
12. **Release builds stamp `main.version` and `main.source`** (Makefile and `install.sh`); `update` depends on `source`.

## Testing tmux safely

The shell you run in may be inside the user's real tmux, where `$TMUX` overrides `TMUX_TMPDIR`. A bare `tmux kill-server` then kills the user's server and all their sessions. This has happened.

- Always use an explicit socket and clear the variable: `env -u TMUX tmux -S /tmp/tz.sock ...`.
- Prefer `kill-session -t <name>` over `kill-server`.
- tmuxizer has no `-S` flag. To run it in isolation use `env -u TMUX TMUX_TMPDIR=/tmp/tz/B tmuxizer`, then clean up with `kill-session` under the same variables.
- Keep socket paths short (the unix socket path limit is about 100 bytes).
- Point tmuxizer at throwaway state with `XDG_CONFIG_HOME` and `XDG_STATE_HOME`, and put fixture projects in a temp dir.

## Before you finish

`gofmt -l .` prints nothing, `go vet ./...` is clean, `go test -race ./...` passes. If you changed the UI, run it in a scratch tmux and look at it at 40x8, 100x24 and 200x50.
