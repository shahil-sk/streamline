package main

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
)

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

var (
	cleanups    []func()
	cleanupsRun bool
)

func registerCleanup(fn func()) {
	if fn != nil {
		cleanups = append(cleanups, fn)
	}
}

func runCleanups() {
	if cleanupsRun {
		return
	}
	cleanupsRun = true
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func exitWithError(msg string) {
	printStatus("error", msg)
	runCleanups()
	os.Exit(1)
}

func check(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s✗ Error:%s %v\n", colorRed, colorReset, err)
		runCleanups()
		os.Exit(1)
	}
}

// exeName appends .exe on Windows for cross-platform portability
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// missingDepError prints a styled, actionable error when a required system
// dependency is not found, then exits with code 1.
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
	runCleanups()
	os.Exit(1)
}

func parseSize(sizeStr string) float64 {
	sizeStr = strings.TrimSpace(sizeStr)
	matches := reParseSize.FindStringSubmatch(sizeStr)
	if len(matches) < 3 {
		return 0
	}
	value, _ := strconv.ParseFloat(matches[1], 64)
	unit := strings.ToUpper(matches[2])
	if len(unit) == 1 && unit != "B" {
		unit += "B"
	}
	multipliers := map[string]float64{
		"B": 1, "KB": 1024, "KIB": 1024,
		"MB": 1024 * 1024, "MIB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024, "GIB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024, "TIB": 1024 * 1024 * 1024 * 1024,
	}
	if mult, ok := multipliers[unit]; ok {
		return value * mult
	}
	if len(unit) > 1 {
		if mult, ok := multipliers[unit[:len(unit)-1]+"IB"]; ok {
			return value * mult
		}
	}
	return 0
}

// copyFile copies src to dst byte-for-byte (cross-device fallback for os.Rename)
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// moveFile renames src to dst, falling back to copy+delete on cross-device moves
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		if err := copyFile(src, dst); err != nil {
			return err
		}
		os.Remove(src)
	}
	return nil
}

func readInput(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func validateURL(urlStr string) error {
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return fmt.Errorf("empty URL")
	}
	u, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return fmt.Errorf("malformed URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must use http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("invalid host")
	}
	return nil
}
