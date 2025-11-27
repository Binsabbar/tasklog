# Contributing to Tasklog

Thank you for your interest in contributing to Tasklog! This guide will help you get started.

## Development Setup

1. **Prerequisites:**
   - Go 1.21 or higher
   - Git
   - Make (optional, but recommended)

2. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd tasklog
   ```

3. **Install dependencies:**
   ```bash
   go mod download
   ```

4. **Set up your development configuration:**
   ```bash
   make setup
   # Edit ~/.tasklog/config.yaml with your credentials
   ```

## Project Structure

```
tasklog/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command and CLI setup
│   ├── init.go            # Initialize config command
│   ├── log.go             # Time logging command
│   ├── sync.go            # Sync unsynced entries
│   └── summary.go         # Display summary
├── internal/
│   ├── config/            # Configuration management
│   │   └── config.go      # Config loading and validation
│   ├── jira/              # Jira API integration
│   │   └── client.go      # Jira REST API client
│   ├── tempo/             # Tempo API integration
│   │   └── client.go      # Tempo REST API client
│   ├── storage/           # Local SQLite cache
│   │   └── storage.go     # Database operations
│   ├── timeparse/         # Time parsing utilities
│   │   ├── timeparse.go   # Parse time formats
│   │   └── timeparse_test.go
│   └── ui/                # Interactive UI components
│       └── prompts.go     # Survey-based prompts
├── main.go                # Application entry point
├── config.example.yaml    # Example configuration
├── Makefile              # Build automation
├── README.md             # User documentation
└── CONTRIBUTING.md       # This file
```

## Making Changes

1. **Create a new branch:**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes:**
   - Write clean, idiomatic Go code
   - Follow existing code style and patterns
   - Add tests for new functionality
   - Update documentation as needed

3. **Run tests:**
   ```bash
   make test
   ```

4. **Build and test locally:**
   ```bash
   make build
   ./tasklog --help
   ```

## Code Style

- Follow standard Go conventions
- Use `gofmt` to format code (run `make fmt`)
- Keep functions focused and concise
- Add comments for exported functions and types
- Use meaningful variable and function names

## Testing

- Write unit tests for new functionality
- Ensure all tests pass before submitting PR
- Aim for good test coverage of critical paths

### Running Tests

```bash
# Run all tests (silent mode)
make go-test

# Run tests with verbose output
make go-test-verbose

# Run tests with coverage and race detector
make go-test-coverage
```

**Test Coverage**: Core business logic achieves **83.9%** average coverage (config: 83%, storage: 81.8%, timeparse: 89.2%). See [TESTING.md](TESTING.md) for details.

### Code Quality

```bash
# Format code
make go-fmt

# Check formatting
make go-fmt-check

# Run linter
make go-lint

# Check for security vulnerabilities
make go-vulncheck
```

### Build Options

```bash
# Development build
make go-build

# Build Docker image
make docker-build VERSION=v1.0.0

# Build and push Docker image
make docker-build-and-push VERSION=v1.0.0

# Create a release with GoReleaser (requires git tag)
git tag v1.0.0
make release

# Build snapshot release locally (no tag required)
make release-snapshot
```

## Changelog Management

Tasklog uses [changie](https://changie.dev/) for managing changelog entries. When making changes:

```bash
# Create a new changelog entry
changie new

# Follow the prompts to select the type and describe your change
```

For detailed release process, see [RELEASE.md](RELEASE.md).

## Commit Messages

Write clear, descriptive commit messages:

```
feat: add support for custom time rounding intervals
fix: resolve issue with Tempo API authentication
docs: update configuration examples
test: add tests for time parsing edge cases
```

## Pull Request Process

1. Update the README.md with details of significant changes
2. Ensure all tests pass
3. Update documentation if you're changing functionality
4. Create a pull request with a clear description of changes

## Feature Ideas

Here are some ideas for future enhancements:

- **Timer functionality:** Start/stop timers for tasks
- **Multiple Jira projects:** Support for different project configurations
- **Reports:** Generate weekly/monthly time reports
- **Export:** Export data to CSV or other formats
- **Team features:** Track team time across multiple users
- **Custom fields:** Support for custom Jira fields
- **Offline mode:** Better handling of offline scenarios
- **Shell completions:** Auto-completion for bash/zsh
- **Web UI:** Optional web interface for viewing logs

## Getting Help

- Check existing issues for similar problems or questions
- Create a new issue for bugs or feature requests
- Join discussions in existing issues or PRs

## Code of Conduct

- Be respectful and constructive
- Help others learn and grow
- Focus on what's best for the project and community

## License

By contributing to Tasklog, you agree that your contributions will be licensed under the MIT License.
