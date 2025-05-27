<p align="center">
  <img src="https://github.com/user-attachments/assets/c07a86d1-3d1f-4d6c-b20e-15a5d2b01c39" alt="Streamline Logo" width="200"/>
</p>

<h1 align="center">Streamline by sk</h1>
<br>

**Streamline** is a portable, dependency-free command-line tool to download YouTube videos or music with embedded metadata and perfectly cropped cover art — all packed into a single native Linux binary.

```
> No need for Python, no need to install ffmpeg or yt-dlp — everything is bundled.
```



## ⚙️ Features

- 🎶 Download YouTube audio as MP3
- 📽 Download YouTube videos (choose quality)
- 🐧 Linux native binary — statically compiled
- ➡️ Move the streamline binary to "/usr/local/bin" for system-wide access
  -   `mv streamline /usr/local/bin`



## 📦 Usage

```bash
./streamline -m <youtube-url>    # Download audio with metadata & cover
./streamline -v <youtube-url>    # Download video (will ask format)
````



## 🔨 Build Instructions

> ⚠️ Requires Go installed (`sudo apt install golang -y`)

```bash
git clone https://github.com/yourusername/streamline.git
cd streamline

# Download dependencies
wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
wget https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz
tar -xf ffmpeg-release-amd64-static.tar.xz
cp ffmpeg-*/ffmpeg .

chmod +x yt-dlp ffmpeg

# Build the final binary
go build -o streamline streamline.go
```



## ✅ Requirements

* Linux x86\_64 system
* Go 1.16+
* Internet only needed for building



## 🤝 Contributing

Pull requests welcome! If you have improvements, ideas, or bug fixes, feel free to open an issue or PR.

Let me know if you'd like a **logo**, **badges**, or want this in a **dark-mode styled README**.
