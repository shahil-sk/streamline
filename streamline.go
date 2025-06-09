package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	authorTag = "Streamline by SK (Shahil Ahmed)"
	// ANSI color codes
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
)

// Embed static yt-dlp binary
//
//go:embed yt-dlp
var ytDLP []byte

// Embed static ffmpeg binary
//
//go:embed ffmpeg
var ffmpegBin []byte

// check is a helper to exit on error
func check(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

// writeBin writes embedded binaries to a temp file and returns the path
func writeBin(tempDir, name string, content []byte) string {
	path := filepath.Join(tempDir, name)
	check(ioutil.WriteFile(path, content, 0755))
	return path
}

// usage prints help and exits
func usage() {
	fmt.Printf(`%sStreamline%s - YouTube Downloader with Style

%sUsage:%s
  %sstreamline -m <url>%s     Download audio with metadata and cover
  %sstreamline -v <url>%s     Download video, choose quality manually
  %sstreamline --about%s      Show author information

%sExamples:%s
  %sstreamline -m https://youtube.com/watch?v=xxxx%s
  %sstreamline -v https://youtube.com/watch?v=xxxx%s

%sFlags:%s
  %s-m%s        Music/audio mode
  %s-v%s        Video mode (you choose quality manually)
  %s--about%s   Show author tag
%s`,
		colorCyan, colorReset,
		colorYellow, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorBlue, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorYellow, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset)
	os.Exit(0)
}

// printStatus prints a status message with color
func printStatus(status, message string) {
	var color string
	switch status {
	case "info":
		color = colorBlue
	case "success":
		color = colorGreen
	case "warning":
		color = colorYellow
	case "error":
		color = colorRed
	default:
		color = colorReset
	}
	fmt.Printf("%s[%s]%s %s\n", color, status, colorReset, message)
}

// runYTDLP runs yt-dlp with environment configured to use embedded ffmpeg
func runYTDLP(binDir string, args ...string) {
	cmd := exec.Command(filepath.Join(binDir, "yt-dlp"), args...)
	// Ensure ffmpeg is found by yt-dlp
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	check(cmd.Run())
}

// embedThumbnail embeds the thumbnail into the MP3 file using ffmpeg
func embedThumbnail(binDir, mp3File, thumbFile string) {
	cmd := exec.Command(filepath.Join(binDir, "ffmpeg"),
		"-i", mp3File,
		"-i", thumbFile,
		"-map", "0:0",
		"-map", "1:0",
		"-c", "copy",
		"-id3v2_version", "3",
		"-metadata:s:v", "title=Album cover",
		"-metadata:s:v", "comment=Cover (front)",
		"-f", "mp3",
		mp3File+".temp")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	check(cmd.Run())

	// Replace original file with the one containing embedded thumbnail
	check(os.Rename(mp3File+".temp", mp3File))
}

// audioDownload downloads and embeds MP3 + metadata + cover art
func audioDownload(binDir, url string) {
	fmt.Printf("\n%s%s%s\n\n", colorCyan, "Streamline by SK", colorReset)
	printStatus("info", "Fetching available formats...")

	// First download the audio and thumbnail separately
	runYTDLP(binDir,
		url,
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--convert-thumbnails", "jpg",
		"--postprocessor-args", "-vf scale=w=800:h=800:force_original_aspect_ratio=increase,crop=800:800",
		"--embed-metadata",
		"--embed-chapters",
		"--add-metadata",
		"--output", "%(title)s.%(ext)s",
		"--write-thumbnail",
	)

	// Get the downloaded files
	matches, err := filepath.Glob("*.mp3")
	check(err)
	if len(matches) == 0 {
		printStatus("error", "No MP3 file found")
		os.Exit(1)
	}
	mp3File := matches[0]

	thumbMatches, err := filepath.Glob("*.jpg")
	check(err)
	if len(thumbMatches) == 0 {
		printStatus("error", "No thumbnail file found")
		os.Exit(1)
	}
	thumbFile := thumbMatches[0]

	printStatus("info", "Embedding thumbnail...")
	embedThumbnail(binDir, mp3File, thumbFile)

	// Clean up the thumbnail file
	os.Remove(thumbFile)
	printStatus("success", fmt.Sprintf("Successfully downloaded and processed: %s", mp3File))
}

// videoDownload fetches formats, lets user choose, then downloads video
func videoDownload(binDir, url string) {
	fmt.Printf("\n%s%s%s\n\n", colorCyan, "Streamline by SK", colorReset)
	printStatus("info", "Fetching available formats...")
	runYTDLP(binDir, "-F", url)

	fmt.Printf("\n%s%s%s\n\n", colorRed, "Streamline by SK", colorReset)
	fmt.Printf("%s%s%s", colorCyan, "Choose a Video ID and Audio ID", colorReset)
	fmt.Printf("\n%sChoose format ID or combo (e.g. 22 or 137+140):%s ", colorYellow, colorReset)
	var code string
	fmt.Scanln(&code)

	printStatus("info", "Downloading video...")
	runYTDLP(binDir,
		"-f", code,
		"-o", "%(title)s.%(ext)s",
		url,
	)

	// Find the downloaded file
	matches, err := filepath.Glob("*.mp4")
	check(err)
	if len(matches) > 0 {
		printStatus("success", fmt.Sprintf("Successfully downloaded: %s", matches[0]))
	}
}

func main() {
	// Handle --about flag
	if len(os.Args) == 2 && os.Args[1] == "--about" {
		fmt.Printf("\n%s%s%s\n", colorCyan, authorTag, colorReset)
		fmt.Printf("\n%sGitHub:%s %shttps://github.com/shahil-sk/streamline%s\n\n",
			colorYellow, colorReset,
			colorBlue, colorReset)
		os.Exit(0)
	}

	// Must have at least a flag and a URL
	if len(os.Args) < 3 {
		usage()
	}

	// Create temporary dir to extract yt-dlp and ffmpeg
	tempDir, err := ioutil.TempDir("", "streamline")
	check(err)
	defer os.RemoveAll(tempDir) // Auto-delete after run

	// Create subdirectory to store embedded binaries
	binDir := filepath.Join(tempDir, "bin")
	check(os.Mkdir(binDir, 0755))

	// Write embedded binaries to temp folder
	writeBin(binDir, "yt-dlp", ytDLP)
	writeBin(binDir, "ffmpeg", ffmpegBin)

	// Parse command and URL
	cmd := os.Args[1]
	url := os.Args[2]

	switch cmd {
	case "-m":
		audioDownload(binDir, url)
	case "-v":
		videoDownload(binDir, url)
	default:
		usage()
	}
}
