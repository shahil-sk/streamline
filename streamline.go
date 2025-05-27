package main

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
  streamline -m <url>    Download audio with metadata and perfectly cropped cover art
  streamline -v <url>    Download video, choose quality`)
	os.Exit(1)
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runYTDLP(binDir string, args ...string) {
	cmd := exec.Command(filepath.Join(binDir, "yt-dlp"), args...)
	// Add binDir to PATH so yt-dlp finds ffmpeg
	cmd.Env = append(os.Environ(), "PATH="+binDir+":"+os.Getenv("PATH"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	check(cmd.Run())
}

func audioDownloadWithCroppedCover(binDir, url string) {
	tempDir, err := ioutil.TempDir("", "streamline-thumb")
	check(err)
	defer os.RemoveAll(tempDir)

	fmt.Println("[*] Downloading thumbnail only...")
	err = runCmd(
		filepath.Join(binDir, "yt-dlp"),
		url,
		"--skip-download",
		"--write-thumbnail",
		"--output", filepath.Join(tempDir, "thumb.%(ext)s"),
	)
	check(err)

	files, err := ioutil.ReadDir(tempDir)
	check(err)

	var thumbPath string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "thumb.") {
			thumbPath = filepath.Join(tempDir, f.Name())
			break
		}
	}
	if thumbPath == "" {
		fmt.Println("Thumbnail not found!")
		os.Exit(1)
	}

	croppedThumb := filepath.Join(tempDir, "thumb_cropped.jpg")
	fmt.Println("[*] Cropping and resizing thumbnail to 300x300...")
	err = runCmd(
		filepath.Join(binDir, "ffmpeg"),
		"-y",
		"-i", thumbPath,
		"-vf", "crop='min(iw,ih)':'min(iw,ih)',scale=300:300",
		"-q:v", "2",
		croppedThumb,
	)
	check(err)

	fmt.Println("[*] Downloading audio (no embedded thumbnail)...")
	err = runCmd(
		filepath.Join(binDir, "yt-dlp"),
		url,
		"-f", "bestaudio",
		"--extract-audio",
		"--audio-format", "mp3",
		"--embed-metadata",
		"--embed-chapters",
		"--add-metadata",
		"--output", "%(title)s.%(ext)s",
		"--no-embed-thumbnail",
	)
	check(err)

	fmt.Println("[*] Finding downloaded mp3 file...")
	audioFiles, err := filepath.Glob("*.mp3")
	check(err)
	if len(audioFiles) == 0 {
		fmt.Println("Audio file not found!")
		os.Exit(1)
	}
	audioFile := audioFiles[0]

	finalAudio := "final_" + audioFile
	fmt.Println("[*] Embedding cropped cover art into audio file...")
	err = runCmd(
		filepath.Join(binDir, "ffmpeg"),
		"-y",
		"-i", audioFile,
		"-i", croppedThumb,
		"-map", "0:0",
		"-map", "1:0",
		"-c", "copy",
		"-id3v2_version", "3",
		"-metadata:s:v", "title=\"Album cover\"",
		"-metadata:s:v", "comment=\"Cover (front)\"",
		finalAudio,
	)
	check(err)

	err = os.Rename(finalAudio, audioFile)
	check(err)

	fmt.Println("✔ Audio downloaded with perfectly cropped and embedded album cover!")
}

func videoDownload(binDir, url string) {
	fmt.Println("[*] Fetching video formats...")
	runYTDLP(binDir, "-F", url)
	fmt.Print("Enter format code: ")
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
		audioDownloadWithCroppedCover(binDir, url)
	case "-v":
		videoDownload(binDir, url)
	default:
		usage()
	}
}
