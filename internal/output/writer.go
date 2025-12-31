package output

// Writer defines the interface for all output operations
type Writer interface {
	// Standard output
	Print(msg string)
	Printf(format string, args ...interface{})
	Println(args ...interface{})

	// Structured/Styled output
	Error(err error)
	Errorf(format string, args ...interface{})
	Success(msg string)
	Warn(msg string)
	Info(msg string)
	Debug(msg string)
}
