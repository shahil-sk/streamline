//go:build !bundled

package main

import (
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
		missingDepError("yt-dlp", "https://github.com/yt-dlp/yt-dlp")
	}

	ffmpegPath, err = exec.LookPath(exeName("ffmpeg"))
	if err != nil {
		missingDepError("ffmpeg", "https://ffmpeg.org/download.html")
	}

	return ytdlpPath, ffmpegPath, cleanup
}
