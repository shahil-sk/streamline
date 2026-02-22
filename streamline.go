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

func check(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s✗ Error:%s %v\n", colorRed, colorReset, err)
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
  streamline -m <url>    Download audio with metadata and cover
  streamline -v <url>    Download video, choose quality manually
  streamline --about     Show author information

%sExamples:%s
  streamline -m https://youtube.com/watch?v=xxxxx
  streamline -v https://youtu.be/xxxxx

%sFlags:%s
  %s-m%s        Music/audio mode (MP3 + metadata + cover art)
  %s-v%s        Video mode (quality selection)
  %s--about%s   Author information

`,
		colorCyan, colorBold, colorReset, colorReset,
		colorYellow, colorReset,
		colorYellow, colorReset,
		colorYellow, colorReset,
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
	)

	for scanner.Scan() {
		line := scanner.Text()

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
			filename := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
			printStatus("info", "File: "+filename)

		case strings.Contains(line, "has already been downloaded"):
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
		fmt.Fprintf(os.Stderr, "%s✗ Download failed:%s %v\n", colorRed, colorReset, err)
		os.Exit(1)
	}
}

// embedThumbnail crops the thumbnail to a square, scales it to 500x500,
// and embeds it into the MP3 as ID3v2 cover art.
//
// Root cause of the old bug: "-c copy" stream-copied the raw 16:9 JPEG
// with zero processing, so the image was never cropped.
//
// Fix:
//   - "-c:a copy"  – copy the audio stream unchanged
//   - "-c:v mjpeg" – re-encode the thumbnail so filters can run
//   - "-vf crop=min(iw\,ih):min(iw\,ih),scale=500:500"
//       crop= trims the wider dimension to match the shorter one (square)
//       scale= normalises the result to 500×500 px
//   - "-q:v 2"     – high-quality JPEG output (scale 1–31, lower = better)
func embedThumbnail(ffmpegPath, mp3File, thumbFile string) {
	printStatus("info", "Cropping thumbnail to square and embedding...")
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
		// crop= uses min(iw\,ih) – the backslash escapes the comma from
		// ffmpeg's filter-chain parser so it is treated as a function
		// argument separator inside min(), not a filter separator.
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
	check(err)
	check(os.Rename(tempFile, mp3File))
}

// copyFile copies src to dst byte-for-byte (cross-device fallback for os.Rename)
func copyFile(src, dst string) {
	in, err := os.Open(src)
	check(err)
	defer in.Close()
	out, err := os.Create(dst)
	check(err)
	defer out.Close()
	_, err = io.Copy(out, in)
	check(err)
}

// moveFile renames src to dst, falling back to copy+delete on cross-device moves
func moveFile(src, dst string) {
	if err := os.Rename(src, dst); err != nil {
		copyFile(src, dst)
		os.Remove(src)
	}
}

// ─── Download Commands ───────────────────────────────────────────────────────

func audioDownload(ytdlpPath, ffmpegPath, workDir, url string) {
	printBanner()
	spinner := NewSpinner("Fetching video information...")
	spinner.Start()
	time.Sleep(500 * time.Millisecond)
	spinner.Stop(true)

	printStatus("info", "Starting audio download...")
	fmt.Println()

	ffmpegDir := filepath.Dir(ffmpegPath)
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
	if len(mp3Files) == 0 {
		printStatus("error", "No MP3 file found")
		os.Exit(1)
	}

	mp3File := mp3Files[0]
	if thumbFiles, _ := filepath.Glob(filepath.Join(workDir, "*.jpg")); len(thumbFiles) > 0 {
		embedThumbnail(ffmpegPath, mp3File, thumbFiles[0])
		os.Remove(thumbFiles[0])
	}

	dest := filepath.Base(mp3File)
	moveFile(mp3File, dest)

	fmt.Println()
	printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, dest, colorReset))
}

func videoDownload(ytdlpPath, ffmpegPath, workDir, url string) {
	printBanner()

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

	fmt.Printf("%sChoose quality (1-6):%s ", colorCyan, colorReset)
	var choice int
	fmt.Scanln(&choice)
	fmt.Println()

	ffmpegDir := filepath.Dir(ffmpegPath)
	var format string

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
		if err == nil {
			fmt.Println(string(output))
		}
		fmt.Printf("\n%sEnter format ID or combination (e.g., 137+140):%s ", colorCyan, colorReset)
		fmt.Scanln(&format)
		fmt.Println()
	default:
		printStatus("warning", "Invalid choice, using best quality")
		format = "bestvideo+bestaudio/best"
	}

	printStatus("info", "Starting video download...")
	fmt.Println()

	runYTDLPWithProgress(ytdlpPath, ffmpegDir, "Downloading video",
		"-f", format,
		"-o", filepath.Join(workDir, "%(title)s.%(ext)s"),
		url)

	if mp4Files, _ := filepath.Glob(filepath.Join(workDir, "*.mp4")); len(mp4Files) > 0 {
		dest := filepath.Base(mp4Files[0])
		moveFile(mp4Files[0], dest)
		fmt.Println()
		printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, dest, colorReset))
	}
}

// ─── Entry Point ─────────────────────────────────────────────────────────────

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--about" {
		fmt.Printf("\n%s%s%s\n", colorCyan, authorTag, colorReset)
		fmt.Printf("\n%sGitHub:%s %shttps://github.com/shahil-sk/streamline%s\n\n",
			colorYellow, colorReset, colorBlue, colorReset)
		os.Exit(0)
	}
	if len(os.Args) < 3 {
		usage()
	}

	ytdlpPath, ffmpegPath, cleanup := resolveBinaries()
	defer cleanup()

	// Isolated work dir prevents glob from accidentally matching files in the user's CWD
	workDir, err := os.MkdirTemp("", "streamline-work")
	check(err)
	defer os.RemoveAll(workDir)

	switch os.Args[1] {
	case "-m":
		audioDownload(ytdlpPath, ffmpegPath, workDir, os.Args[2])
	case "-v":
		videoDownload(ytdlpPath, ffmpegPath, workDir, os.Args[2])
	default:
		usage()
	}
}
