import os
import subprocess
import json
import argparse
from pathlib import Path
from mutagen.easyid3 import EasyID3
from mutagen.id3 import ID3, APIC
import sys
sys.path.insert(0, str(Path(__file__).parent / "yt-dlp"))
from yt_dlp import YoutubeDL

DOWNLOAD_DIR = Path.home() / "Downloads"
TMP_MP3 = "downloaded.mp3"
TMP_VIDEO = "video_downloaded"
TMP_JSON = "downloaded.info.json"
TMP_COVER = "cover.jpg"

FFMPEG = str(Path(__file__).parent / "ffmpeg" / "ffmpeg")
YTDLP = str(Path(__file__).parent / "yt-dlp" / "yt-dlp")


def run_cmd(cmd):
    subprocess.run(cmd, shell=True, check=True)

def download_music(youtube_url):
    print("\n📥 Downloading music and metadata...")
    run_cmd(
        f'yt-dlp -x --audio-format mp3 --embed-thumbnail --embed-metadata '
        f'--add-metadata --write-info-json --write-thumbnail '
        f'-o "downloaded.%(ext)s" "{youtube_url}"'
    )

def download_video(youtube_url):
    print("\n🎞️ Fetching available formats...")
    subprocess.run(f'{YTDLP} -F "{youtube_url}"', shell=True)
    fmt = input("\n🎚️ Enter desired format code (e.g. 22, 137+140): ").strip()
    print("\n📥 Downloading selected video format...")
    run_cmd(f'{YTDLP} -f "{fmt}" -o "{TMP_VIDEO}.%(ext)s" "{youtube_url}"')

def process_thumbnail():
    for ext in ["jpg", "webp", "png"]:
        thumb = Path(f"downloaded.{ext}")
        if thumb.exists():
            cmd = rf'{FFMPEG} -y -i "{thumb}" -vf "crop=\'min(in_w\\,in_h)\':\'min(in_w\\,in_h)\',scale=500:500" -frames:v 1 "{TMP_COVER}"'
            run_cmd(cmd)
            return
    raise FileNotFoundError("Thumbnail not found")

def embed_music_metadata():
    with open(TMP_JSON, "r", encoding="utf-8") as f:
        data = json.load(f)

    title = data.get("track") or data.get("title", "Unknown Title")
    artist = data.get("artist") or data.get("uploader", "Unknown Artist")
    album = data.get("album") or "YouTube Downloads"
    track = str(data.get("track_number") or "1")

    print("🔖 Embedding metadata...")
    audio = EasyID3(TMP_MP3)
    audio["title"] = title
    audio["artist"] = artist
    audio["album"] = album
    audio["tracknumber"] = track
    audio.save()

    audio = ID3(TMP_MP3)
    with open(TMP_COVER, "rb") as img:
        audio["APIC"] = APIC(
            encoding=3,
            mime="image/jpeg",
            type=3,
            desc="Cover",
            data=img.read()
        )
    audio.save()

    final_file = DOWNLOAD_DIR / f"{artist} - {title}.mp3"
    os.rename(TMP_MP3, final_file)
    print(f"\n✅ Saved to: {final_file}")

def cleanup():
    for ext in ["mp3", "mp4", "webm", "webp", "jpg", "json", "png"]:
        for file in Path(".").glob(f"downloaded*.{ext}"):
            print(f"🧹 Removing: {file}")
            file.unlink()

    if Path("cover.jpg").exists():
        print("🧹 Removing: cover.jpg")
        Path("cover.jpg").unlink()


def main():
    parser = argparse.ArgumentParser(description="YouTube Media Downloader")
    parser.add_argument("url", help="YouTube URL")
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("-m", "--music", action="store_true", help="Download music with metadata")
    group.add_argument("-v", "--video", action="store_true", help="Download video")
    args = parser.parse_args()

    if args.music:
        download_music(args.url)
        process_thumbnail()
        embed_music_metadata()
    elif args.video:
        download_video(args.url)
        cleanup()
        print(f"\n✅ Video saved in current directory with prefix: {TMP_VIDEO}")

if __name__ == "__main__":
    main()