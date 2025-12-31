package output

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Styles
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	debugStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))  // Cyan

	// Icons
	successIcon = successStyle.Render("✓")
	warnIcon    = warnStyle.Render("⚠")
	errorIcon   = errorStyle.Render("✗")
	infoIcon    = infoStyle.Render("ℹ")
)

type consoleWriter struct {
	stdout  io.Writer
	stderr  io.Writer
	verbose bool
	plain   bool
}

// ConsoleOption defines a functional option for configuring the ConsoleWriter
type ConsoleOption func(*consoleWriter)

// NewConsole creates a new ConsoleWriter
func NewConsole(opts ...ConsoleOption) Writer {
	cw := &consoleWriter{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		verbose: false,
	}

	for _, opt := range opts {
		opt(cw)
	}

	return cw
}

// WithStdout sets the stdout writer
func WithStdout(w io.Writer) ConsoleOption {
	return func(cw *consoleWriter) {
		cw.stdout = w
	}
}

// WithStderr sets the stderr writer
func WithStderr(w io.Writer) ConsoleOption {
	return func(cw *consoleWriter) {
		cw.stderr = w
	}
}

// WithVerbose sets the verbose flag
func WithVerbose(verbose bool) ConsoleOption {
	return func(cw *consoleWriter) {
		cw.verbose = verbose
	}
}

// WithPlain disables styling
func WithPlain(plain bool) ConsoleOption {
	return func(cw *consoleWriter) {
		cw.plain = plain
	}
}

func (c *consoleWriter) Print(msg string) {
	fmt.Fprint(c.stdout, msg)
}

func (c *consoleWriter) Printf(format string, args ...interface{}) {
	fmt.Fprintf(c.stdout, format, args...)
}

func (c *consoleWriter) Println(args ...interface{}) {
	fmt.Fprintln(c.stdout, args...)
}

func (c *consoleWriter) Error(err error) {
	if c.plain {
		fmt.Fprintf(c.stderr, "Error: %v\n", err)
		return
	}
	fmt.Fprintf(c.stderr, "%s %v\n", errorIcon, err)
}

func (c *consoleWriter) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if c.plain {
		fmt.Fprintf(c.stderr, "Error: %s\n", msg)
		return
	}
	fmt.Fprintf(c.stderr, "%s %s\n", errorIcon, msg)
}

func (c *consoleWriter) Success(msg string) {
	if c.plain {
		fmt.Fprintf(c.stdout, "Success: %s\n", msg)
		return
	}
	fmt.Fprintf(c.stdout, "%s %s\n", successIcon, msg)
}

func (c *consoleWriter) Warn(msg string) {
	if c.plain {
		fmt.Fprintf(c.stdout, "Warning: %s\n", msg)
		return
	}
	fmt.Fprintf(c.stdout, "%s %s\n", warnIcon, msg)
}

func (c *consoleWriter) Info(msg string) {
	if c.plain {
		fmt.Fprintf(c.stdout, "Info: %s\n", msg)
		return
	}
	fmt.Fprintf(c.stdout, "%s %s\n", infoIcon, msg)
}

func (c *consoleWriter) Debug(msg string) {
	if !c.verbose {
		return
	}
	if c.plain {
		fmt.Fprintf(c.stdout, "Debug: %s\n", msg)
		return
	}
	fmt.Fprintf(c.stdout, "%s %s\n", debugStyle.Render("Debug:"), msg)
}
