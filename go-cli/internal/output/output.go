package output

import (
	"fmt"
	"os"
)

var Verbose bool

func Info(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func Success(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

func Debug(format string, args ...any) {
	if Verbose {
		fmt.Fprintf(os.Stderr, "debug: "+format+"\n", args...)
	}
}
