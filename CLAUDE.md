# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Tasklog is a CLI tool for tracking time on Jira tasks with Tempo integration. It provides:
- Interactive task selection and time logging
- Local SQLite caching with sync recovery
- Jira Cloud API v3 and Tempo API v4 integration
- Slack integration for break notifications
- Predefined shortcuts for automation

**Important**: This project was built entirely with GitHub Copilot. See README.md for context.

## Build Commands

```bash
# Build
make go-build                  # Creates bin/tasklog

# Testing
make go-test                   # Run tests (silent mode)
make go-test-verbose           # Run tests with output
make go-test-coverage          # Run tests with coverage & race detector
go test ./internal/config -v   # Run specific package tests
go test -run TestFunctionName ./cmd  # Run specific test

# Code Quality
make go-fmt                    # Format code with gofmt
make go-fmt-check              # Verify formatting
make go-lint                   # Run golangci-lint (production code only)
make go-vulncheck              # Security vulnerability check

# Release
make release-snapshot          # Build snapshot locally (no tag)
git tag v1.0.0 && make release # Create tagged release with GoReleaser
make docker-build VERSION=v1.0.0  # Build Docker image
```

## Architecture Overview

### Package Structure

```
cmd/                  # CLI commands (Cobra framework)
├── root.go          # Root command, version info, update checks
├── log.go           # Main time logging with shortcuts
├── summary.go       # Display Tempo worklogs
├── sync.go          # Retry failed syncs
├── break.go         # Break notifications
├── config.go        # Config management (show/compare/example)
├── init.go          # Initialize config
├── upgrade.go       # Update installation (hidden if not official build)
└── version.go       # Version info

internal/
├── config/          # Config loading, validation, comparison
│   ├── config.go    # Load/validate YAML with go-playground/validator
│   ├── template.go  # Config template generation
│   └── compare.go   # Compare user config with example
├── jira/            # Jira Cloud REST API v3 client
├── tempo/           # Tempo API v4 client
├── slack/           # Slack API for status/messages
├── storage/         # SQLite database (modernc.org/sqlite)
│   └── storage.go   # TimeEntry CRUD, sync tracking
├── timeparse/       # Time format parsing & rounding
│   ├── timeparse.go # Parse "2h 30m", "2.5h", "150m"
│   └── datetime.go  # Parse --at flag (yesterday, 2pm, etc.)
├── ui/              # Interactive prompts (AlecAivazis/survey)
├── updater/         # GitHub release checking with cache
└── prerelease/      # Pre-release config validation
```

### Data Flow

1. **Time Logging (`log` command)**:
   - User selects task (in-progress list, search, or manual entry)
   - User provides time, label, optional comment
   - Entry saved to SQLite with `synced_to_jira=false`
   - Attempt sync to Jira worklog API
   - On success: update `synced_to_jira=true`, store `jira_worklog_id`
   - Show today's summary from Tempo

2. **Storage Layer**:
   - SQLite stores all entries locally (source of record for unsent data)
   - Tracks sync status per entry (`synced_to_jira`, `jira_worklog_id`)
   - Enables offline work and retry via `sync` command

3. **API Integration**:
   - **Jira**: POST to `/rest/api/3/issue/{issueKey}/worklog` (creates Tempo entry automatically)
   - **Tempo**: GET worklog summaries (read-only, source of truth for display)
   - **Slack** (optional): Update status + post channel message

### Key Design Decisions

1. **Time Rounding**: All time inputs rounded to nearest 5 minutes (`timeparse` package)

2. **Shortcuts**: Config-defined shortcuts become subcommands (e.g., `tasklog log daily`)
   - Defined in `config.yaml` under `jira.shortcuts`
   - Can specify task, time (optional), label
   - Perfect for cronjobs

3. **Sync Model**:
   - Logs ONLY to Jira (which creates Tempo entries automatically)
   - Tempo used ONLY for reading summary (source of truth)
   - Local SQLite cache enables retry on failure

4. **Version Detection**:
   - `builtBy` ldflags distinguishes official (goreleaser) vs dev builds
   - `IsOfficialBuild()`: enables update checks
   - `IsPreReleaseBuild()`: enables pre-release config validation

5. **Configuration Versioning**:
   - `config.version` field supports future migrations
   - `config compare` helps users discover new fields after upgrades

## Testing Strategy

**Coverage Goals**: >80% on business logic (config, storage, timeparse)

**What's Tested**:
- Config validation, shortcut lookup, label filtering (83% coverage)
- Storage CRUD, sync tracking, time calculations (81.8% coverage)
- Time parsing, rounding, validation (89.2% coverage)

**What's NOT Tested** (by design):
- HTTP API calls (jira, tempo clients) - structure tests only
- Interactive UI prompts (survey library)
- CLI commands (integration testing not included)

**Test Patterns**:
- Table-driven tests with `t.Run()`
- `httptest.NewServer` for HTTP mocking
- `t.TempDir()` for test files
- Silent mode by default (`TEST_SILENT=1`)

## Development Workflow

1. **Before Changes**:
   ```bash
   make go-test && make go-lint && make go-fmt-check
   ```

2. **After Changes**:
   ```bash
   make go-fmt              # Format code
   make go-test-coverage    # Run tests with race detector
   make go-lint             # Check style
   make go-vulncheck        # Security check
   ```

3. **Changelog Management**:
   ```bash
   changie new              # Create changelog entry for changes
   ```
   See RELEASE.md for full release process.

## Important Conventions

### Imports
- Group: stdlib, third-party, internal
- Internal imports use `tasklog/` prefix (e.g., `tasklog/internal/config`)

### Logging
- Use `zerolog` for all logging
- `log.Debug()` for verbose/trace info
- `log.Info()` for important events
- `log.Error()` for errors
- Structured fields: `log.Info().Str("task", key).Msg("message")`
- Set `TASKLOG_LOG_LEVEL=debug` for verbose output

### Error Handling
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Log errors before returning: `log.Error().Err(err).Msg("description")`
- Never panic unless unrecoverable

### Configuration
- Config at `~/.tasklog/config.yaml` (override via `TASKLOG_CONFIG`)
- Validation via `github.com/go-playground/validator/v10`
- All Jira fields required, Tempo/Slack/Labels optional
- Use `config.Load()` to load and validate

### CLI Design (Cobra)
- Commands in `cmd/` package
- Use `RunE` for error returns
- Persistent pre-run hook checks updates (official builds only)
- Hidden commands: `upgrade` (hidden if not official build)

### Database
- SQLite via `modernc.org/sqlite` (pure Go, no CGo)
- Default path: `~/.tasklog/tasklog.db`
- Schema initialization on first open
- Always `defer store.Close()`

## Common Patterns

### Adding a New Command
1. Create `cmd/newcommand.go`
2. Define `cobra.Command` with `Use`, `Short`, `Long`, `RunE`
3. Add to `rootCmd` in `init()`
4. Load config with `checkConfig()`
5. Initialize clients/storage as needed

### Adding Config Field
1. Add field to struct in `internal/config/config.go`
2. Add validation tag if required
3. Update `internal/config/template.go` with example
4. Test with `make go-test`
5. Users run `tasklog config compare` to discover new field

### Adding API Client Method
1. Define request/response structs
2. Create method on client struct
3. Use context with timeout
4. Log errors with structured fields
5. Add structure test (no mocking required for basic coverage)

## Environment Variables

- `TASKLOG_CONFIG` - Config file path (default: `~/.tasklog/config.yaml`)
- `TASKLOG_LOG_LEVEL` - Set `debug` for verbose logging (default: `info`)
- `TEST_SILENT` - Set `1` for silent test output
- `NO_COLOR` - Disable colored output

## Known Issues / Pre-release Warnings

Pre-release builds (`alpha`, `beta`, `rc`) check for deprecated config patterns:
- Validates against known breaking changes
- Warnings printed to stderr before command execution
- See `internal/prerelease/config_validator.go`

## References

- Full user docs: README.md
- Contributing guide: CONTRIBUTING.md
- Testing details: TESTING.md
- Release process: RELEASE.md
- Agent coding guide: AGENTS.md (comprehensive coding standards)
