package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const authorTag = "Streamline by SK (Shahil Ahmed)"
const releasesURL = "https://github.com/shahil-sk/streamline/releases/latest"

func usage() {
	fmt.Printf(`%s   _____ __                               ___          
  / ___// /_________  ____ _____ ___     / (_)___  ___ 
  \__ \/ __/ ___/ _ \/ __ '/ __ '__ \   / / / __ \/ _ \
 ___/ / /_/ /  /  __/ /_/ / / / / / /  / / / / / /  __/
/____/\__/_/   \___/\__,_/_/ /_/ /_/  /_/_/_/ /_/\___/ %s
      %sUniversal Media Downloader (1000+ Sites)%s

%sUsage:%s
  streamline -m [flags] <url>    Download audio
  streamline -v [flags] <url>    Download video

%sExamples:%s
  streamline -m https://youtube.com/watch?v=xxxxx
  streamline -v -q -o ~/Downloads https://youtu.be/xxxxx

%sFlags:%s

  [ Core ]
  %s-m%s          Music/audio mode
  %s-v%s          Video mode
  %s-o%s          Output directory (default: current directory)
  %s-q%s          Quiet mode (skip prompts, use best quality)
  %s--about%s     Author information

  [ Media Processing ]
  %s--subs%s      Embed subtitles (video only)
  %s--start%s     Start timestamp for clipping (e.g. 01:00)
  %s--end%s       End timestamp for clipping (e.g. 02:30)

  [ SponsorBlock ]
  %s-s%s          Remove sponsor segments
  %s--sp-mark%s   Mark sponsor segments as chapters instead of removing
  %s--sp-cats%s   SponsorBlock categories (default: "default")

  [ Batch & Playlist ]
  %s--batch%s     File containing URLs to download
  %s-j%s          Number of concurrent downloads (default: 1)
  %s--select%s    Interactive playlist item selector (TUI)

  [ Network & Auth ]
  %s--cookies%s   Extract cookies from browser (e.g. chrome, firefox)
  %s--dns%s       Bypass system DNS via custom server or DoH endpoint

`,
		colorCyan, colorReset, colorYellow, colorReset,
		colorYellow, colorReset,
		colorYellow, colorReset,
		// Core
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		// Media
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		// Sponsorblock
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		// Batch
		colorGreen, colorReset,
		colorGreen, colorReset,
		colorGreen, colorReset,
		// Network
		colorGreen, colorReset,
		colorGreen, colorReset)
	runCleanups()
	os.Exit(0)
}

// scannerBufSize is large enough to handle yt-dlp's widest output lines
const scannerBufSize = 256 * 1024

func main() {

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
	sponsorMark := flag.Bool("sp-mark", false, "Mark sponsor segments as chapters instead of removing")
	sponsorCats := flag.String("sp-cats", "default", "SponsorBlock categories (e.g. sponsor,intro,outro)")
	subtitles := flag.Bool("subs", false, "Embed subtitles")
	selectItems := flag.Bool("select", false, "Interactive playlist item selector")
	cookies := flag.String("cookies", "", "Extract cookies from browser (e.g. chrome, firefox)")
	about := flag.Bool("about", false, "Show author info")
	dnsServer := flag.String("dns", "", "Use custom DNS server (bypasses system DNS)")
	start := flag.String("start", "", "Start timestamp for clipping (e.g. 01:00)")
	end := flag.String("end", "", "End timestamp for clipping (e.g. 02:30)")
	batchFile := flag.String("batch", "", "File containing multiple URLs to download")
	concurrent := flag.Int("j", 1, "Number of concurrent downloads (batch mode)")

	flag.Usage = usage
	flag.Parse()

	if *about {
		printBanner()
		fmt.Printf("%sStreamline%s is an open-source universal media downloader.\n", colorCyan, colorReset)
		fmt.Printf("Built with ❤️ by %sShahil Ahmed (SK)%s\n\n", colorYellow, colorReset)
		fmt.Printf("%sGitHub:%s   https://github.com/shahil-sk/streamline\n", colorCyan, colorReset)
		fmt.Printf("%sLicense:%s  MIT License\n\n", colorCyan, colorReset)
		os.Exit(0)
	}

	var urls []string
	if *batchFile != "" {
		b, err := os.ReadFile(*batchFile)
		if err != nil {
			exitWithError(fmt.Sprintf("Failed to read batch file: %v", err))
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				urls = append(urls, line)
			}
		}
	} else if flag.NArg() > 0 {
		urls = flag.Args()
	}

	if (!*musicMode && !*videoMode) || len(urls) == 0 {
		usage()
	}

	for _, u := range urls {
		if err := validateURL(u); err != nil {
			exitWithError(fmt.Sprintf("Invalid URL %s: %v", u, err))
		}
	}

	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0755); err != nil {
			exitWithError(fmt.Sprintf("Failed to create output directory: %v", err))
		}
	}

	ytdlpPath, ffmpegPath, cleanup := resolveBinaries()
	registerCleanup(cleanup)
	defer runCleanups()

	workDir, err := os.MkdirTemp("", "streamline-work")
	check(err)
	registerCleanup(func() { os.RemoveAll(workDir) })

	var proxyURL string
	if *dnsServer != "" {
		proxyURL, err = startDNSProxy(*dnsServer)
		if err != nil {
			exitWithError(fmt.Sprintf("Failed to start DNS proxy: %v", err))
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrent)

	for i, u := range urls {
		wg.Add(1)
		go func(index int, rawUrl string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if len(urls) > 1 {
				fmt.Printf("\n%s[%d/%d] Processing: %s%s\n", colorCyan, index+1, len(urls), rawUrl, colorReset)
			}

			var plItems string
			if *selectItems && !*quiet {
				plItems = selectPlaylistItems(ytdlpPath, rawUrl, proxyURL, *cookies)
			}

			if *musicMode {
				audioDownload(ytdlpPath, ffmpegPath, workDir, rawUrl, *outDir, proxyURL, *quiet || len(urls) > 1, *sponsorBlock, *sponsorMark, *sponsorCats, *start, *end, plItems, *cookies)
			} else if *videoMode {
				videoDownload(ytdlpPath, ffmpegPath, workDir, rawUrl, *outDir, proxyURL, *quiet || len(urls) > 1, *sponsorBlock, *sponsorMark, *sponsorCats, *subtitles, *start, *end, plItems, *cookies)
			}
			if len(urls) > 1 {
				fmt.Printf("%s[%d/%d] Completed: %s%s\n", colorGreen, index+1, len(urls), rawUrl, colorReset)
			}
		}(i, u)
	}

	wg.Wait()
}
