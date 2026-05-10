//go:build !bundled

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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

// debugMode is enabled by the --debug or --verbose flag.
var debugMode bool

// Package-level precompiled regexes – compiled once at startup, not per call
var (
	reProgressFull = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?\s*([\d.]+\s*[KMGT]i?B?)`)
	reProgressPct  = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)
	reSizeExtract  = regexp.MustCompile(`of\s+~?\s*([\d.]+\s*[KMGT]i?B?)`)
	reParseSize    = regexp.MustCompile(`([\d.]+)\s*([KMGT]i?B?)`)
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

// ─── Debug / Verbose Logging ─────────────────────────────────────────────────

// debugLog prints a timestamped debug line to stderr when --debug is active.
// Format: [DEBUG 15:04:05.000] <message>
func debugLog(format string, args ...any) {
	if !debugMode {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s[DEBUG %s]%s %s\n", colorDim, ts, colorReset, msg)
}

// ─── Missing Dependency Error ─────────────────────────────────────────────────

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

// resolveBinaries locates yt-dlp and ffmpeg on the system PATH.
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
	debugLog("ProgressBar created: %q (width=%d)", description, width)
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
		const mib = 1024 * 1024
		debugLog("ProgressBar %q complete: %.2f MB in %s",
			p.description, p.total/mib,
			formatDuration(time.Since(p.startTime).Seconds()))
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
	debugLog("Spinner started: %q", message)
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

func (s *Spinner) Stop(success bool) {
	close(s.stop)
	time.Sleep(100 * time.Millisecond)
	icon, color := "✓", colorGreen
	if !success {
		icon, color = "✗", colorRed
	}
	fmt.Printf("\r%s%s%s %s\n", color, icon, colorReset, s.message)
	debugLog("Spinner stopped: %q success=%v", s.message, success)
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
  streamline -m <url>        Download audio with metadata and cover
  streamline -v <url>        Download video, choose quality manually
  streamline --about         Show author information
  streamline --debug -m <url>  Enable verbose debug output

%sExamples:%s
  streamline -m https://youtube.com/watch?v=xxxxx
  streamline -v https://youtu.be/xxxxx
  streamline --debug -m https://youtube.com/watch?v=xxxxx

%sFlags:%s
  %s-m%s          Music/audio mode (MP3 + metadata + cover art)
  %s-v%s          Video mode (quality selection)
  %s--about%s     Author information
  %s--debug%s     Enable verbose debug/diagnostic output
  %s--verbose%s   Alias for --debug

`,
		colorCyan, colorBold, colorReset, colorReset,
		colorYellow, colorReset,
		colorYellow, colorReset,
		colorYellow, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset)
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
		debugLog("parseSize: could not parse %q", sizeStr)
		return 0
	}
	value, _ := strconv.ParseFloat(matches[1], 64)
	unit := strings.ToUpper(matches[2])
	if len(unit) == 1 {
		unit += "B"
	}
	multipliers := map[string]float64{
		"B": 1, "KB": 1024, "KIB": 1024,
		"MB": 1024 * 1024, "MIB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024, "GIB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024, "TIB": 1024 * 1024 * 1024 * 1024,
	}
	if mult, ok := multipliers[unit]; ok {
		debugLog("parseSize: %q → %.2f bytes", sizeStr, value*mult)
		return value * mult
	}
	if len(unit) > 1 {
		if mult, ok := multipliers[unit[:len(unit)-1]+"IB"]; ok {
			debugLog("parseSize: %q → %.2f bytes (IB fallback)", sizeStr, value*mult)
			return value * mult
		}
	}
	debugLog("parseSize: unknown unit %q in %q", unit, sizeStr)
	return 0
}

const scannerBufSize = 256 * 1024

func runYTDLPWithProgress(ytdlpPath, ffmpegDir, description string, args ...string) {
	args = append(args, "--newline", "--progress")

	debugLog("Launching yt-dlp: %s %s", ytdlpPath, strings.Join(args, " "))

	cmd := exec.Command(ytdlpPath, args...)
	cmd.Env = append(os.Environ(),
		"PATH="+ffmpegDir+string(filepath.ListSeparator)+os.Getenv("PATH"))

	stdout, err := cmd.StdoutPipe()
	check(err)
	stderr, err := cmd.StderrPipe()
	check(err)
	check(cmd.Start())

	debugLog("yt-dlp PID=%d started", cmd.Process.Pid)

	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	scanner.Buffer(make([]byte, scannerBufSize), scannerBufSize)

	var (
		progressBar *ProgressBar
		totalSize   float64
		linesRead   int
	)

	for scanner.Scan() {
		line := scanner.Text()
		linesRead++
		debugLog("yt-dlp[%d]: %s", linesRead, line)

		if !strings.Contains(line, "[download]") {
			if strings.Contains(line, "Merging formats") {
				if progressBar != nil {
					progressBar.Complete()
					progressBar = nil
				}
				printStatus("info", "Merging video and audio streams...")
			} else if strings.Contains(line, "Extracting audio") {
				printStatus("info", "Extracting and converting to MP3...")
			} else if strings.Contains(line, "[EmbedThumbnail]") {
				printStatus("info", "Embedding thumbnail via yt-dlp...")
			} else if strings.Contains(line, "[Metadata]") {
				printStatus("info", "Writing metadata tags...")
			} else if strings.Contains(line, "ERROR") {
				printStatus("error", strings.TrimSpace(line))
			} else if strings.Contains(line, "WARNING") {
				printStatus("warning", strings.TrimSpace(line))
			}
			continue
		}

		if totalSize == 0 {
			if m := reSizeExtract.FindStringSubmatch(line); len(m) >= 2 {
				totalSize = parseSize(m[1])
				if totalSize > 0 {
					const mib = 1024 * 1024
					printStatus("info", fmt.Sprintf("File size: %s%.2f MB%s", colorCyan, totalSize/mib, colorReset))
				}
			}
		}

		switch {
		case strings.Contains(line, "Destination:"):
			if progressBar != nil {
				progressBar.Complete()
				progressBar = nil
			}
			filename := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
			printStatus("info", "Saving to: "+filename)
			debugLog("Destination file: %s", filename)

		case strings.Contains(line, "has already been downloaded"):
			printStatus("warning", "File already exists, skipping download.")

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

	debugLog("yt-dlp output finished: %d lines processed", linesRead)

	if progressBar != nil {
		progressBar.Complete()
	}
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "%s✗ Download failed:%s %v\n", colorRed, colorReset, err)
		debugLog("yt-dlp exited with error: %v", err)
		os.Exit(1)
	}
	debugLog("yt-dlp exited cleanly")
}

func embedThumbnail(ffmpegPath, mp3File, thumbFile string) {
	printStatus("info", "Cropping thumbnail to square and embedding...")
	debugLog("embedThumbnail: mp3=%s thumb=%s ffmpeg=%s", mp3File, thumbFile, ffmpegPath)

	spinner := NewSpinner("Embedding album art (500×500)...")
	spinner.Start()

	tempFile := mp3File + ".temp"
	cmd := exec.Command(ffmpegPath,
		"-i", mp3File,
		"-i", thumbFile,
		"-map", "0:0",
		"-map", "1:0",
		"-c:a", "copy",
		"-c:v", "mjpeg",
		"-vf", "crop=min(iw\\,ih):min(iw\\,ih),scale=500:500",
		"-q:v", "2",
		"-id3v2_version", "3",
		"-metadata:s:v", "title=Album cover",
		"-metadata:s:v", "comment=Cover (front)",
		"-y",
		"-loglevel", "error",
		"-f", "mp3",
		tempFile)

	err := cmd.Run()
	spinner.Stop(err == nil)
	if err != nil {
		debugLog("embedThumbnail ffmpeg error: %v", err)
	}
	check(err)

	debugLog("embedThumbnail: replacing %s with temp file", mp3File)
	check(os.Rename(tempFile, mp3File))
	printStatus("success", "Album art embedded successfully")
}

func copyFile(src, dst string) {
	debugLog("copyFile: %s → %s", src, dst)
	in, err := os.Open(src)
	check(err)
	defer in.Close()
	out, err := os.Create(dst)
	check(err)
	defer out.Close()
	_, err = io.Copy(out, in)
	check(err)
}

func moveFile(src, dst string) {
	debugLog("moveFile: %s → %s", src, dst)
	if err := os.Rename(src, dst); err != nil {
		debugLog("os.Rename failed (%v); falling back to copy+delete", err)
		copyFile(src, dst)
		os.Remove(src)
	}
}

// ─── Download Commands ───────────────────────────────────────────────────────

func audioDownload(ytdlpPath, ffmpegPath, workDir, url string) {
	printBanner()

	printStatus("info", fmt.Sprintf("URL: %s%s%s", colorBlue, url, colorReset))
	printStatus("info", fmt.Sprintf("Work dir: %s%s%s", colorDim, workDir, colorReset))
	debugLog("audioDownload called: url=%s workDir=%s", url, workDir)

	spinner := NewSpinner("Fetching video information...")
	spinner.Start()
	time.Sleep(500 * time.Millisecond)
	spinner.Stop(true)

	printStatus("info", "Mode: %saudio (MP3 + metadata + cover art)%s")
	printStatus("info", "Starting audio download...")
	fmt.Println()

	ffmpegDir := filepath.Dir(ffmpegPath)
	debugLog("ffmpegDir resolved to: %s", ffmpegDir)

	runYTDLPWithProgress(ytdlpPath, ffmpegDir, "Downloading audio",
		url,
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--convert-thumbnails", "jpg",
		"--embed-metadata",
		"--embed-chapters",
		"--add-metadata",
		"-o", filepath.Join(workDir, "%(title)s.%(ext)s"),
		"--write-thumbnail")

	mp3Files, err := filepath.Glob(filepath.Join(workDir, "*.mp3"))
	check(err)
	debugLog("MP3 glob found %d file(s): %v", len(mp3Files), mp3Files)

	if len(mp3Files) == 0 {
		printStatus("error", "No MP3 file found in work directory")
		debugLog("workDir contents follow")
		if entries, e := os.ReadDir(workDir); e == nil {
			for _, entry := range entries {
				debugLog("  %s", entry.Name())
			}
		}
		os.Exit(1)
	}

	mp3File := mp3Files[0]
	printStatus("info", fmt.Sprintf("Audio file: %s%s%s", colorBold, filepath.Base(mp3File), colorReset))

	if thumbFiles, _ := filepath.Glob(filepath.Join(workDir, "*.jpg")); len(thumbFiles) > 0 {
		debugLog("Thumbnail found: %s", thumbFiles[0])
		embedThumbnail(ffmpegPath, mp3File, thumbFiles[0])
		os.Remove(thumbFiles[0])
		debugLog("Thumbnail removed after embedding")
	} else {
		printStatus("warning", "No thumbnail found; skipping cover art embedding")
		debugLog("No *.jpg files in workDir")
	}

	dest := filepath.Base(mp3File)
	printStatus("info", fmt.Sprintf("Moving file to current directory: %s", dest))
	moveFile(mp3File, dest)

	fi, err := os.Stat(dest)
	if err == nil {
		const mib = 1024 * 1024
		printStatus("info", fmt.Sprintf("Final file size: %s%.2f MB%s",
			colorCyan, float64(fi.Size())/mib, colorReset))
	}

	fmt.Println()
	printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, dest, colorReset))
	debugLog("audioDownload finished: output=%s", dest)
}

func videoDownload(ytdlpPath, ffmpegPath, workDir, url string) {
	printBanner()

	printStatus("info", fmt.Sprintf("URL: %s%s%s", colorBlue, url, colorReset))
	printStatus("info", fmt.Sprintf("Work dir: %s%s%s", colorDim, workDir, colorReset))
	debugLog("videoDownload called: url=%s workDir=%s", url, workDir)

	presets := []struct{ label, format string }{
		{"Best Quality (Auto)", "bestvideo+bestaudio/best"},
		{"1080p", "bestvideo[height<=1080]+bestaudio/best[height<=1080]"},
		{"720p", "bestvideo[height<=720]+bestaudio/best[height<=720]"},
		{"480p", "bestvideo[height<=480]+bestaudio/best[height<=480]"},
		{"360p", "bestvideo[height<=360]+bestaudio/best[height<=360]"},
		{"Custom Format (Advanced)", ""},
	}

	fmt.Printf("%s┌─ Quality Presets ───────────────────────────┐%s\n", colorYellow, colorReset)
	for i, p := range presets {
		fmt.Printf("%s│%s %s%d.%s %-40s %s│%s\n",
			colorYellow, colorReset,
			colorGreen, i+1, colorReset,
			p.label,
			colorYellow, colorReset)
	}
	fmt.Printf("%s└─────────────────────────────────────────────┘%s\n\n", colorYellow, colorReset)

	fmt.Printf("%sChoose quality (1-%d):%s ", colorCyan, len(presets), colorReset)
	var choice int
	fmt.Scanln(&choice)
	fmt.Println()

	debugLog("User selected quality preset: %d", choice)

	ffmpegDir := filepath.Dir(ffmpegPath)
	debugLog("ffmpegDir resolved to: %s", ffmpegDir)
	var format string

	switch {
	case choice > 0 && choice < len(presets):
		format = presets[choice-1].format
		printStatus("info", fmt.Sprintf("Selected quality: %s%s%s", colorBold, presets[choice-1].label, colorReset))
		debugLog("Format string: %s", format)
	case choice == len(presets):
		printStatus("info", "Fetching available formats from server...")
		spinner := NewSpinner("Fetching available formats...")
		spinner.Start()
		cmd := exec.Command(ytdlpPath, "-F", url)
		cmd.Env = append(os.Environ(),
			"PATH="+ffmpegDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
		output, err := cmd.CombinedOutput()
		spinner.Stop(err == nil)
		if err != nil {
			debugLog("-F command failed: %v", err)
		}
		if err == nil {
			fmt.Println(string(output))
		}
		fmt.Printf("\n%sEnter format ID or combination (e.g., 137+140):%s ", colorCyan, colorReset)
		fmt.Scanln(&format)
		fmt.Println()
		debugLog("Custom format entered: %s", format)
	default:
		printStatus("warning", fmt.Sprintf("Invalid choice %d — falling back to best quality", choice))
		format = "bestvideo+bestaudio/best"
		debugLog("Invalid choice %d, defaulting to: %s", choice, format)
	}

	printStatus("info", "Starting video download...")
	fmt.Println()

	runYTDLPWithProgress(ytdlpPath, ffmpegDir, "Downloading video",
		"-f", format,
		"-o", filepath.Join(workDir, "%(title)s.%(ext)s"),
		url)

	if mp4Files, _ := filepath.Glob(filepath.Join(workDir, "*.mp4")); len(mp4Files) > 0 {
		debugLog("MP4 file found: %s", mp4Files[0])
		dest := filepath.Base(mp4Files[0])
		printStatus("info", fmt.Sprintf("Moving file to current directory: %s", dest))
		moveFile(mp4Files[0], dest)

		fi, err := os.Stat(dest)
		if err == nil {
			const mib = 1024 * 1024
			printStatus("info", fmt.Sprintf("Final file size: %s%.2f MB%s",
				colorCyan, float64(fi.Size())/mib, colorReset))
		}

		fmt.Println()
		printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, dest, colorReset))
		debugLog("videoDownload finished: output=%s", dest)
	} else {
		printStatus("warning", "No MP4 file found. The video may have been saved with a different extension.")
		debugLog("No *.mp4 found in workDir; listing workDir contents")
		if entries, e := os.ReadDir(workDir); e == nil {
			for _, entry := range entries {
				debugLog("  %s", entry.Name())
				printStatus("info", fmt.Sprintf("Found file: %s", entry.Name()))
			}
		}
	}
}

// ─── Entry Point ─────────────────────────────────────────────────────────────

func main() {
	// Strip --debug / --verbose early so other arg parsing is not affected.
	args := make([]string, 0, len(os.Args))
	for _, a := range os.Args {
		if a == "--debug" || a == "--verbose" {
			debugMode = true
		} else {
			args = append(args, a)
		}
	}

	if debugMode {
		printStatus("info", fmt.Sprintf("%sDebug mode enabled – verbose output is ON%s", colorYellow, colorReset))
		debugLog("Streamline starting up (GOOS=%s GOARCH=%s)", runtime.GOOS, runtime.GOARCH)
		debugLog("Args (after flag strip): %v", args[1:])
	}

	if len(args) == 2 && args[1] == "--about" {
		fmt.Printf("\n%s%s%s\n", colorCyan, authorTag, colorReset)
		fmt.Printf("\n%sGitHub:%s %shttps://github.com/shahil-sk/streamline%s\n\n",
			colorYellow, colorReset, colorBlue, colorReset)
		os.Exit(0)
	}
	if len(args) < 3 {
		usage()
	}

	ytdlpPath, ffmpegPath, cleanup := resolveBinaries()
	defer cleanup()

	workDir, err := os.MkdirTemp("", "streamline-work")
	check(err)
	debugLog("Temporary work directory created: %s", workDir)
	defer func() {
		debugLog("Cleaning up work directory: %s", workDir)
		os.RemoveAll(workDir)
	}()

	switch args[1] {
	case "-m":
		audioDownload(ytdlpPath, ffmpegPath, workDir, args[2])
	case "-v":
		videoDownload(ytdlpPath, ffmpegPath, workDir, args[2])
	default:
		usage()
	}
}
