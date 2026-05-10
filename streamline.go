//go:build !bundled

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// resolveBinaries locates yt-dlp and ffmpeg on the system PATH.
// This file is excluded when building with -tags bundled;
// bins_bundled.go provides an alternative resolveBinaries that extracts
// embedded binaries instead.
func resolveBinaries() (ytdlpPath, ffmpegPath string, cleanup func()) {
	cleanup = func() {}

	printStatus("info", "Resolving dependencies...")
	debugLog("Looking up yt-dlp on PATH (GOOS=%s)", runtime.GOOS)

	var err error
	ytdlpPath, err = exec.LookPath(exeName("yt-dlp"))
	if err != nil {
		missingDepError("yt-dlp", "https://github.com/yt-dlp/yt-dlp")
	}
	debugLog("yt-dlp found: %s", ytdlpPath)

	debugLog("Looking up ffmpeg on PATH")
	ffmpegPath, err = exec.LookPath(exeName("ffmpeg"))
	if err != nil {
		missingDepError("ffmpeg", "https://ffmpeg.org/download.html")
	}
	debugLog("ffmpeg found: %s", ffmpegPath)

	printStatus("success", fmt.Sprintf("Dependencies OK  %s(yt-dlp: %s | ffmpeg: %s)%s",
		colorDim, filepath.Base(ytdlpPath), filepath.Base(ffmpegPath), colorReset))

	return ytdlpPath, ffmpegPath, cleanup
}

// missingDepError prints a helpful install hint and exits.
func missingDepError(name, installURL string) {
	var installHint string
	switch runtime.GOOS {
	case "linux":
		switch name {
		case "yt-dlp":
			installHint = "sudo curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && sudo chmod +x /usr/local/bin/yt-dlp"
		case "ffmpeg":
			installHint = "sudo apt install ffmpeg   # Debian/Ubuntu\n  sudo dnf install ffmpeg   # Fedora/RHEL\n  sudo pacman -S ffmpeg     # Arch"
		}
	case "darwin":
		switch name {
		case "yt-dlp":
			installHint = "brew install yt-dlp"
		case "ffmpeg":
			installHint = "brew install ffmpeg"
		}
	case "windows":
		switch name {
		case "yt-dlp":
			installHint = "winget install yt-dlp.yt-dlp   OR   scoop install yt-dlp"
		case "ffmpeg":
			installHint = "winget install Gyan.FFmpeg   OR   scoop install ffmpeg"
		}
	}
	if installHint == "" {
		installHint = installURL
	}

	fmt.Fprintf(os.Stderr, `
%s╔══════════════════════════════════════════════════╗
║  Missing dependency: %-28s║
╚══════════════════════════════════════════════════╝%s

%s✗ %s%s was not found on your system PATH.

%sOption 1 – Install %s:%s
  %s

%sOption 2 – Use the standalone (bundled) build:%s
  Download a self-contained binary that includes yt-dlp and ffmpeg.
  No extra installs needed.

  %s%s%s

`,
		colorRed, name+" ", colorReset,
		colorRed, name, colorReset,
		colorYellow, name, colorReset,
		installHint,
		colorYellow, colorReset,
		colorBlue, releasesURL, colorReset,
	)
	os.Exit(1)
}
