package main

import (
	"os"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"http://soundcloud.com/user/track", false},
		{"ftp://invalid-scheme.com", true},
		{"not-a-url", true},
		{"", true},
		{"   ", true},
		{"https://", true},
	}

	for _, tt := range tests {
		err := validateURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
		}
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		sizeStr string
		want    float64
	}{
		{"1.5MiB", 1.5 * 1024 * 1024},
		{"2.0 GB", 2.0 * 1024 * 1024 * 1024},
		{"500K", 500 * 1024},
		{"100 B", 100},
		{"invalid", 0},
	}

	for _, tt := range tests {
		got := parseSize(tt.sizeStr)
		if got != tt.want {
			t.Errorf("parseSize(%q) = %v, want %v", tt.sizeStr, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{65, "01:05"},
		{3600, "60:00"},
		{3661, "1h 1m"},
		{-5, "--:--"},
		{86401, "--:--"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.seconds)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestSponsorBlockTrimming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	ytdlpPath, ffmpegPath, cleanup := resolveBinaries()
	defer cleanup()
	
	workDir, err := os.MkdirTemp("", "sponsor-test")
	if err != nil {
		t.Fatalf("Failed to create workdir: %v", err)
	}
	defer os.RemoveAll(workDir)
	
	outDir, err := os.MkdirTemp("", "sponsor-out")
	if err != nil {
		t.Fatalf("Failed to create outdir: %v", err)
	}
	defer os.RemoveAll(outDir)
	
	url := "https://www.youtube.com/watch?v=kG22Z4vJhXY"
	
	// Run video download with sponsorblock remove
	videoDownload(ytdlpPath, ffmpegPath, workDir, url, outDir, "", true, "sponsor", "", false, "", "", "", "")
	
	files, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("Failed to read outDir: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("Expected downloaded file, got none")
	}
}
