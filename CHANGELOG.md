# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-01-08

### Added

- **E2E Encryption**: AES-256-GCM encryption enabled by default, key shared via URL fragment
- **--no-encrypt flag**: Option to disable encryption for faster transfers

## [0.1.1] - 2026-01-08

### Added

- **Clipboard Bridge (Push)**: `share clip` - Share Mac clipboard text to mobile
- **Clipboard Bridge (Pull)**: `share paste` - Receive text from mobile to Mac clipboard

## [0.1.0] - 2026-01-08

### Added

- **File Sharing**: Share any file via `share <filename>` command
- **Folder Sharing**: Share folders with automatic zip streaming (no temp files)
- **QR Code Display**: ASCII art QR code in terminal for easy mobile scanning
- **Receive Mode**: Accept file uploads via `share --receive`
- **Auto Network Discovery**: Automatically finds LAN IP address
- **Port Selection**: Default port 8080 with automatic fallback
- **Obfuscated URLs**: Random token in URL for security
- **One-Shot Mode**: Auto-close after first download with `--one-shot` flag
- **Graceful Shutdown**: Clean exit on Ctrl+C
- **Download/Upload Logging**: Real-time connection and transfer logs
- **Modern Upload UI**: Glassmorphism design with drag-and-drop support
