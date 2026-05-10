package main

import (
	"os"
	"runtime"
)

// ANSI colour codes – zeroed out on non-TTY or Windows.
// Kept in a separate file (no build tag) so both the default and bundled
// builds can access them. helpers.go and bins_bundled.go depend on these.
var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// debugMode is enabled by the --debug or --verbose CLI flag.
// Declared here (no build tag) so both builds see it.
var debugMode bool

func init() {
	if runtime.GOOS == "windows" || !isTerminal() {
		colorReset, colorRed, colorGreen, colorYellow = "", "", "", ""
		colorBlue, colorCyan, colorBold, colorDim = "", "", "", ""
	}
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}
