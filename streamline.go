package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed yt-dlp
var ytDLP []byte

//go:embed ffmpeg
var ffmpegBin []byte

func check(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func writeBin(tempDir, name string, content []byte) string {
	path := filepath.Join(tempDir, name)
	check(ioutil.WriteFile(path, content, 0755))
	return path
}

func usage() {
	fmt.Println(`Usage:
  streamline -m <url>    Download audio with metadata and cover
  streamline -v <url>    Download video, choose quality`)
	os.Exit(1)
}

func runYTDLP(binDir string, args ...string) {
	cmd := exec.Command(filepath.Join(binDir, "yt-dlp"), args...)
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	check(cmd.Run())
}

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

func videoDownload(binDir, url string) {
	fmt.Println("[*] Fetching formats...")
	runYTDLP(binDir, "-F", url)
	fmt.Print("\nChoose 2 IDs (audio only+mp4) Shown eg: 22, 174+233: ")
	var code string
	fmt.Scanln(&code)
	runYTDLP(binDir,
		"-f", code,
		"-o", "%(title)s.%(ext)s",
		url,
	)
}

func main() {
	if len(os.Args) < 3 {
		usage()
	}

	tempDir, err := ioutil.TempDir("", "streamline")
	check(err)
	defer os.RemoveAll(tempDir)

	binDir := filepath.Join(tempDir, "bin")
	check(os.Mkdir(binDir, 0755))

	writeBin(binDir, "yt-dlp", ytDLP)
	writeBin(binDir, "ffmpeg", ffmpegBin)

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
