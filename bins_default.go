//go:build !bundled

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const releasesURL = "https://github.com/shahil-sk/streamline/releases/latest"

// missingDepError prints a styled, actionable error when a required dependency
// is not found, then exits. It shows:
//   - which binary is missing
//   - how to install it
//   - an alternative: grab the bundled (standalone) release
func missingDepError(name, installURL string) {
	// Derive a platform-appropriate install hint
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
  Download a self-contained binary that includes
  yt-dlp and ffmpeg — no extra installs needed.

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
