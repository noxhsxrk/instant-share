# Instant Share

🚀 Zero-configuration CLI tool for instant file sharing over LAN via QR code.

## Features

- **Zero Config** - Auto-detects your LAN IP and available port
- **QR Code** - Scan to download/upload from any device
- **Universal** - Works with any device with a browser (Android, iOS, Windows, etc.)
- **Folder Support** - Auto-zips folders on-the-fly
- **Receive Mode** - Accept files from other devices
- **Secure** - Random token in URL prevents guessing
- **One-Shot** - Auto-close after first download

## Installation

### From Source

```bash
# Clone and build
git clone https://github.com/saharat-wae/instant-share.git
cd instant-share
go build -o share .

# Install globally (requires sudo)
sudo cp share /usr/local/bin/share

# Or install to GOPATH
go install .
```

## Usage

### Share a File

```bash
share myfile.pdf
```

### Share a Folder

```bash
share ./my-folder
```

Folders are automatically zipped and streamed (no temp files created).

### Receive Files

```bash
share --receive
```

Opens a web upload form for receiving files from other devices.

### Options

```
Flags:
  -p, --port int   Port to serve on (default 8080)
  -r, --receive    Enable receive mode (accept file uploads)
      --one-shot   Auto-close after first download
  -h, --help       Help for share
```

## Example Output

```
$ share presentation.pdf

[INFO] Preparing to share 'presentation.pdf' (15.2 MB)
[INFO] Server started at http://192.168.1.45:8080/x7f9a

  ██▀▀▀▀▀██ ▄ ▄ ██▀▀▀▀▀██  
  ██ ███ ██ ▄▄█ ██ ███ ██  
  ██ ▀▀▀ ██ ▄▀  ██ ▀▀▀ ██  
  (QR Code)

Scan this QR code to download.
Press Ctrl+C to stop.
```

## Requirements

- Go 1.20+
- Same Wi-Fi network between devices
- macOS firewall must allow the port

## License

MIT
