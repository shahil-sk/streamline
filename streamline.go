package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const authorTag = "Streamline by SK (Shahil Ahmed)"
const releasesURL = "https://github.com/shahil-sk/streamline/releases/latest"

// ANSI colour codes – zeroed out on non-TTY or Windows
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

// Package-level precompiled regexes – compiled once at startup, not per call
var (
	reProgressFull = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?\s*([\d.]+\s*[KMGT]i?B?)`)
	reProgressPct  = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)
	reSizeExtract  = regexp.MustCompile(`of\s+~?\s*([\d.]+\s*[KMGT]i?B?)`)
	reParseSize    = regexp.MustCompile(`([\d.]+)\s*([KMGT]?i?B?)`)
)

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

// ─── Missing Dependency Error ─────────────────────────────────────────────────

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


// ─── Progress Bar ────────────────────────────────────────────────────────────

type ProgressBar struct {
	total       float64
	current     float64
	width       int
	description string
	startTime   time.Time
	lastUpdate  time.Time
}

func NewProgressBar(description string, width int) *ProgressBar {
	now := time.Now()
	return &ProgressBar{
		description: description,
		width:       width,
		startTime:   now,
		lastUpdate:  now,
	}
}

func (p *ProgressBar) Update(current, total float64) {
	p.current = current
	p.total = total
	if time.Since(p.lastUpdate) < 100*time.Millisecond && current < total {
		return
	}
	p.lastUpdate = time.Now()
	p.Render()
}

func (p *ProgressBar) Render() {
	if p.total == 0 {
		return
	}
	percent := (p.current / p.total) * 100
	if percent > 100 {
		percent = 100
	}
	filled := int((percent / 100) * float64(p.width))
	if filled > p.width {
		filled = p.width
	}
	bar := strings.Repeat("█", filled)
	empty := strings.Repeat("░", p.width-filled)

	elapsed := time.Since(p.startTime).Seconds()
	if elapsed < 0.1 {
		elapsed = 0.1
	}
	speed := p.current / elapsed
	remaining := 0.0
	if speed > 0 {
		remaining = (p.total - p.current) / speed
	}

	const mib = 1024 * 1024
	fmt.Printf("\r%s%s%s %s%s%s%s%s │ %s%.1f%%%s │ %s%.2f/%.2f MB%s │ %s%.2f MB/s%s │ ETA: %s%s%s    ",
		colorBold, p.description, colorReset,
		colorGreen, bar, colorDim, empty, colorReset,
		colorCyan, percent, colorReset,
		colorYellow, p.current/mib, p.total/mib, colorReset,
		colorBlue, speed/mib, colorReset,
		colorGreen, formatDuration(remaining), colorReset)
}

func (p *ProgressBar) Complete() {
	if p.total > 0 {
		p.current = p.total
		p.Render()
	}
	fmt.Println()
}

// ─── Spinner ─────────────────────────────────────────────────────────────────

type Spinner struct {
	frames  []string
	index   int
	message string
	stop    chan struct{}
}

func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		message: message,
		stop:    make(chan struct{}),
	}
}

func (s *Spinner) Start() {
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				fmt.Printf("\r%s%s%s %s   ", colorCyan, s.frames[s.index], colorReset, s.message)
				s.index = (s.index + 1) % len(s.frames)
			}
		}
	}()
}

// Stop signals the spinner goroutine via channel close (race-free, one-shot)
func (s *Spinner) Stop(success bool) {
	close(s.stop)
	time.Sleep(100 * time.Millisecond)
	icon, color := "✓", colorGreen
	if !success {
		icon, color = "✗", colorRed
	}
	fmt.Printf("\r%s%s%s %s\n", color, icon, colorReset, s.message)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func formatDuration(seconds float64) string {
	if seconds < 0 || seconds > 86400 {
		return "--:--"
	}
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	if minutes > 60 {
		hours := minutes / 60
		minutes %= 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func usage() {
	fmt.Printf(`%s╔═════════════════════════════════════════════╗
║  %sStreamline%s - YouTube/SoundCloud Downloader ║
╚═════════════════════════════════════════════╝%s

%sUsage:%s
  streamline -m [flags] <url>    Download audio
  streamline -v [flags] <url>    Download video

%sExamples:%s
  streamline -m https://youtube.com/watch?v=xxxxx
  streamline -v -q -o ~/Downloads https://youtu.be/xxxxx

%sFlags:%s
  %s-m%s        Music/audio mode
  %s-v%s        Video mode
  %s-o%s        Output directory (default: current directory)
  %s-q%s        Quiet mode (skip prompts, use best quality)
  %s-s%s        Remove sponsor segments (SponsorBlock)
  %s--subs%s    Embed subtitles (video only)
  %s--about%s   Author information

`,
		colorCyan, colorBold, colorReset, colorReset,
		colorYellow, colorReset,
		colorYellow, colorReset,
		colorYellow, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset)
	runCleanups()
	os.Exit(0)
}

func printBanner() {
	const banner = `
╔═════════════════════════════════════════════╗
║ Streamline - YouTube/SoundCloud Downloader  ║
╚═════════════════════════════════════════════╝`
	fmt.Printf("%s%s%s\n", colorCyan, banner, colorReset)
}

func printStatus(status, message string) {
	type entry struct{ icon, color string }
	table := map[string]entry{
		"info":    {"ℹ", colorBlue},
		"success": {"✓", colorGreen},
		"warning": {"⚠", colorYellow},
		"error":   {"✗", colorRed},
	}
	e, ok := table[status]
	if !ok {
		e = entry{"•", colorReset}
	}
	fmt.Printf("%s%s%s %s\n", e.color, e.icon, colorReset, message)
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

// scannerBufSize is large enough to handle yt-dlp's widest output lines
const scannerBufSize = 256 * 1024

func runYTDLPWithProgress(ytdlpPath, ffmpegDir, description string, args ...string) {
	args = append(args, "--newline", "--progress")
	cmd := exec.Command(ytdlpPath, args...)
	cmd.Env = append(os.Environ(),
		"PATH="+ffmpegDir+string(filepath.ListSeparator)+os.Getenv("PATH"))

	stdout, err := cmd.StdoutPipe()
	check(err)
	stderr, err := cmd.StderrPipe()
	check(err)
	check(cmd.Start())

	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	scanner.Buffer(make([]byte, scannerBufSize), scannerBufSize)

	var (
		progressBar *ProgressBar
		totalSize   float64
		lastError   string
	)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "ERROR:") {
			lastError = line
		}

		if !strings.Contains(line, "[download]") {
			if strings.Contains(line, "Merging formats") {
				if progressBar != nil {
					progressBar.Complete()
					progressBar = nil
				}
				printStatus("info", "Merging video and audio streams...")
			}
			continue
		}

		// Lazily capture total size from the first size annotation seen
		if totalSize == 0 {
			if m := reSizeExtract.FindStringSubmatch(line); len(m) >= 2 {
				totalSize = parseSize(m[1])
			}
		}

		switch {
		case strings.Contains(line, "Destination:"):
			if progressBar != nil {
				progressBar.Complete()
				progressBar = nil
			}
			totalSize = 0 // Reset for the new file in playlist
			filename := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
			printStatus("info", "File: "+filepath.Base(filename))

		case strings.Contains(line, "has already been downloaded"):
			if progressBar != nil {
				progressBar.Complete()
				progressBar = nil
			}
			totalSize = 0 // Reset for the new file in playlist
			printStatus("warning", "File already exists, skipping...")

		default:
			if m := reProgressFull.FindStringSubmatch(line); len(m) >= 3 {
				pct, _ := strconv.ParseFloat(m[1], 64)
				total := parseSize(m[2])
				if total > 0 {
					totalSize = total
					if progressBar == nil {
						progressBar = NewProgressBar(description, 40)
					}
					progressBar.Update(total*(pct/100), total)
					if pct >= 100 {
						progressBar.Complete()
						progressBar = nil
					}
				}
			} else if m := reProgressPct.FindStringSubmatch(line); len(m) >= 2 && totalSize > 0 {
				pct, _ := strconv.ParseFloat(m[1], 64)
				if progressBar == nil {
					progressBar = NewProgressBar(description, 40)
				}
				progressBar.Update(totalSize*(pct/100), totalSize)
				if pct >= 100 {
					progressBar.Complete()
					progressBar = nil
				}
			}
		}
	}

	if progressBar != nil {
		progressBar.Complete()
	}
	if err := cmd.Wait(); err != nil {
		if lastError != "" {
			fmt.Fprintf(os.Stderr, "\n%s✗ Download failed:%s %s\n", colorRed, colorReset, lastError)
		} else {
			fmt.Fprintf(os.Stderr, "\n%s✗ Download failed:%s %v\n", colorRed, colorReset, err)
		}
		runCleanups()
		os.Exit(1)
	}
}

// embedThumbnail crops the thumbnail to a square, scales it to 500x500,
// and embeds it into the audio file as cover art.
func embedThumbnail(ffmpegPath, audioFile, thumbFile string) {
	ext := strings.ToLower(filepath.Ext(audioFile))
	if ext != ".mp3" && ext != ".m4a" && ext != ".flac" {
		return // Manual square embed not strictly supported for this format
	}

	printStatus("info", "Cropping thumbnail to square and embedding...")
	spinner := NewSpinner("Embedding album art (500×500)...")
	spinner.Start()

	tempFile := audioFile + ".temp"
	var args []string

	if ext == ".mp3" {
		args = []string{
			"-i", audioFile, "-i", thumbFile,
			"-map", "0:0", "-map", "1:0",
			"-c:a", "copy", "-c:v", "mjpeg",
			"-vf", "crop=min(iw\\,ih):min(iw\\,ih),scale=500:500",
			"-q:v", "2", "-id3v2_version", "3",
			"-metadata:s:v", "title=Album cover",
			"-metadata:s:v", "comment=Cover (front)",
			"-y", "-loglevel", "error", "-f", "mp3", tempFile,
		}
	} else if ext == ".m4a" {
		args = []string{
			"-i", audioFile, "-i", thumbFile,
			"-map", "0:a", "-map", "1:v",
			"-c:a", "copy", "-c:v", "mjpeg",
			"-vf", "crop=min(iw\\,ih):min(iw\\,ih),scale=500:500",
			"-q:v", "2", "-disposition:v", "attached_pic",
			"-y", "-loglevel", "error", "-f", "ipod", tempFile,
		}
	} else if ext == ".flac" {
		args = []string{
			"-i", audioFile, "-i", thumbFile,
			"-map", "0:a", "-map", "1:v",
			"-c:a", "copy", "-c:v", "mjpeg",
			"-vf", "crop=min(iw\\,ih):min(iw\\,ih),scale=500:500",
			"-q:v", "2", "-disposition:v", "attached_pic",
			"-y", "-loglevel", "error", "-f", "flac", tempFile,
		}
	}

	cmd := exec.Command(ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	spinner.Stop(err == nil)
	if err != nil {
		exitWithError(fmt.Sprintf("Failed to embed thumbnail: %v\n%s", err, string(output)))
	}
	check(os.Rename(tempFile, audioFile))
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

// ─── Download Commands ───────────────────────────────────────────────────────

func audioDownload(ytdlpPath, ffmpegPath, workDir, url, outDir string, quiet, sponsorBlock bool) {
	if !quiet {
		printBanner()
	}

	var audioFmt string
	var audioQuality string
	formats := []string{"mp3", "m4a", "flac", "wav", "opus"}

	if quiet {
		audioFmt = "mp3"
		audioQuality = "320K"
	} else {
		fmt.Printf("%s┌─ Audio Format ──────────────────────────────┐%s\n", colorYellow, colorReset)
		for i, f := range formats {
			fmt.Printf("%s│%s %s%d.%s %-40s %s│%s\n",
				colorYellow, colorReset,
				colorGreen, i+1, colorReset,
				strings.ToUpper(f),
				colorYellow, colorReset)
		}
		fmt.Printf("%s└─────────────────────────────────────────────┘%s\n\n", colorYellow, colorReset)

		input := readInput(fmt.Sprintf("%sChoose format (1-%d) [default: 1]:%s ", colorCyan, len(formats), colorReset))
		choice, _ := strconv.Atoi(input)
		if choice < 1 || choice > len(formats) {
			choice = 1
		}
		audioFmt = formats[choice-1]
		fmt.Println()

		if audioFmt == "mp3" || audioFmt == "m4a" || audioFmt == "opus" {
			fmt.Printf("%s┌─ Audio Quality ─────────────────────────────┐%s\n", colorYellow, colorReset)
			qualities := []struct{ label, val string }{
				{"320kbps (Best)", "320K"},
				{"256kbps (High)", "256K"},
				{"192kbps (Standard)", "192K"},
				{"128kbps (Low)", "128K"},
			}
			for i, q := range qualities {
				fmt.Printf("%s│%s %s%d.%s %-40s %s│%s\n",
					colorYellow, colorReset,
					colorGreen, i+1, colorReset,
					q.label,
					colorYellow, colorReset)
			}
			fmt.Printf("%s└─────────────────────────────────────────────┘%s\n\n", colorYellow, colorReset)

			qInput := readInput(fmt.Sprintf("%sChoose quality (1-%d) [default: 1]:%s ", colorCyan, len(qualities), colorReset))
			qChoice, _ := strconv.Atoi(qInput)
			if qChoice < 1 || qChoice > len(qualities) {
				qChoice = 1
			}
			audioQuality = qualities[qChoice-1].val
			fmt.Println()
		}
	}

	if !quiet {
		spinner := NewSpinner("Fetching video information...")
		spinner.Start()
		time.Sleep(500 * time.Millisecond)
		spinner.Stop(true)
	}

	printStatus("info", "Starting audio download...")
	if !quiet {
		fmt.Println()
	}

	ffmpegDir := filepath.Dir(ffmpegPath)
	
	args := []string{
		url,
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", audioFmt,
		"--convert-thumbnails", "jpg",
		"--embed-metadata",
		"--embed-chapters",
		"--add-metadata",
		"-o", filepath.Join(workDir, "%(title)s.%(ext)s"),
		"--write-thumbnail",
	}
	if audioQuality != "" {
		args = append(args, "--audio-quality", audioQuality)
	}
	if sponsorBlock {
		args = append(args, "--sponsorblock-remove", "all")
	}

	runYTDLPWithProgress(ytdlpPath, ffmpegDir, "Downloading audio", args...)

	audioFiles, err := filepath.Glob(filepath.Join(workDir, "*."+audioFmt))
	if err != nil {
		exitWithError(fmt.Sprintf("Failed to scan output directory: %v", err))
	}
	if len(audioFiles) == 0 {
		exitWithError("No audio file found")
	}

	for _, audioFile := range audioFiles {
		baseWithoutExt := strings.TrimSuffix(audioFile, "."+audioFmt)
		
		thumbExtensions := []string{".jpg", ".jpeg", ".png", ".webp"}
		var thumbFile string
		for _, ext := range thumbExtensions {
			p := baseWithoutExt + ext
			if _, err := os.Stat(p); err == nil {
				thumbFile = p
				break
			}
		}
		if thumbFile != "" {
			embedThumbnail(ffmpegPath, audioFile, thumbFile)
			os.Remove(thumbFile)
		}

		destName := filepath.Base(audioFile)
		destPath := destName
		if outDir != "" {
			destPath = filepath.Join(outDir, destName)
		}
		if err := moveFile(audioFile, destPath); err != nil {
			exitWithError(fmt.Sprintf("Failed to save audio file: %v", err))
		}

		if !quiet {
			fmt.Println()
		}
		printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, destName, colorReset))
	}
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

func videoDownload(ytdlpPath, ffmpegPath, workDir, url, outDir string, quiet, sponsorBlock, subtitles bool) {
	if !quiet {
		printBanner()
	}

	var format string
	presets := []struct{ label, format string }{
		{"Best Quality (Auto)", "bestvideo+bestaudio/best"},
		{"1080p", "bestvideo[height<=1080]+bestaudio/best[height<=1080]"},
		{"720p", "bestvideo[height<=720]+bestaudio/best[height<=720]"},
		{"480p", "bestvideo[height<=480]+bestaudio/best[height<=480]"},
		{"360p", "bestvideo[height<=360]+bestaudio/best[height<=360]"},
		{"Custom Format (Advanced)", ""},
	}

	ffmpegDir := filepath.Dir(ffmpegPath)

	if quiet {
		format = presets[0].format
	} else {
		fmt.Printf("%s┌─ Quality Presets ───────────────────────────┐%s\n", colorYellow, colorReset)
		for i, p := range presets {
			fmt.Printf("%s│%s %s%d.%s %-40s %s│%s\n",
				colorYellow, colorReset,
				colorGreen, i+1, colorReset,
				p.label,
				colorYellow, colorReset)
		}
		fmt.Printf("%s└─────────────────────────────────────────────┘%s\n\n", colorYellow, colorReset)

		input := readInput(fmt.Sprintf("%sChoose quality (1-6):%s ", colorCyan, colorReset))
		var choice int
		if input != "" {
			choice, _ = strconv.Atoi(input)
		}
		fmt.Println()

		switch {
		case choice > 0 && choice < len(presets):
			format = presets[choice-1].format
		case choice == len(presets):
			spinner := NewSpinner("Fetching available formats...")
			spinner.Start()
			cmd := exec.Command(ytdlpPath, "-F", url)
			cmd.Env = append(os.Environ(),
				"PATH="+ffmpegDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
			output, err := cmd.CombinedOutput()
			spinner.Stop(err == nil)
			if err != nil {
				exitWithError(fmt.Sprintf("Failed to fetch formats:\n%s", string(output)))
			}
			fmt.Println(string(output))
			format = readInput(fmt.Sprintf("\n%sEnter format ID or combination (e.g., 137+140):%s ", colorCyan, colorReset))
			fmt.Println()
			if format == "" {
				printStatus("warning", "No format entered, using best quality")
				format = "bestvideo+bestaudio/best"
			}
		default:
			printStatus("warning", "Invalid choice, using best quality")
			format = "bestvideo+bestaudio/best"
		}
	}

	printStatus("info", "Starting video download...")
	if !quiet {
		fmt.Println()
	}

	args := []string{
		"-f", format,
		"-o", filepath.Join(workDir, "%(title)s.%(ext)s"),
	}
	if sponsorBlock {
		args = append(args, "--sponsorblock-remove", "all")
	}
	if subtitles {
		args = append(args, "--write-auto-subs", "--write-subs", "--embed-subs", "--sub-langs", "all,-live_chat")
	}
	args = append(args, url)

	runYTDLPWithProgress(ytdlpPath, ffmpegDir, "Downloading video", args...)

	files, err := filepath.Glob(filepath.Join(workDir, "*"))
	if err != nil {
		exitWithError(fmt.Sprintf("Failed to scan output directory: %v", err))
	}

	var downloadedCount int
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".part" || ext == ".ytdl" {
			continue
		}
		fi, err := os.Stat(file)
		if err != nil || fi.IsDir() {
			continue
		}

		destName := filepath.Base(file)
		destPath := destName
		if outDir != "" {
			destPath = filepath.Join(outDir, destName)
		}
		if err := moveFile(file, destPath); err != nil {
			exitWithError(fmt.Sprintf("Failed to save video file: %v", err))
		}
		if !quiet {
			fmt.Println()
		}
		printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, destName, colorReset))
		downloadedCount++
	}

	if downloadedCount == 0 {
		exitWithError("No video files downloaded")
	}
}

// ─── Entry Point ─────────────────────────────────────────────────────────────

func main() {
	// Setup signal handling for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\n\n%s✗ Interrupted, cleaning up...%s\n", colorRed, colorReset)
		runCleanups()
		os.Exit(1)
	}()

	musicMode := flag.Bool("m", false, "Music/audio mode")
	videoMode := flag.Bool("v", false, "Video mode")
	outDir := flag.String("o", "", "Output directory")
	quiet := flag.Bool("q", false, "Quiet mode")
	sponsorBlock := flag.Bool("s", false, "Remove sponsor segments")
	subtitles := flag.Bool("subs", false, "Embed subtitles")
	about := flag.Bool("about", false, "Show author info")

	flag.Usage = usage
	flag.Parse()

	if *about {
		fmt.Printf("\n%s%s%s\n", colorCyan, authorTag, colorReset)
		fmt.Printf("\n%sGitHub:%s %shttps://github.com/shahil-sk/streamline%s\n\n",
			colorYellow, colorReset, colorBlue, colorReset)
		os.Exit(0)
	}

	if (!*musicMode && !*videoMode) || flag.NArg() < 1 {
		usage()
	}

	urlArg := strings.TrimSpace(flag.Arg(0))
	if err := validateURL(urlArg); err != nil {
		exitWithError(fmt.Sprintf("Invalid URL: %v", err))
	}

	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0755); err != nil {
			exitWithError(fmt.Sprintf("Failed to create output directory: %v", err))
		}
	}

	ytdlpPath, ffmpegPath, cleanup := resolveBinaries()
	registerCleanup(cleanup)
	defer runCleanups()

	// Isolated work dir prevents glob from accidentally matching files in the user's CWD
	workDir, err := os.MkdirTemp("", "streamline-work")
	check(err)
	registerCleanup(func() { os.RemoveAll(workDir) })

	if *musicMode {
		audioDownload(ytdlpPath, ffmpegPath, workDir, urlArg, *outDir, *quiet, *sponsorBlock)
	} else if *videoMode {
		videoDownload(ytdlpPath, ffmpegPath, workDir, urlArg, *outDir, *quiet, *sponsorBlock, *subtitles)
	}
}
