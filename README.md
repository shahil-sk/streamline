<p align="center">
  <img src="https://github.com/user-attachments/assets/83c7f414-a8ea-4316-8ca3-9314fa6bb857" width="180" alt="Streamline Logo"/>
</p>

<h1 align="center">Streamline</h1>
<p align="center">
  A fast, portable media downloader for YouTube and SoundCloud
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Platform-Linux_|_macOS_|_Windows-1f1f1f?style=for-the-badge" />
  <img src="https://img.shields.io/badge/Built_with-Go-1f1f1f?style=for-the-badge&logo=go" />
  <img src="https://img.shields.io/badge/License-MIT-1f1f1f?style=for-the-badge" />
</p>

---

## Overview

**Streamline** downloads audio or video from YouTube and SoundCloud with embedded metadata and cover art.

Two build modes are available:

| Mode | Binary size | Requires |
|------|-------------|----------|
| **Lightweight** (default) | ~2 MB | `yt-dlp` + `ffmpeg` on PATH |
| **Portable** (`-tags bundled`) | large | nothing – tools embedded inside |

---

## Features

* Download YouTube/SoundCloud audio as MP3 with embedded metadata and cover art
* Download YouTube videos with interactive quality selection
* Real-time progress bar with speed and ETA
* Cross-platform: Linux, macOS, Windows
* Can be placed in `/usr/local/bin` for global usage

---

## Usage

### Download Audio (MP3 + metadata + cover art)

```bash
streamline -m <url>
```

### Download Video (interactive quality selection)

```bash
streamline -v <url>
```

Supports YouTube and SoundCloud URLs.

---

## Installation (Prebuilt Binary)

```bash
chmod +x streamline
sudo mv streamline /usr/local/bin
```

Then run from anywhere:

```bash
streamline -m <url>
```

---

## Build From Source

### Requirements

* Go 1.17+
* `make` (optional, for convenience)

```bash
git clone https://github.com/shahil-sk/streamline.git
cd streamline
```

### Lightweight Build (recommended)

Produces a small (~2 MB) binary. Requires `yt-dlp` and `ffmpeg` to be installed on the target machine.

```bash
make build
# or manually:
go build -ldflags="-s -w" -trimpath -o streamline .
```

Install dependencies if needed:

```bash
# yt-dlp
sudo curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp
sudo chmod +x /usr/local/bin/yt-dlp

# ffmpeg (Debian/Ubuntu)
sudo apt install ffmpeg -y
```

### Portable / Bundled Build

Embeds `yt-dlp` and `ffmpeg` inside the binary. No runtime dependencies needed on the target machine.

```bash
# 1. Place the binaries in the project directory
wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
wget https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz
tar -xf ffmpeg-release-amd64-static.tar.xz
cp ffmpeg-*/ffmpeg .
chmod +x yt-dlp ffmpeg

# 2. Build
make build-portable
# or manually:
go build -tags bundled -ldflags="-s -w" -trimpath -o streamline .
```

---

## Why Streamline?

Many downloaders require multiple dependencies or runtime environments.
Streamline focuses on:

* Simplicity – one command, clean output
* Portability – run anywhere, with or without pre-installed tools
* Zero system pollution – temporary files are cleaned up automatically

---

## Contributing

Pull requests are welcome.
If you have improvements, performance ideas, or bug fixes, feel free to open an issue or submit a PR.
