package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

type PlaylistItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func selectPlaylistItems(ytdlpPath, url, proxyURL string) string {
	spinner := NewSpinner("Fetching playlist items...")
	spinner.Start()

	args := []string{"--flat-playlist", "--dump-json", url}
	if proxyURL != "" {
		args = append([]string{"--proxy", proxyURL}, args...)
	}

	cmd := exec.Command(ytdlpPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		spinner.Stop(false)
		exitWithError("Failed to create stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		spinner.Stop(false)
		exitWithError("Failed to start yt-dlp")
	}

	var items []string
	var indices []string

	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	count := 1
	for scanner.Scan() {
		line := scanner.Text()
		var item PlaylistItem
		if err := json.Unmarshal([]byte(line), &item); err == nil {
			title := item.Title
			if title == "" || title == "NA" || title == "null" {
				title = item.ID
			}
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			items = append(items, fmt.Sprintf("%d. %s", count, title))
			indices = append(indices, strconv.Itoa(count))
			count++
		}
	}

	cmd.Wait()
	spinner.Stop(true)

	if len(items) == 0 {
		return "" // Not a playlist or no items
	}

	var selected []string
	prompt := &survey.MultiSelect{
		Message:  "Select playlist items to download (Space to select, Enter to confirm):",
		Options:  items,
		PageSize: 15,
	}

	err = survey.AskOne(prompt, &selected)
	if err != nil {
		exitWithError("Selection cancelled")
	}

	if len(selected) == 0 {
		exitWithError("No items selected")
	}

	var selectedIndices []string
	for _, s := range selected {
		for i, item := range items {
			if s == item {
				selectedIndices = append(selectedIndices, indices[i])
				break
			}
		}
	}

	return strings.Join(selectedIndices, ",")
}
