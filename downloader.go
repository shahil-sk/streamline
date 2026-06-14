package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Package-level precompiled regexes – compiled once at startup, not per call
var (
	reProgressFull = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?\s*([\d.]+\s*[KMGT]i?B?)`)
	reProgressPct  = regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)
	reSizeExtract  = regexp.MustCompile(`of\s+~?\s*([\d.]+\s*[KMGT]i?B?)`)
	reParseSize    = regexp.MustCompile(`([\d.]+)\s*([KMGT]?i?B?)`)
)

func runYTDLPWithProgress(ytdlpPath, ffmpegDir, description string, quiet bool, args ...string) {
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
			if strings.Contains(line, "Merging formats") ||
				strings.Contains(line, "[ExtractAudio]") ||
				strings.Contains(line, "[SponsorBlock]") ||
				strings.Contains(line, "[Metadata]") ||
				strings.Contains(line, "[ModifyChapters]") ||
				strings.Contains(line, "[Fixup") {

				if progressBar != nil {
					progressBar.Complete()
					progressBar = nil
				}

				if !quiet {
					if strings.Contains(line, "Merging formats") {
						printStatus("info", "Merging streams...")
					} else if strings.Contains(line, "[ExtractAudio]") {
						printStatus("info", "Extracting audio...")
					} else if strings.Contains(line, "[SponsorBlock]") {
						printStatus("info", "Processing SponsorBlock...")
					} else if strings.Contains(line, "[Metadata]") {
						printStatus("info", "Writing metadata...")
					} else if strings.Contains(line, "[ModifyChapters]") {
						printStatus("info", "Writing chapters...")
					} else if strings.Contains(line, "[Fixup") {
						printStatus("info", "Fixing container...")
					}
				}
			}
			continue
		}

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
			totalSize = 0
			if !quiet {
				filename := strings.TrimSpace(strings.TrimPrefix(line, "[download] Destination:"))
				printStatus("info", "File: "+filepath.Base(filename))
			}

		case strings.Contains(line, "has already been downloaded"):
			if progressBar != nil {
				progressBar.Complete()
				progressBar = nil
			}
			totalSize = 0
			if !quiet {
				printStatus("warning", "File already exists, skipping...")
			}

		default:
			if quiet {
				continue
			}
			if m := reProgressFull.FindStringSubmatch(line); len(m) >= 3 {
				pct, _ := strconv.ParseFloat(m[1], 64)
				total := parseSize(m[2])
				if total > 0 {
					totalSize = total
					if progressBar == nil {
						progressBar = NewProgressBar(description, 40)
					}
					progressBar.Update(total*(pct/100), total)
				}
			} else if m := reProgressPct.FindStringSubmatch(line); len(m) >= 2 && totalSize > 0 {
				pct, _ := strconv.ParseFloat(m[1], 64)
				if progressBar == nil {
					progressBar = NewProgressBar(description, 40)
				}
				progressBar.Update(totalSize*(pct/100), totalSize)
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
		return
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

func audioDownload(ytdlpPath, ffmpegPath, workDir, url, outDir, proxyURL string, quiet, sponsorBlock, sponsorMark bool, sponsorCats, start, end, playlistItems, cookies string) {
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

	if !quiet && !sponsorBlock && !sponsorMark {
		fmt.Printf("%s┌─ SponsorBlock ──────────────────────────────┐%s\n", colorYellow, colorReset)
		sbOptions := []string{
			"None (Default)",
			"Remove sponsor segments",
			"Mark sponsor segments as chapters",
		}
		for i, opt := range sbOptions {
			fmt.Printf("%s│%s %s%d.%s %-40s %s│%s\n",
				colorYellow, colorReset,
				colorGreen, i+1, colorReset,
				opt,
				colorYellow, colorReset)
		}
		fmt.Printf("%s└─────────────────────────────────────────────┘%s\n\n", colorYellow, colorReset)

		sbInput := readInput(fmt.Sprintf("%sChoose SponsorBlock option (1-%d) [default: 1]:%s ", colorCyan, len(sbOptions), colorReset))
		sbChoice, _ := strconv.Atoi(sbInput)
		if sbChoice == 2 {
			sponsorBlock = true
		} else if sbChoice == 3 {
			sponsorMark = true
		}
		fmt.Println()
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
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", audioFmt,
		"--convert-thumbnails", "jpg",
		"--embed-metadata",
		"--embed-chapters",
		"--add-metadata",
		"-o", filepath.Join(workDir, "%(title)s.%(ext)s"),
		"--write-thumbnail",
		"--ffmpeg-location", ffmpegPath,
	}
	if audioQuality != "" {
		args = append(args, "--audio-quality", audioQuality)
	}
	if sponsorBlock {
		args = append(args, "--sponsorblock-remove", sponsorCats)
		args = append(args, "--force-keyframes-at-cuts")
	} else if sponsorMark {
		args = append(args, "--sponsorblock-mark", sponsorCats)
	}
	if proxyURL != "" {
		args = append(args, "--proxy", proxyURL)
	}
	if cookies != "" {
		args = append(args, "--cookies-from-browser", cookies)
	}
	if start != "" || end != "" {
		if start == "" {
			start = "0"
		}
		if end == "" {
			end = "inf"
		}
		args = append(args, "--download-sections", fmt.Sprintf("*%s-%s", start, end))

		args = append(args, "--force-keyframes-at-cuts")
	}
	if playlistItems != "" {
		args = append(args, "--playlist-items", playlistItems)
	}
	args = append(args, url)

	runYTDLPWithProgress(ytdlpPath, ffmpegDir, "Downloading audio", quiet, args...)

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
		printStatus("success", fmt.Sprintf("Successfully downloaded: %s%s%s", colorBold, destName, colorReset))
	}
}

func videoDownload(ytdlpPath, ffmpegPath, workDir, url, outDir, proxyURL string, quiet, sponsorBlock, sponsorMark bool, sponsorCats string, subtitles bool, start, end, playlistItems, cookies string) {
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
			args := []string{"-F", url}
			if proxyURL != "" {
				args = append([]string{"--proxy", proxyURL}, args...)
			}
			cmd := exec.Command(ytdlpPath, args...)
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

		if !sponsorBlock && !sponsorMark {
			fmt.Printf("%s┌─ SponsorBlock ──────────────────────────────┐%s\n", colorYellow, colorReset)
			sbOptions := []string{
				"None (Default)",
				"Remove sponsor segments",
				"Mark sponsor segments as chapters",
			}
			for i, opt := range sbOptions {
				fmt.Printf("%s│%s %s%d.%s %-40s %s│%s\n",
					colorYellow, colorReset,
					colorGreen, i+1, colorReset,
					opt,
					colorYellow, colorReset)
			}
			fmt.Printf("%s└─────────────────────────────────────────────┘%s\n\n", colorYellow, colorReset)

			sbInput := readInput(fmt.Sprintf("%sChoose SponsorBlock option (1-%d) [default: 1]:%s ", colorCyan, len(sbOptions), colorReset))
			sbChoice, _ := strconv.Atoi(sbInput)
			if sbChoice == 2 {
				sponsorBlock = true
			} else if sbChoice == 3 {
				sponsorMark = true
			}
			fmt.Println()
		}
	}

	printStatus("info", "Starting video download...")
	if !quiet {
		fmt.Println()
	}

	args := []string{
		"-f", format,
		"-o", filepath.Join(workDir, "%(title)s.%(ext)s"),
		"--ffmpeg-location", ffmpegPath,
	}
	if sponsorBlock {
		args = append(args, "--sponsorblock-remove", sponsorCats)
		args = append(args, "--force-keyframes-at-cuts")
	} else if sponsorMark {
		args = append(args, "--sponsorblock-mark", sponsorCats)
	}
	if subtitles {
		args = append(args, "--write-auto-subs", "--write-subs", "--embed-subs", "--sub-langs", "all,-live_chat")
	}
	if proxyURL != "" {
		args = append(args, "--proxy", proxyURL)
	}
	if cookies != "" {
		args = append(args, "--cookies-from-browser", cookies)
	}
	if start != "" || end != "" {
		if start == "" {
			start = "0"
		}
		if end == "" {
			end = "inf"
		}
		args = append(args, "--download-sections", fmt.Sprintf("*%s-%s", start, end))
		args = append(args, "--force-keyframes-at-cuts")
	}
	if playlistItems != "" {
		args = append(args, "--playlist-items", playlistItems)
	}
	args = append(args, url)

	runYTDLPWithProgress(ytdlpPath, ffmpegDir, "Downloading video", quiet, args...)

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
		printStatus("success", fmt.Sprintf("Successfully downloaded: %s%s%s", colorBold, destName, colorReset))
		downloadedCount++
	}

	if downloadedCount == 0 {
		exitWithError("No video files downloaded")
	}
}
