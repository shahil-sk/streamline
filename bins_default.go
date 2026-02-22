//go:build !bundled

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// resolveBinaries locates yt-dlp and ffmpeg on the system PATH.
// Returns their full paths and a no-op cleanup function.
// This is the default (lightweight) build – the binary itself stays tiny.
func resolveBinaries() (ytdlpPath, ffmpegPath string, cleanup func()) {
	cleanup = func() {}

	var err error
	ytdlpPath, err = exec.LookPath(exeName("yt-dlp"))
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"%s✗ Error:%s yt-dlp not found in PATH.\n  Install: https://github.com/yt-dlp/yt-dlp\n",
			colorRed, colorReset)
		os.Exit(1)
	}

	ffmpegPath, err = exec.LookPath(exeName("ffmpeg"))
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"%s✗ Error:%s ffmpeg not found in PATH.\n  Install: https://ffmpeg.org/download.html\n",
			colorRed, colorReset)
		os.Exit(1)
	}

	return ytdlpPath, ffmpegPath, cleanup
}
