<p align="center">
  <img src="https://github.com/user-attachments/assets/83c7f414-a8ea-4316-8ca3-9314fa6bb857" width="180" alt="Streamline Logo"/>
</p>

<h1 align="center">Streamline</h1>

<p align="center">
  <strong>A lightning-fast, ultra-portable media downloader for YouTube, SoundCloud, and Instagram.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Platform-Linux_|_macOS_|_Windows-1f1f1f?style=for-the-badge" />
  <img src="https://img.shields.io/badge/Built_with-Go-1f1f1f?style=for-the-badge&logo=go" />
  <img src="https://img.shields.io/badge/License-MIT-1f1f1f?style=for-the-badge" />
</p>

<br/>

##  Overview

**Streamline** provides an interactive and highly optimized CLI experience for downloading high-quality audio and video. It wraps industry-standard tools into a beautifully simple interface, while adding powerful custom features like **native DNS-over-HTTPS (DoH)** to bypass restrictive corporate/school firewalls.

##  Features

- **Multi-Platform Support**: Works flawlessly with **YouTube**, **SoundCloud**, and **Instagram**.
- **Audio Extraction**: Converts media to high-quality formats (MP3, FLAC, M4A, WAV, OPUS).
- **Rich Metadata**: Automatically embeds ID3 tags, artist metadata, and cover art into the downloaded files.
- **Interactive Prompts**: Clean, intuitive TUI for selecting media quality and formats.
- **SponsorBlock Integration**: Pass the `-s` flag to automatically strip out baked-in sponsor segments, intros, and outros.
- **Subtitle Embedding**: Embed auto-generated or official subtitles directly into video files.
- **Firewall & DNS Bypass**: Use the `--dns` flag with a DoH endpoint (e.g. `https://dns.google/resolve`) to securely route traffic through a custom Go-native proxy, bypassing Network Restricted Modes.
- **Zero Configuration**: Available as a fully bundled single-binary containing all underlying dependencies.

---

## 💻 Usage

### Download Audio (with metadata & cover art)

```bash
streamline -m <url>
```

### Download Video (interactive quality selection)

```bash
streamline -v <url>
```

### Bypass Restricted Networks

If your network forces YouTube Restricted Mode, bypass it using DoH:
```bash
streamline -v --dns https://dns.google/resolve <url>
```

### Additional Flags

```bash
  -m        Music/audio mode
  -v        Video mode
  -o        Output directory (default: current directory)
  -q        Quiet mode (skip prompts, use best quality)
  -s        Remove sponsor segments (SponsorBlock)
  --subs    Embed subtitles (video only)
  --dns     Bypass system DNS via custom server or DoH endpoint
  --about   Author information
```

---

## 📦 Installation

### Prebuilt Bundled Binary (Recommended)
Download the latest portable release for your OS from the **[Releases](https://github.com/shahil-sk/streamline/releases)** page. The bundled release contains `yt-dlp` and `ffmpeg` embedded directly inside the binary.

```bash
chmod +x streamline-linux-amd64-bundled
sudo mv streamline-linux-amd64-bundled /usr/local/bin/streamline
```

### Build From Source

**Requirements:**
* Go 1.17+
* `make` (optional)

#### 1. Lightweight Build
Produces a small (~2 MB) binary. Requires `yt-dlp` and `ffmpeg` to be installed on the target machine.
```bash
git clone https://github.com/shahil-sk/streamline.git
cd streamline
go build -ldflags="-s -w" -trimpath -o streamline .
```

#### 2. Bundled Build
Embeds the required binaries into the executable.
```bash
make build-portable
# Or manually build using the 'bundled' tag:
# go build -tags bundled -ldflags="-s -w" -trimpath -o streamline .
```

---

## 🤝 Contributing
Pull requests are welcome! If you have improvements, performance ideas, or bug fixes, feel free to open an issue or submit a PR.
