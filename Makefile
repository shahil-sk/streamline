.PHONY: build build-portable clean

# Lightweight build – uses system yt-dlp and ffmpeg (tiny binary, ~2 MB)
# -ldflags="-s -w" strips symbol table + DWARF debug info
# -trimpath removes local file paths embedded in the binary
build:
	go build -ldflags="-s -w" -trimpath -o streamline .

# Portable/bundled build – embeds yt-dlp and ffmpeg into the binary
# Before running, place yt-dlp and ffmpeg executables in this directory:
#   wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp
#   # (extract ffmpeg from https://johnvansickle.com/ffmpeg/)
#   chmod +x yt-dlp ffmpeg
build-portable:
	go build -tags bundled -ldflags="-s -w" -trimpath -o streamline .

clean:
	rm -f streamline
