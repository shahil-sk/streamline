<p align="center">
  <img src="https://github.com/user-attachments/assets/83c7f414-a8ea-4316-8ca3-9314fa6bb857" width="180" alt="Streamline Logo"/>
</p>

<h1 align="center">Streamline</h1>
<p align="center">
  A portable, dependency-free media downloader for YouTube and SoundCloud
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Platform-Linux_x86_64-1f1f1f?style=for-the-badge" />
  <img src="https://img.shields.io/badge/Built_with-Go-1f1f1f?style=for-the-badge&logo=go" />
  <img src="https://img.shields.io/badge/License-MIT-1f1f1f?style=for-the-badge" />
</p>

---

## Overview

**Streamline** is a single native Linux binary that allows you to download audio or video from YouTube and SoundCloud with embedded metadata and properly cropped cover art.

No Python installation required.
No separate ffmpeg installation.
No external yt-dlp setup.

Everything is bundled into one portable executable.

---

## Features

* Download YouTube audio as MP3 with embedded metadata
* Download YouTube videos with selectable quality
* Built-in SoundCloud support
* Automatically embeds and crops cover art
* Statically compiled Linux binary
* Can be moved to `/usr/local/bin` for global usage

---

## Installation (Prebuilt Binary)

After building or downloading the binary:

```bash
chmod +x streamline
sudo mv streamline /usr/local/bin
```

Then run from anywhere:

```bash
streamline -m <url>
```

---

## Usage

### Download Audio (MP3 with metadata)

```bash
streamline -m <url>
```

### Download Video (Interactive quality selection)

```bash
streamline -v <url>
```

Supports both YouTube and SoundCloud URLs out of the box.

---

## Build From Source

### Requirements

* Linux x86_64
* Go 1.16+
* Internet required only during build

Install Go if needed:

```bash
sudo apt install golang -y
```

### Build Steps

```bash
git clone https://github.com/yourusername/streamline.git
cd streamline

wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
wget https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz

tar -xf ffmpeg-release-amd64-static.tar.xz
cp ffmpeg-*/ffmpeg .

chmod +x yt-dlp ffmpeg

go build -o streamline streamline.go
```

---

## Why Streamline?

Many downloaders require multiple dependencies or runtime environments.
Streamline focuses on:

* Simplicity
* Portability
* Clean output files
* Zero system pollution

Move one binary. Run one command. Done.

---

## Contributing

Pull requests are welcome.
If you have improvements, performance ideas, or bug fixes, feel free to open an issue or submit a PR.
