package main

import (
	_ "embed"
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

// ANSI color codes and formatting
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

//go:embed yt-dlp
var ytDLP []byte

//go:embed ffmpeg
var ffmpegBin []byte

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

func writeBin(tempDir, name string, content []byte) string {
	path := filepath.Join(tempDir, name)
	check(os.WriteFile(path, content, 0755))
	return path
}

// Progress bar component
type ProgressBar struct {
	total       float64
	current     float64
	width       int
	description string
	startTime   time.Time
	lastUpdate  time.Time
}

func NewProgressBar(description string, width int) *ProgressBar {
	return &ProgressBar{
		description: description,
		width:       width,
		startTime:   time.Now(),
		lastUpdate:  time.Now(),
	}
}

func (p *ProgressBar) Update(current, total float64) {
	p.current = current
	p.total = total
	
	// Throttle updates to every 100ms
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
	
	// Build progress bar
	bar := strings.Repeat("█", filled)
	empty := strings.Repeat("░", p.width-filled)
	
	// Calculate speed and ETA
	elapsed := time.Since(p.startTime).Seconds()
	if elapsed < 0.1 {
		elapsed = 0.1
	}
	speed := p.current / elapsed
	remaining := 0.0
	if speed > 0 {
		remaining = (p.total - p.current) / speed
	}
	
	// Format sizes
	currentMB := p.current / (1024 * 1024)
	totalMB := p.total / (1024 * 1024)
	speedMB := speed / (1024 * 1024)
	
	fmt.Printf("\r%s%s%s %s%s%s%s%s │ %s%.1f%%%s │ %s%.2f/%.2f MB%s │ %s%.2f MB/s%s │ ETA: %s%s%s    ",
		colorBold, p.description, colorReset,
		colorGreen, bar, colorDim, empty, colorReset,
		colorCyan, percent, colorReset,
		colorYellow, currentMB, totalMB, colorReset,
		colorBlue, speedMB, colorReset,
		colorGreen, formatDuration(remaining), colorReset)
}

func (p *ProgressBar) Complete() {
	if p.total > 0 {
		p.current = p.total
		p.Render()
	}
	fmt.Println()
}

// Spinner for indeterminate progress
type Spinner struct {
	frames  []string
	index   int
	message string
	active  bool
	stop    chan bool
}

func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		message: message,
		stop:    make(chan bool),
	}
}

func (s *Spinner) Start() {
	s.active = true
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				if s.active {
					fmt.Printf("\r%s%s%s %s   ", colorCyan, s.frames[s.index], colorReset, s.message)
					s.index = (s.index + 1) % len(s.frames)
				}
			}
		}
	}()
}

func (s *Spinner) Stop(success bool) {
	s.active = false
	s.stop <- true
	time.Sleep(100 * time.Millisecond) // Give time for goroutine to stop
	
	icon := "✓"
	color := colorGreen
	if !success {
		icon = "✗"
		color = colorRed
	}
	
	fmt.Printf("\r%s%s%s %s\n", color, icon, colorReset, s.message)
}

func formatDuration(seconds float64) string {
	if seconds < 0 || seconds > 86400 {
		return "--:--"
	}
	
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	
	if minutes > 60 {
		hours := minutes / 60
		minutes = minutes % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func usage() {
	fmt.Printf(`
%s╔═════════════════════════════════════════════╗
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
// 	banner := `
//   ╔═══════════════════╗
//   ║  Streamline by SK ║
//   ╚═══════════════════╝
// `
	banner:= `
╔═════════════════════════════════════════════╗
║ Streamline - YouTube/SoundCloud Downloader  ║
╚═════════════════════════════════════════════╝
	`
	fmt.Printf("%s%s%s\n", colorCyan, banner, colorReset)
}

func printStatus(status, message string) {
	icons := map[string]string{
		"info":    "ℹ",
		"success": "✓",
		"warning": "⚠",
		"error":   "✗",
	}
	colors := map[string]string{
		"info":    colorBlue,
		"success": colorGreen,
		"warning": colorYellow,
		"error":   colorRed,
	}
	
	icon := icons[status]
	color := colors[status]
	if icon == "" {
		icon = "•"
	}
	if color == "" {
		color = colorReset
	}
	
	fmt.Printf("%s%s%s %s\n", color, icon, colorReset, message)
}

func parseSize(sizeStr string) float64 {
	// Remove any whitespace
	sizeStr = strings.TrimSpace(sizeStr)
	
	// Parse sizes like "12.34MiB" or "1.2GiB" or "12.34M" or "1.2G"
	re := regexp.MustCompile(`([\d.]+)\s*([KMGT]i?B?)`)
	matches := re.FindStringSubmatch(sizeStr)
	
	if len(matches) < 3 {
		return 0
	}
	
	value, _ := strconv.ParseFloat(matches[1], 64)
	unit := strings.ToUpper(matches[2])
	
	// Normalize unit
	if len(unit) == 1 {
		unit = unit + "B"
	}
	
	multipliers := map[string]float64{
		"B":   1,
		"KB":  1024,
		"KIB": 1024,
		"MB":  1024 * 1024,
		"MIB": 1024 * 1024,
		"GB":  1024 * 1024 * 1024,
		"GIB": 1024 * 1024 * 1024,
		"TB":  1024 * 1024 * 1024 * 1024,
		"TIB": 1024 * 1024 * 1024 * 1024,
	}
	
	mult, ok := multipliers[unit]
	if !ok {
		mult = multipliers[unit[:len(unit)-1]+"IB"]
	}
	
	return value * mult
}

func runYTDLPWithProgress(binDir string, description string, args ...string) {
	// Add progress flags
	args = append(args, "--newline", "--progress")
	
	cmd := exec.Command(filepath.Join(binDir, "yt-dlp"), args...)
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
	
	stdout, err := cmd.StdoutPipe()
	check(err)
	
	stderr, err := cmd.StderrPipe()
	check(err)
	
	check(cmd.Start())
	
	var progressBar *ProgressBar
	
	// Read both stdout and stderr
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	
	// Regex patterns for different output formats
	progressRegex := regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?\s*([\d.]+\s*[KMGT]i?B)`)
	progressRegex2 := regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)
	sizeRegex := regexp.MustCompile(`of\s+~?\s*([\d.]+\s*[KMGT]i?B)`)
	
	var totalSize float64
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Check for download progress
		if strings.Contains(line, "[download]") {
			// Try to extract total size if not yet known
			if totalSize == 0 {
				if matches := sizeRegex.FindStringSubmatch(line); len(matches) >= 2 {
					totalSize = parseSize(matches[1])
				}
			}
			
			// Try first pattern: [download]  45.2% of 12.34MiB
			if matches := progressRegex.FindStringSubmatch(line); len(matches) >= 3 {
				percent, _ := strconv.ParseFloat(matches[1], 64)
				total := parseSize(matches[2])
				
				if total > 0 {
					totalSize = total
					current := total * (percent / 100)
					
					if progressBar == nil {
						progressBar = NewProgressBar(description, 40)
					}
					progressBar.Update(current, total)
				}
			} else if matches := progressRegex2.FindStringSubmatch(line); len(matches) >= 2 {
				// Try second pattern: just percentage
				percent, _ := strconv.ParseFloat(matches[1], 64)
				
				if totalSize > 0 {
					current := totalSize * (percent / 100)
					if progressBar == nil {
						progressBar = NewProgressBar(description, 40)
					}
					progressBar.Update(current, totalSize)
				}
			}
			
			// Check for destination
			if strings.Contains(line, "Destination:") {
				if progressBar != nil {
					progressBar.Complete()
					progressBar = nil
				}
				filename := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
				printStatus("info", fmt.Sprintf("File: %s", filename))
			} else if strings.Contains(line, "has already been downloaded") {
				printStatus("warning", "File already exists, skipping...")
			} else if strings.Contains(line, "100%") || strings.Contains(line, "download completed") {
				if progressBar != nil {
					progressBar.Complete()
					progressBar = nil
				}
			}
		} else if strings.Contains(line, "Merging formats") {
			if progressBar != nil {
				progressBar.Complete()
				progressBar = nil
			}
			printStatus("info", "Merging video and audio streams...")
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

func embedThumbnail(binDir, mp3File, thumbFile string) {
	spinner := NewSpinner("Embedding thumbnail into audio file...")
	spinner.Start()
	
	tempFile := mp3File + ".temp"
	cmd := exec.Command(filepath.Join(binDir, "ffmpeg"),
		"-i", mp3File,
		"-i", thumbFile,
		"-map", "0:0",
		"-map", "1:0",
		"-c", "copy",
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

func audioDownload(binDir, url string) {
	printBanner()
	
	spinner := NewSpinner("Fetching video information...")
	spinner.Start()
	time.Sleep(500 * time.Millisecond)
	spinner.Stop(true)
	
	printStatus("info", "Starting audio download...")
	fmt.Println()
	
	runYTDLPWithProgress(binDir, "Downloading audio",
		url,
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--convert-thumbnails", "jpg",
		"--embed-metadata",
		"--embed-chapters",
		"--add-metadata",
		"-o", "%(title)s.%(ext)s",
		"--write-thumbnail")

	matches, err := filepath.Glob("*.mp3")
	check(err)
	if len(matches) == 0 {
		printStatus("error", "No MP3 file found")
		os.Exit(1)
	}

	mp3File := matches[0]
	thumbMatches, err := filepath.Glob("*.jpg")
	check(err)
	
	if len(thumbMatches) > 0 {
		thumbFile := thumbMatches[0]
		embedThumbnail(binDir, mp3File, thumbFile)
		os.Remove(thumbFile)
	}
	
	fmt.Println()
	printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, mp3File, colorReset))
}

func videoDownload(binDir, url string) {
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

	var format string
	if choice > 0 && choice < len(presets) {
		format = presets[choice-1].format
	} else if choice == len(presets) {
		spinner := NewSpinner("Fetching available formats...")
		spinner.Start()
		
		cmd := exec.Command(filepath.Join(binDir, "yt-dlp"), "-F", url)
		cmd.Env = append(os.Environ(), "PATH="+binDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
		output, err := cmd.CombinedOutput()
		
		spinner.Stop(err == nil)
		
		if err == nil {
			fmt.Println(string(output))
		}

		fmt.Printf("\n%sEnter format ID or combination (e.g., 137+140):%s ", colorCyan, colorReset)
		fmt.Scanln(&format)
		fmt.Println()
	} else {
		printStatus("warning", "Invalid choice, using best quality")
		format = "bestvideo+bestaudio/best"
	}

	printStatus("info", "Starting video download...")
	fmt.Println()
	
	runYTDLPWithProgress(binDir, "Downloading video",
		"-f", format,
		"-o", "%(title)s.%(ext)s",
		url)

	matches, _ := filepath.Glob("*.mp4")
	if len(matches) > 0 {
		fmt.Println()
		printStatus("success", fmt.Sprintf("✨ Successfully downloaded: %s%s%s", colorBold, matches[0], colorReset))
	}
}

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

	tempDir, err := os.MkdirTemp("", "streamline")
	check(err)
	defer os.RemoveAll(tempDir)

	binDir := filepath.Join(tempDir, "bin")
	check(os.Mkdir(binDir, 0755))

	writeBin(binDir, "yt-dlp", ytDLP)
	writeBin(binDir, "ffmpeg", ffmpegBin)

	cmd, url := os.Args[1], os.Args[2]
	switch cmd {
	case "-m":
		audioDownload(binDir, url)
	case "-v":
		videoDownload(binDir, url)
	default:
		usage()
	}
}
