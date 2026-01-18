# AGENTS.md

This file contains guidelines and commands for agentic coding agents working on the tasklog repository.

## Build/Test/Lint Commands

### Core Commands
- `make go-build` - Build the binary to bin/tasklog
- `make go-test` - Run all tests (silent mode)
- `make go-test-verbose` - Run all tests with verbose output
- `make go-test-coverage` - Run all tests with race detector and coverage
- `make go-lint` - Run golangci-lint checks (production code only)
- `make go-vulncheck` - Run govulncheck for security vulnerabilities
- `make go-fmt` - Format code with gofmt
- `make go-fmt-check` - Check if code is properly formatted

### Running Single Tests
- `go test ./cmd -run TestSpecificFunction` - Run specific test function
- `go test ./internal/jira -v` - Run tests for specific package with verbose output
- `go test -run TestCreateNewConfig ./cmd` - Run specific test in specific package

### Test Environment
- Tests run silently by default (set TEST_SILENT=1)
- Use `go test -v` for verbose output during development
- Test files use `*_test.go` naming convention
- Use `t.TempDir()` for temporary files in tests
- Use `httptest.NewServer` for HTTP client testing

### CI/CD Pipeline
- GitHub Actions run on push to non-main branches
- Pipeline includes: vet, test, golangci-lint, govulncheck
- Go version: 1.25
- golangci-lint version: 2.5.0

## Code Style Guidelines

### Import Organization
- Use `goimports` for automatic import formatting
- Group imports: standard library, third-party, internal packages
- Internal packages use the `tasklog/` prefix (e.g., `tasklog/internal/config`)
- Avoid blank imports except for drivers (e.g., `database/sql` drivers)

### Formatting
- Use `gofmt` with `-s` flag for code formatting
- Maximum line length: 120 characters (configured in golangci-lint)
- Use `goimports` for import organization
- No trailing whitespace

### Naming Conventions
- **Packages**: lowercase, single word when possible (e.g., `config`, `jira`, `tempo`)
- **Constants**: `CamelCase` for exported, `camelCase` for unexported
- **Variables**: `camelCase` (e.g., `taskKey`, `timeSpent`)
- **Functions**: `CamelCase` for exported, `camelCase` for unexported
- **Structs**: `CamelCase` for exported, `camelCase` for unexported
- **Interfaces**: `CamelCase` ending in `er` (e.g., `Client`, `Writer`)
- **Error variables**: `ErrXxx` (e.g., `ErrConfigNotFound`)

### Type Definitions
- Use meaningful type names
- Export fields with `CamelCase` and YAML tags for config structs
- Use validation tags where appropriate (`validate:"required"`)
- Include comments for exported types and fields

### Error Handling
- Always check for errors
- Use `zerolog` for structured logging
- Log errors with context: `log.Error().Err(err).Msg("description")`
- Wrap errors when adding context: `fmt.Errorf("operation failed: %w", err)`
- Return errors from functions, don't panic unless unrecoverable

### Function Design
- Keep functions focused and small
- Use descriptive names
- Limit parameters (consider structs for many parameters)
- Use pointer receivers for methods that modify the receiver
- Use value receivers for methods that don't modify the receiver

### Struct Design
- Use YAML tags for configuration structs
- Use validation tags for required fields
- Include JSON tags for API structs
- Use pointer types for optional fields in structs

### Testing Patterns
- Table-driven tests for multiple scenarios
- Use `t.Helper()` in test helper functions
- Use `t.Run()` for subtests
- Mock external dependencies with interfaces
- Use `httptest.NewServer` for HTTP client testing
- Test both success and error cases

### Logging
- Use `zerolog` for structured logging
- Log levels: Debug, Info, Error, Fatal
- Use structured fields: `log.Info().Str("task", key).Msg("message")`
- Avoid `fmt.Printf` for logging (use zerolog instead)
- Use `log.Fatal()` for unrecoverable errors

### Configuration
- Use YAML configuration files
- Support environment variable overrides
- Use `github.com/go-playground/validator/v10` for validation
- Include version field for config migrations
- Store config in `~/.tasklog/config.yaml` by default

### HTTP Client Patterns
- Create client structs with configuration
- Use context with timeouts for HTTP requests
- Handle HTTP status codes appropriately
- Use structured logging for HTTP operations
- Test with `httptest.NewServer`

### CLI Design (Cobra)
- Use `cobra.Command` for CLI commands
- Add persistent flags for common options
- Use `RunE` for commands that can return errors
- Include examples in command help
- Use `cmd/root.go` for shared configuration

### Database Patterns
- Use `modernc.org/sqlite` for SQLite database
- Handle database errors appropriately
- Use transactions for multiple operations
- Close database connections properly
- Use prepared statements for performance

### API Integration
- Create client packages for external APIs
- Use structs for API request/response models
- Handle API errors gracefully
- Use appropriate HTTP methods
- Include API version information in requests

### Project Structure
```
tasklog/
├── cmd/           # CLI commands
├── internal/      # Internal packages
│   ├── config/    # Configuration management
│   ├── jira/      # Jira API client
│   ├── tempo/     # Tempo API client
│   ├── slack/     # Slack API client
│   ├── storage/   # Database operations
│   ├── ui/        # User interface components
│   └── output/    # Output formatting
├── main.go        # Application entry point
└── Makefile       # Build commands
```

### Linting Rules
- golangci-lint configuration in `.golangci.yml`
- Disabled rules for CLI-specific patterns (forbidigo, gochecknoglobals)
- Enabled rules for security (gosec), error handling (errcheck), and style
- Maximum cyclomatic complexity: 30
- Maximum function lines: 100

### Security
- Never commit API tokens or credentials
- Use environment variables for sensitive configuration
- Run `govulncheck` to check for vulnerabilities
- Validate all external inputs
- Use HTTPS for all API calls

### Performance
- Use context with timeouts for HTTP requests
- Close HTTP clients properly
- Use connection pooling for database operations
- Avoid unnecessary allocations in hot paths
- Use buffered I/O for file operations

### Documentation
- Include package comments for all packages
- Document exported functions, types, and methods
- Include examples in command help text
- Use clear, descriptive variable names
- Add inline comments for complex logic

## Development Workflow

1. Before making changes:
   - Run `make go-test` to ensure tests pass
   - Run `make go-lint` to check code style
   - Run `make go-fmt-check` to verify formatting

2. After making changes:
   - Run `make go-test` to verify tests still pass
   - Run `make go-lint` to fix any style issues
   - Run `make go-fmt` to format code
   - Add tests for new functionality
   - Update documentation as needed

3. Before committing:
   - Ensure all tests pass: `make go-test-coverage`
   - Ensure linting passes: `make go-lint`
   - Ensure code is formatted: `make go-fmt-check`
   - Run security check: `make go-vulncheck`

## Environment Variables

- `TASKLOG_CONFIG` - Path to config file (default: `~/.tasklog/config.yaml`)
- `TASKLOG_LOG_LEVEL` - Set to `debug` for verbose logging (default: `info`)
- `TEST_SILENT` - Set to `1` for silent test output (default in Makefile)
- `NO_COLOR` - Set to disable colored output

## Testing Requirements

- Aim for high test coverage on business logic
- Test error paths and edge cases
- Use table-driven tests for multiple scenarios
- Mock external dependencies (HTTP clients, databases)
- Use `t.TempDir()` for temporary test files
- Test both success and failure cases
- Include integration tests for external APIs when possible