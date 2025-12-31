# Migration Notes - Tasklog v2 (Refactor)

This release introduces a major internal refactor, replacing the `survey` library with `bubbletea` (via `huh`) and standardizing output.

## New Package Layout

- **internal/output**: Containing standard output logic.
  - `output.Writer` interface for unified logging (Print, Success, Warn, Error, Debug).
  - Use `output.NewConsole()` to instantiate.
- **internal/ui**: Now uses `github.com/charmbracelet/huh` for interactive forms.
  - Split into `select_task.go`, `input.go`, `label.go`, `confirm.go`.
- **internal/updater**: Removed `PerformUpgrade` (which contained UI logic). Use `InstallUpdate` instead and handle UI in the calling command.

## How to Add a New Screen

1. **Define the interaction**:
   Use `huh.NewForm` and groups to define the fields.

   ```go
   form := huh.NewForm(
       huh.NewGroup(
           huh.NewInput().Title("Question?").Value(&result),
       ),
   )
   ```

2. **Run the form**:

   ```go
   if err := form.Run(); err != nil {
       return err
   }
   ```

3. **Style**:
   `huh` handles most styling. For custom output, use `internal/output` package which uses `lipgloss`.

## How to Print Output Correctly

**Do not use** `fmt.Println` or `log.Println` directly.

Use the global `cmd.Out` (or pass `output.Writer` explicitly):

```go
// Output normal information
cmd.Out.Println("Task created")

// Output success (green checkmark)
cmd.Out.Success("Task created successfully")

// Output warnings (yellow warning sign)
cmd.Out.Warn("Configuration missing")

// Output errors (red cross)
cmd.Out.Error(err)

// Output debug output (only if verbose, printed in grey)
cmd.Out.Debug("Parsing response...")
```
