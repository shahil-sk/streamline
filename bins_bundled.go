//go:build bundled

package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed yt-dlp
var ytDLP []byte

//go:embed ffmpeg
var ffmpegBin []byte

// resolveBinaries extracts the embedded yt-dlp and ffmpeg binaries into a
// temporary directory. Returns their paths and a cleanup function that removes
// the temp dir on exit. Built with: go build -tags bundled
func resolveBinaries() (ytdlpPath, ffmpegPath string, cleanup func()) {
	tempDir, err := os.MkdirTemp("", "streamline-bins")
	check(err)
	cleanup = func() { os.RemoveAll(tempDir) }

	// Windows does not honour the Unix execute bit; 0666 is sufficient there
	perm := os.FileMode(0755)
	if runtime.GOOS == "windows" {
		perm = 0666
	}

	ytdlpPath = filepath.Join(tempDir, exeName("yt-dlp"))
	ffmpegPath = filepath.Join(tempDir, exeName("ffmpeg"))

	check(os.WriteFile(ytdlpPath, ytDLP, perm))
	check(os.WriteFile(ffmpegPath, ffmpegBin, perm))

	return ytdlpPath, ffmpegPath, cleanup
}
