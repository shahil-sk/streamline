package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

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

func init() {
	if runtime.GOOS == "windows" || !isTerminal() {
		colorReset, colorRed, colorGreen, colorYellow = "", "", "", ""
		colorBlue, colorCyan, colorBold, colorDim = "", "", "", ""
	}
}

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

type Spinner struct {
	frames   []string
	index    int
	message  string
	stop     chan struct{}
	stopOnce sync.Once
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
	s.stopOnce.Do(func() {
		close(s.stop)
		time.Sleep(100 * time.Millisecond)
		icon, color := "[OK]", colorGreen
		if !success {
			icon, color = "[FAIL]", colorRed
		}
		fmt.Printf("\r%s%s%s %s\n", color, icon, colorReset, s.message)
	})
}

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

func printBanner() {
	const banner = `   _____ __                               ___          
  / ___// /_________  ____ _____ ___     / (_)___  ___ 
  \__ \/ __/ ___/ _ \/ __ '/ __ '__ \   / / / __ \/ _ \
 ___/ / /_/ /  /  __/ /_/ / / / / / /  / / / / / /  __/
/____/\__/_/   \___/\__,_/_/ /_/ /_/  /_/_/_/ /_/\___/ 
      Universal Media Downloader (1000+ Sites)
          with Native SponsorBlock`
	fmt.Printf("%s%s%s\n\n", colorCyan, banner, colorReset)
}

func printStatus(status, message string) {
	type entry struct{ icon, color string }
	table := map[string]entry{
		"info":    {"[INFO]", colorBlue},
		"success": {"[OK]", colorGreen},
		"warning": {"[WARN]", colorYellow},
		"error":   {"[FAIL]", colorRed},
	}
	e, ok := table[status]
	if !ok {
		e = entry{"[*]", colorReset}
	}
	fmt.Printf("%s%s%s %s\n", e.color, e.icon, colorReset, message)
}
