# Design: `--watch` flag for list commands

## Overview

Add a `--watch` / `-w` flag (with optional `--watch-interval`) to list-style `hlctl` commands so users can re-run them on a fixed interval without writing a shell loop. Initial integration: `hlctl system utilization`, `hlctl docker containers list`, `hlctl network clients list`. A reusable helper in a new `internal/cli/watch` package lets any future command opt in with a one-line change.

The model is `watch(1)`-style — clear the screen and redraw each tick. When stdout is not a TTY, fall back to appending each snapshot so output remains pipe-friendly.

## Flags

Two flags, registered per-command via the helper:

- `--watch` / `-w` — `bool`, default `false`. Enables the watch loop.
- `--watch-interval` — `time.Duration`, default `2s`. Tick interval. Parsed via `time.ParseDuration` so `500ms`, `5s`, `1m` all work. Minimum enforced value: `100ms` (return an error below that to prevent runaway loops).

The interval flag has no effect when `--watch` is not set; that combination is silently allowed (not an error) so users can leave `--watch-interval` in shell aliases.

## Helper package

New package `internal/cli/watch` exports:

```go
type TickFunc func(ctx context.Context, w io.Writer) error

// RegisterFlags adds --watch/-w and --watch-interval to cmd.
func RegisterFlags(cmd *cobra.Command)

// Wrap returns a cobra RunE that either:
//   - calls fn once with cmd.OutOrStdout() (watch disabled), or
//   - runs fn on a ticker until ctx is cancelled (watch enabled).
func Wrap(fn TickFunc) func(cmd *cobra.Command, args []string) error
```

Command authors integrate like this:

```go
cmd := &cobra.Command{Use: "list", ...}
cmd.RunE = watch.Wrap(func(ctx context.Context, w io.Writer) error {
    // existing list-command body, writing to w instead of cmd.OutOrStdout()
})
watch.RegisterFlags(cmd)
```

The helper reads its own flags from the cobra command via `cmd.Flags().GetBool("watch")` and `GetDuration("watch-interval")`. Flag state lives on the cobra command, not in package-level globals — this keeps it easy to test commands in isolation.

## Behavior matrix

| Mode                         | Output format | Behavior |
|------------------------------|---------------|----------|
| Watch off                    | any           | Call `fn` once, return its error. No flag overhead. |
| Watch on, TTY, table         | table         | Each tick: render to buffer, clear screen, print header, print buffer. |
| Watch on, non-TTY, table     | table         | Each tick: print header, print body, blank line. No clear. |
| Watch on, any TTY, JSON      | json          | Each tick: print one compact JSON line (NDJSON). No header, no clear. |

TTY detection: `term.IsTerminal(int(os.Stdout.Fd()))` from `golang.org/x/term` (already an indirect dep via cobra; verify in implementation and add to go.mod if needed). The helper resolves the file descriptor from `cmd.OutOrStdout()` when it is an `*os.File`; otherwise treats it as non-TTY (this also makes tests deterministic — `bytes.Buffer` is non-TTY).

Output format is read from the existing `flags.GetOutputFormat()` so the helper integrates with the current `-o json|table` flag without duplicating state.

### Screen clear

Use ANSI `\x1b[H\x1b[2J` (cursor home + clear screen) followed by `\x1b[3J` (clear scrollback) on TTY. No external dependency on `tput` or termcap. Acceptable trade-off: assumes a vt100-compatible terminal, which Ghostty, iTerm2, Terminal.app, tmux, and most Linux terminals support. Document this in the helper's package comment.

### Header

TTY watch-mode only. Format:

```
Every 2s: hlctl system utilization                              14:23:01
```

- Left: `Every <interval>: <command path>`. Command path reconstructed by walking `cmd.CommandPath()`.
- Right: `time.Now().Format("15:04:05")`, right-aligned to terminal width (best-effort; fall back to left-aligned if width detection fails).
- Followed by one blank line, then the rendered output.

Non-TTY mode emits a simpler one-line header per snapshot so logs stay grep-able:

```
--- 2026-05-25T14:23:01+02:00  hlctl system utilization ---
```

### Errors per tick

If `fn` returns an error during a tick:
- Watch on, TTY: render `error: <message>` where the table would go. Keep ticking.
- Watch on, non-TTY: print `error: <message>` after the header. Keep ticking.
- Watch off: return the error as today (no change).

`fn` errors are formatted via `%v` — the existing `apiclient.ParseError` already produces a useful string. The watch loop never exits non-zero on transient tick errors; only signals end the loop.

### Cancellation and exit

The helper:
1. Creates a derived `context.Context` via `signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)`.
2. On TTY entry, writes the hide-cursor sequence (`\x1b[?25l`) and `defer`s the show-cursor sequence (`\x1b[?25h`) so Ctrl-C restores it.
3. Calls `fn` once immediately (first tick has no initial delay).
4. Starts `time.NewTicker(interval)` and loops: on each tick channel receive, call `fn`; on `ctx.Done()`, stop the ticker and return `nil`.

The loop returns `nil` on signal-driven exit; the process exits 0.

## Integration sites

Three commands rewrite their `RunE` to use `watch.Wrap`. Each existing body becomes a closure that writes to the passed `io.Writer` instead of `cmd.OutOrStdout()`. No other behavioral changes.

- `internal/cli/system/system.go` — `newUtilizationCmd`
- `internal/cli/docker/docker.go` — `newListCmd` (containers)
- `internal/cli/network/network.go` — `newListClientsCmd`

For each command:
1. Move the body of `RunE` into a `func(ctx context.Context, w io.Writer) error` closure.
2. Replace direct `cmd.OutOrStdout()` writes with the passed `w`.
3. Use `ctx` for the API call (`c.ListContainers(ctx, params)` instead of `context.Background()`).
4. Wrap with `watch.Wrap(...)` and call `watch.RegisterFlags(cmd)`.

Switching to `ctx` for the API call means a Ctrl-C during a slow request cancels it cleanly — small but worthwhile correctness improvement that comes for free.

## Testing

New tests in `internal/cli/watch/watch_test.go`:

- `TestWrap_watchDisabled_callsOnce` — bool stays false, helper calls `fn` exactly once with `cmd.OutOrStdout()`.
- `TestWrap_nonTTY_appendsSnapshots` — passing `*bytes.Buffer`. Drive ticks deterministically: `fn` increments a counter; after the third call it cancels the context. Assert: exactly three header lines, three rendered bodies, separator between.
- `TestWrap_nonTTY_tickError_continues` — same counter-driven harness; `fn` returns an error on the second call and writes normal output on calls 1 and 3. Assert: first and third snapshots present, `error: ...` line where the second body would be, function returns `nil`.
- `TestWrap_jsonMode_emitsNDJSON` — output format set to `json`. Counter-driven `fn` writes a distinct JSON doc per tick, cancels after the third. Assert: exactly three lines; each parses as JSON; no extra header lines.
- `TestWrap_intervalBelowMinimum` — `--watch-interval=10ms` returns error pre-loop.
- `TestRegisterFlags_addsBothFlags` — sanity check.

Existing command tests are unchanged. They construct the command with `nil`/stub clients and `cmd.Execute()`s once — that path now goes through `watch.Wrap` with `--watch=false`, which is the once-through branch.

TTY-mode behavior (screen clear, cursor hide, terminal-width header) is not covered by automated tests; it's verified manually. The helper's TTY-only paths are guarded by `if isTTY { ... }` so the test paths exercise the same code modulo the escape sequences.

## Out of scope

- No diff/highlight of changed rows between ticks. (Future: could add `--watch-diff` like `watch -d`.)
- No `--watch` on detail commands (`get <id>`) for now — the use cases listed are all lists. Easy to add later: the helper doesn't care.
- No streaming-event API integration. Each tick is a plain HTTP GET. If/when the Homelab API grows a server-sent-events endpoint, watch can be extended to prefer it.
- No structured "changed since last tick" output in JSON mode beyond plain NDJSON.

## Risks and mitigations

- **Terminal compatibility.** Reliance on ANSI escapes; documented assumption. Non-vt100 terminals would see garbled output. Mitigation: explicit comment in helper, and non-TTY fallback (which most CI/script users hit anyway) is escape-free.
- **Cursor left hidden on crash.** `defer` restores cursor on normal exit and on signal-driven cancellation. A panic mid-loop could leave it hidden — acceptable, recoverable with `reset` or a new shell.
- **Tight intervals.** Minimum `100ms` prevents accidentally hammering the API. A user setting `--watch-interval=100ms` against a 50ms-latency API will queue requests if any tick stalls; we use a ticker (drops missed ticks) not `time.After` chaining, so we don't pile up goroutines.
