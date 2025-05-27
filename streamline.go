package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

const authorTag = "Streamline by SK (Shahil Ahmed)"

// Embed static yt-dlp binary
//go:embed yt-dlp
var ytDLP []byte

// Embed static ffmpeg binary
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
	fmt.Println(`Usage:
  streamline -m <url>     Download audio with metadata and cover
  streamline -v <url>     Download video, choose quality manually
  streamline --about      Show author information

Examples:
  streamline -m https://youtube.com/watch?v=xxxx
  streamline -v https://youtube.com/watch?v=xxxx

Flags:
  -m        Music/audio mode
  -v        Video mode (you choose quality manually (example: audio only ID+video ID > (136+22)   )
  --about   Show author tag
`)
	os.Exit(0)
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

// audioDownload downloads and embeds MP3 + metadata + cover art
func audioDownload(binDir, url string) {
	runYTDLP(binDir,
		url,
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--embed-thumbnail",
		"--embed-metadata",
		"--embed-chapters",
		"--add-metadata",
		"--output", "%(title)s.%(ext)s",
	)
}

// videoDownload fetches formats, lets user choose, then downloads video
func videoDownload(binDir, url string) {
	fmt.Println("[*] Fetching available formats...")
	runYTDLP(binDir, "-F", url)
	fmt.Print("\nChoose format ID or combo (e.g. 22 or 137+140): ")
	var code string
	fmt.Scanln(&code)
	runYTDLP(binDir,
		"-f", code,
		"-o", "%(title)s.%(ext)s",
		url,
	)
}

func main() {
	fmt.Print("Streamline by sk\n")
	// Handle --about flag
	if len(os.Args) == 2 && os.Args[1] == "--about" {
		fmt.Println(authorTag)
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
