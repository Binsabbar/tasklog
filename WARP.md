# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Common Commands

### Build
- Development build (binary to `bin/tasklog`):
  - `make go-build`
- Direct Go build (bypassing Makefile):
  - `go build -o bin/tasklog ./main.go`

### Tests
- Run all tests (silent mode, same as CI):
  - `make go-test`
- Run all tests with verbose output:
  - `make go-test-verbose`
- Run tests with race detector and coverage (used for coverage targets):
  - `make go-test-coverage`
- Run tests for a specific package (examples):
  - `go test -v ./internal/timeparse/`
  - `go test -v ./internal/storage/`
  - `go test -v ./internal/config/`

See `TESTING.md` for current coverage targets and philosophy.

### Linting, Formatting, and Security
- Run golangci-lint with project configuration (`.golangci.yml`), same as CI:
  - `make go-lint`
- Format Go code with `gofmt`:
  - `make go-fmt`
- Check formatting only (non-empty output means files need formatting):
  - `make go-fmt-check`
- Run Go vulnerability scan (govulncheck):
  - `make go-vulncheck`

### Docker and Releases
- Build Docker image (tag defaults to `latest`, override with `VERSION=...`):
  - `make docker-build VERSION=v1.0.0`
- Build and push Docker image to GHCR (requires auth and `GITHUB_TOKEN`):
  - `make docker-build-and-push VERSION=v1.0.0`
- Build a local snapshot release with GoReleaser (no tag required, used by CI snapshot workflow):
  - `make release-snapshot`
- Full release with GoReleaser (expects a git tag and is normally driven by CI):
  - `make release`

### Running the CLI Locally
- Build then run via local binary:
  - `make go-build`
  - `./bin/tasklog --help`
- Common commands during manual testing (assumes `~/.tasklog/config.yaml` exists):
  - `tasklog init` – generate example config at `~/.tasklog/config.yaml`
  - `tasklog log` – interactive time logging
  - `tasklog log daily` – example shortcut-based logging (depends on config shortcuts)
  - `tasklog summary` – requires Tempo enabled in config
  - `tasklog sync` – retry unsynced entries
  - `tasklog break lunch` – Slack-based break handling if Slack is configured

Refer to `README.md` for full end-user usage and configuration details.

## High-Level Architecture

### Overall Structure
The project is a Go CLI application using Cobra for command routing, Zerolog for logging, SQLite for local persistence, and custom clients for Jira, Tempo, and Slack.

Top-level layout (see `CONTRIBUTING.md` for detailed tree):
- `main.go` – entry point; wires logging, then calls `cmd.Execute()`.
- `cmd/` – Cobra commands and CLI orchestration.
- `internal/` – core business logic and integration layers (config, storage, APIs, UI utilities).
- `.github/` – CI workflows and release automation (described in `.github/WORKFLOWS.md`).

### CLI Layer (`cmd/`)
The `cmd` package owns the CLI surface and orchestrates cross-cutting operations.

Key files:
- `cmd/root.go`
  - Defines the root Cobra command (`tasklog`) with description and version string.
  - Registers a global `OnInitialize` hook that ensures the config directory exists.
  - Exposes `Execute()` used by `main.go` and a `version` subcommand that logs structured version info via Zerolog.
- `cmd/init.go`
  - Implements `tasklog init`.
  - Ensures `~/.tasklog/` exists and writes `config.yaml` based on `config.example.yaml` or an inline default template.
- `cmd/log.go`
  - Implements `tasklog log [shortcut-name]`.
  - Primary orchestration command that ties together config, Jira, Tempo, storage, time parsing, and UI.
  - Responsibilities:
    - Interpret optional positional `shortcut-name` plus flags (`--task`, `--time`, `--label`).
    - Load config via `checkConfig()` (root helper) and validate.
    - Instantiate `jira.Client`, `tempo.Client`, and `storage.Storage`.
    - Resolve issue via direct key or interactive flows (using `internal/ui`).
    - Parse and normalize duration via `internal/timeparse`.
    - Enforce label rules via `config.Config.IsLabelAllowed` and possibly interactive selection.
    - Confirm log details, persist to SQLite (`internal/storage`), then call Jira API.
    - Derive Tempo sync status from Jira + config (`Tempo.Enabled`), update local record, and finally render an end-of-command summary via `showTodaySummary`.
- `cmd/summary.go`
  - Implements `tasklog summary`.
  - Requires Tempo to be enabled; loads config, initializes Jira + Tempo clients and storage, and delegates to `showTodaySummary` from `log.go`.
- `cmd/sync.go`
  - Implements `tasklog sync`.
  - Loads config, opens storage, fetches unsynced entries via `storage.Storage.GetUnsyncedEntries`, then retries Jira worklog creation and updates sync flags.
- `cmd/break.go`
  - Implements `tasklog break [break-name]`.
  - Reads break definitions and Slack credentials from config, then:
    - Computes return time based on break duration.
    - Updates Slack user status (with emoji and expiration buffer) via `internal/slack`.
    - Posts a formatted message to the configured Slack channel.
  - Handles fallbacks when emojis are invalid or Slack is partially configured.

Pattern: the `cmd` layer should remain thin, delegating real logic to `internal/*` packages and keeping side-effects/coordinating flows at the edges.

### Configuration Layer (`internal/config`)
- `internal/config/config.go`
  - Defines the main `Config` struct with nested configuration types for Jira, Tempo, labels, shortcuts, database, Slack, and breaks.
  - `Load()`:
    - Determines config path via env var `TASKLOG_CONFIG` or `~/.tasklog/config.yaml`.
    - Reads YAML, unmarshals into `Config`, applies defaults (notably SQLite path), and validates required fields.
    - Produces detailed error messages for missing or invalid configuration.
  - Helper methods:
    - `Validate()` – field-level checks (required Jira fields, Tempo token when enabled).
    - `GetShortcut(name)` – lookup by shortcut name.
    - `IsLabelAllowed(label)` – implements label whitelisting; if no labels configured, all are allowed.
    - `GetBreak(name)` – lookup for break definitions.
  - `EnsureConfigDir()` – creates `~/.tasklog` if missing; used at startup and by `init` command.

This layer centralizes configuration semantics and should be the only place that knows config file layout and default resolution.

### Persistence Layer (`internal/storage`)
- `internal/storage/storage.go`
  - Wraps SQLite access behind a `Storage` struct.
  - Manages schema creation on startup (`initSchema`) including indices for common queries.
  - `TimeEntry` struct represents the local cache entity, including sync flags and remote worklog IDs.
  - Core methods (non-exhaustive):
    - `NewStorage(dbPath)` – open DB and ensure schema.
    - `AddTimeEntry(*TimeEntry)` – insert new entry and assign `ID`.
    - `UpdateTimeEntry(*TimeEntry)` – update sync flags and worklog IDs.
    - `GetTodayEntries()` – filter by `DATE(started)` and order by `started` descending.
    - `GetUnsyncedEntries()` – retrieve entries not fully synced to Jira/Tempo (used by `sync` command).

Local SQLite is the source of truth for what the CLI attempted to log, while Tempo is treated as the canonical source of truth for actual logged time (see summary display logic).

### Time Parsing Utilities (`internal/timeparse`)
- `internal/timeparse/timeparse.go`
  - Provides user-facing duration handling and normalization:
    - `Parse(string) (int, error)` – accepts a wide range of textual inputs (e.g., `2h 30m`, `2.5h`, `150m`), normalizes them, delegates to `go-str2duration`, and enforces:
      - Positive durations only.
      - Rounding to nearest 5 minutes (core business rule).
    - `Format(int) string` – human-readable formatting (`2h 30m`, `45m`, etc.).
    - `Validate(string) error` – wrapper around `Parse` for simple validation use cases.

This package is pure and heavily tested; new time-related rules should generally live here rather than in command code.

### Integration Clients (`internal/jira`, `internal/tempo`, `internal/slack`)
- `internal/jira/client.go`
  - Encapsulates Jira Cloud REST API calls.
  - Responsibilities include:
    - Fetching in-progress issues filtered by project and status list.
    - Searching issues by user-provided queries.
    - Fetching full issue details.
    - Getting current user details (used to filter Tempo worklogs).
    - Creating worklogs (`AddWorklog`) used by both `log` and `sync` flows.
- `internal/tempo/client.go`
  - Encapsulates Tempo API access.
  - Core usage pattern in this codebase is read-only aggregation for summary:
    - `GetTodayWorklogs(accountID)` – used by `showTodaySummary` to compute and display daily totals.
- `internal/slack/client.go`
  - Thin wrapper around Slack Web API.
  - Provides two primary methods:
    - `SetStatus(text, emoji, expirationMinutes)` – user presence updates.
    - `PostMessage(message)` – send break notifications to a channel.

These packages are designed such that higher-level commands build clients once using config, then pass them into orchestration functions.

### UI / Interaction Layer (`internal/ui`)
- `internal/ui/prompts.go`
  - Houses all interactive TUI flows, using the `survey` library.
  - Exposes helpers used by `cmd/log.go` and related flows:
    - `SelectTask(...)` – choose from in-progress issues or trigger free-text search.
    - `SelectFromSearchResults(...)` – refine choice from Jira search results.
    - `PromptTimeSpent()` – interactive duration input.
    - `SelectLabel([]string)` – label selection respecting configured allowed labels.
    - `PromptComment()` – optional worklog comment.
    - `Confirm(prompt string)` – generic confirmation used just before persisting/logging.

Future work that changes user interaction should prefer to add functions here rather than directly calling `survey` from commands.

### Summary and Reconciliation Logic
- Implemented in `showTodaySummary` (in `cmd/log.go`) and reused by `summary` command.
- Flow:
  - Determine current Jira user (via `jira.Client`).
  - Fetch Tempo worklogs for today as the canonical record.
  - Fetch local time entries from SQLite for today.
  - Compute separate totals and show:
    - Detailed Tempo worklogs (times, durations, descriptions, issue keys).
    - Local cache entries with sync status indicators (synced, Jira only, Tempo only, not synced).
  - Compare totals and print high-level reconciliation message (matched, more in Tempo, or more locally).

This function is a good reference entrypoint for any future reporting or reconciliation features.

### CI/CD and Release Automation
Key documents for automation and release behavior:
- `.github/WORKFLOWS.md` – authoritative description of GitHub Actions workflows, triggers, and how they map to Makefile targets.
- `RELEASE.md` – release process and how `changie`, GoReleaser, and GitHub Actions work together.
- `CHANGELOG.md` + `.changes/` – changelog entries managed by `changie`.

Important points for agents:
- CI uses `make go-test`, `make go-lint`, and `make release-snapshot` for tests, linting, and snapshots.
- Production releases are intended to be driven by GitHub Actions and GoReleaser; avoid hand-rolling release artifacts or tags unless explicitly requested.

## Agent-Focused Notes

- Prefer editing or adding commands under `cmd/` and delegating business logic into `internal/*` packages.
- When adding new features that touch external systems (Jira, Tempo, Slack), extend or reuse the existing `internal/{service}` client rather than calling HTTP directly from commands.
- Keep configuration schema changes localized to `internal/config/config.go` and update `config.example.yaml`, `README.md`, and any relevant interactive flows (`init` and `ui` prompts) in tandem.
- For behavior that affects time parsing, rounding, or representation, update `internal/timeparse` first and then adjust callers.
