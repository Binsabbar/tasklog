# Tasklog

An interactive CLI tool for tracking time on Jira tasks with seamless integration to Jira Cloud API and Tempo.

## Features

- 🎯 **Interactive Task Selection**: List your in-progress tasks or search for any task
- 🔍 **Project Filtering**: Optionally filter tasks to a specific Jira project
- ⏱️ **Flexible Time Entry**: Support for multiple time formats (2h 30m, 2.5h, 150m)
- 🏷️ **Label Management**: Configure and use labels for categorizing work
- ⚡ **Shortcuts**: Define shortcuts for repetitive tasks (perfect for cronjobs)
- ⏸️ **Break Management**: Register breaks with automatic Slack status updates and channel notifications
- 💬 **Slack Integration**: Update status and post messages when taking breaks (optional)
- 💾 **Local Cache**: SQLite database keeps track of all entries locally
- 🔄 **Dual Sync**: Automatically logs time to both Jira and Tempo
- 📊 **Daily Summary**: View your logged time for the day
- 🔁 **Sync Recovery**: Retry failed syncs with the sync command

## Installation

### Prerequisites

- Go 1.21 or higher
- Access to Jira Cloud with API token
- Tempo API token
- (Optional) Slack bot token for break notifications

### Build from Source

```bash
git clone <repository-url>
cd timetracking
go build -o tasklog
sudo mv tasklog /usr/local/bin/  # Optional: Install globally
```

Or use the Makefile:

```bash
make build    # Build the binary
make install  # Build and install to /usr/local/bin
make test     # Run tests
```

## Quick Start

1. **Build and install:**
   ```bash
   make install
   ```

2. **Initialize configuration:**
   ```bash
   tasklog init
   ```

3. **Edit config file with your credentials:**
   ```bash
   nano ~/.tasklog/config.yaml
   ```
   - Add your Jira URL, username, and API token
   - Add your Tempo API token
   - Optionally configure labels and shortcuts

4. **Start logging time:**
   ```bash
   tasklog log
   ```

That's it! The tool will guide you through the rest.

## Configuration

### Quick Setup

Initialize tasklog with a template configuration:

```bash
tasklog init
```

This creates `~/.tasklog/config.yaml` with an example configuration. Edit it with your credentials.

### Manual Setup

Create a configuration file at `~/.tasklog/config.yaml`:

```yaml
jira:
  url: "https://your-domain.atlassian.net"
  username: "your-email@example.com"
  api_token: "your-jira-api-token"
  project_key: "PROJ"  # Project key to filter tasks (required)

tempo:
  api_token: "your-tempo-api-token"  # Required

# Optional: Filter labels that can be used for time logging
# If not specified, labels will need to be entered manually
labels:
  allowed_labels:
    - "development"
    - "code-review"
    - "meeting"
    - "testing"
    - "documentation"
    - "bug-fix"

# Optional: Define shortcuts for quick time logging
# Shortcuts allow you to quickly log time without interactive prompts
shortcuts:
  - name: "daily"
    task: "PROJ-123"
    time: "30m"
    label: "meeting"
  
  - name: "standup"
    task: "PROJ-123"
    time: "15m"
    label: "meeting"
  
  - name: "code-review"
    task: "PROJ-456"
    # time not specified - will prompt user
    label: "code-review"

# Optional: Database path (defaults to ~/.tasklog/tasklog.db)
database:
  path: ""

# Optional: Slack integration for break notifications
slack:
  bot_token: "xoxb-your-slack-bot-token"
  channel_id: "C1234567890"  # Channel ID for break messages

# Optional: Define break types for quick break registration
breaks:
  - name: "lunch"
    duration: 60
    emoji: ":fork_and_knife:"
  
  - name: "prayer"
    duration: 15
    emoji: ":pray:"
  
  - name: "coffee"
    duration: 10
    emoji: ":coffee:"
```

### Getting API Tokens

#### Jira Configuration

**Finding Your Jira URL:**
- Your Jira Cloud URL is typically: `https://your-domain.atlassian.net`
- Example: `https://mycompany.atlassian.net`

**Getting Jira API Token:**
1. Go to https://id.atlassian.com/manage-profile/security/api-tokens
2. Click "Create API token"
3. Give it a descriptive name (e.g., "Tasklog CLI")
4. Copy the token immediately (you won't be able to see it again)

**Your Jira Username:**
- Use the email address associated with your Jira account
- Example: `your-email@example.com`

**Project Key:**
- This filters tasks to a specific Jira project
- Found in task IDs (e.g., `PROJ-123` → project key is `PROJ`)
- Or check your Jira project settings

#### Tempo Configuration

**Getting Tempo API Token:**
1. In Jira, go to **Tempo** in the top navigation
2. Click **Settings** (gear icon)
3. Select **API Integration** from the left sidebar
4. Click **New Token**
5. Give it a name (e.g., "Tasklog CLI")
6. Copy the generated token

**Note:** Tempo must be installed in your Jira workspace for time tracking.
3. Copy the token

**Slack Bot Token (Optional for break notifications):**
1. Go to https://api.slack.com/apps
2. Create a new app or select existing
3. Go to "OAuth & Permissions"
4. Add bot token scopes: `chat:write`, `users.profile:write`
5. Install app to workspace
6. Copy the "Bot User OAuth Token" (starts with `xoxb-`)
7. Get channel ID by right-clicking channel > View channel details

### Slack Setup (Optional)

Slack integration is required only if you want to use the `tasklog break` command to automatically update your Slack status and post break notifications to a channel.

#### Creating a Slack App

1. **Visit Slack API Dashboard**
   - Go to https://api.slack.com/apps
   - Click "Create New App"
   - Choose "From scratch"
   - Give it a name (e.g., "Tasklog Break Notifications")
   - Select your workspace

2. **Configure OAuth & Permissions**
   - In the left sidebar, click "OAuth & Permissions"
   - Scroll down to "Scopes" > "Bot Token Scopes"
   - Add the following scopes:
     - `chat:write` - Post messages to channels
     - `users.profile:write` - Update your own status

3. **Install App to Workspace**
   - Scroll to the top of the "OAuth & Permissions" page
   - Click "Install to Workspace"
   - Review permissions and click "Allow"

4. **Get Your Bot Token**
   - After installation, you'll see "Bot User OAuth Token"
   - Copy this token (starts with `xoxb-`)
   - Add it to your `~/.tasklog/config.yaml` under `slack.bot_token`

5. **Get Channel ID**
   - In Slack, right-click the channel where you want break notifications
   - Select "View channel details"
   - Scroll down to find the Channel ID (e.g., `C1234567890`)
   - Add it to your `~/.tasklog/config.yaml` under `slack.channel_id`

6. **Invite Bot to Channel**
   - In the Slack channel, type: `/invite @YourBotName`
   - This allows the bot to post messages

#### Testing Slack Integration

After configuration, test it with:

```bash
tasklog break coffee
```

You should see:
- Your Slack status updated with coffee emoji
- A message posted in the configured channel
- Status automatically cleared after the break duration

## Usage

### Interactive Mode

Log time interactively (recommended for first-time use):

```bash
tasklog log
```

This will:
1. Show your in-progress Jira tasks
2. Let you select a task (or search/enter manually)
3. Prompt for time spent
4. Prompt for a label
5. Ask for an optional comment
6. Confirm before logging
7. Log to both Jira and Tempo
8. Show today's summary

### Using Shortcuts

Log time quickly using predefined shortcuts:

```bash
# Use shortcut with predefined time
tasklog log --shortcut daily

# Use shortcut but override the time
tasklog log --shortcut daily --time 45m

# Short form
tasklog log -s standup
```

### Command-Line Flags

Skip interactive prompts by providing values via flags:

```bash
# Log 2.5 hours to a specific task
tasklog log --task PROJ-123 --time 2.5h --label development

# Short form
tasklog log -t PROJ-123 -d 2h30m -l bug-fix
```

### View Summary

See today's logged time:

```bash
tasklog summary
```

### Register a Break

Take a break and automatically update Slack status and post a message:

```bash
# Take a lunch break (updates Slack status and posts message)
tasklog break lunch

# Take a prayer break
tasklog break prayer

# Take a coffee break
tasklog break coffee
```

This will:
1. Update your Slack status with the break emoji and duration
2. Post a formatted message in the configured channel (e.g., "🔔 Taking a *lunch break* — Back in 60 minutes at *2:30 PM*")
3. Set Slack status to auto-expire after the break duration
4. Display confirmation with return time

**Example Slack Message:**
```
🔔 Taking a lunch break — Back in 60 minutes at 2:30 PM
```

**Note:** Slack integration is optional. If not configured, the break will be registered locally but Slack won't be updated.
tasklog summary
```

Example output:
```
═══════════════════════════════════════════
Today's Summary (3 entries)
═══════════════════════════════════════════
✓ 09:00 - 2h         [development] PROJ-123
✓ 11:30 - 30m        [meeting] PROJ-456
✓ 14:00 - 1h 30m     [code-review] PROJ-789
───────────────────────────────────────────
Total: 4h
═══════════════════════════════════════════
```

### Sync Failed Entries

If logging to Jira or Tempo fails, entries are saved locally. Retry syncing:

```bash
tasklog sync
```

## Time Format Support

Tasklog supports multiple time formats, all rounded to the nearest 5 minutes:

- `2h 30m` - Hours and minutes with space
- `2h30m` - Hours and minutes without space
- `2.5h` - Decimal hours
- `150m` - Minutes only
- `2h` - Hours only

Examples:
- `2h 32m` → rounded to `2h 30m` (150 minutes)
- `2h 27m` → rounded to `2h 25m` (145 minutes)
- `1m` → rounded to `5m`

## Shortcuts for Automation

Shortcuts are perfect for cronjobs or repetitive tasks:

```bash
# Add to crontab for daily standup at 9:30 AM
30 9 * * 1-5 /usr/local/bin/tasklog log -s daily
```

If time is not predefined in the shortcut, the command will prompt for it.

## Development

### Project Structure

```
tasklog/
├── cmd/                 # CLI commands
│   ├── root.go         # Root command
│   ├── log.go          # Log time command
│   ├── sync.go         # Sync command
│   └── summary.go      # Summary command
├── internal/
│   ├── config/         # Configuration management
│   ├── jira/           # Jira API client
│   ├── tempo/          # Tempo API client
│   ├── storage/        # SQLite storage layer
│   ├── timeparse/      # Time parsing logic
│   └── ui/             # Interactive prompts
├── main.go             # Application entry point
├── config.example.yaml # Example configuration
└── README.md
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
make test-coverage

# View coverage summary
./scripts/coverage.sh
```

**Test Coverage**: Core business logic achieves **83.9%** average coverage (config: 83%, storage: 81.8%, timeparse: 89.2%). See [TESTING.md](TESTING.md) for details.

### Build Options

```bash
# Development build
go build -o tasklog

# Production build with optimizations
go build -ldflags="-s -w" -o tasklog

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o tasklog-linux
```

## Troubleshooting

### Config file not found

```bash
# Check if config directory exists
ls ~/.tasklog/

# Create config from example
cp config.example.yaml ~/.tasklog/config.yaml
# Then edit with your credentials
```

### Database errors

```bash
# Check database file
ls -la ~/.tasklog/tasklog.db

# Remove and recreate (WARNING: loses local data)
rm ~/.tasklog/tasklog.db
tasklog log  # Will recreate on first use
```

### API authentication errors

- Verify your Jira URL (should end with .atlassian.net)
- Ensure API tokens are valid and not expired
- Check that your Jira username is correct (usually your email)
- Verify Tempo is installed in your Jira instance

### Enable debug logging

Set the log level to debug for more verbose output:

```bash
# Edit main.go and change:
zerolog.SetGlobalLevel(zerolog.DebugLevel)
```

## Environment Variables

Alternative to config file:

- `TASKLOG_CONFIG` - Path to config file (default: `~/.tasklog/config.yaml`)

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - See LICENSE file for details

## Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Uses [Survey](https://github.com/AlecAivazis/survey) for interactive prompts
- Logging with [Zerolog](https://github.com/rs/zerolog)
- SQLite integration via [go-sqlite3](https://github.com/mattn/go-sqlite3)
