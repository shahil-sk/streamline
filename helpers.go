package main

import (
	"fmt"
	"os"
	"runtime"
)

// check exits with a styled error message if err is non-nil.
// No build tag – visible to both the default and bundled builds.
func check(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s✗ Error:%s %v\n", colorRed, colorReset, err)
		os.Exit(1)
	}
}

// exeName appends .exe on Windows for cross-platform portability.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
