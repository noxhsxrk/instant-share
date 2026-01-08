package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/saharat-wae/instant-share/internal/crypto"
	"github.com/saharat-wae/instant-share/internal/network"
	"github.com/saharat-wae/instant-share/internal/qr"
	"github.com/saharat-wae/instant-share/internal/server"
	"github.com/saharat-wae/instant-share/internal/token"
	"github.com/spf13/cobra"
)

var (
	port        int
	receiveMode bool
	oneShot     bool
	noEncrypt   bool
)

var rootCmd = &cobra.Command{
	Use:   "share [file or folder]",
	Short: "Instant Share - Zero-config file sharing over LAN",
	Long: `Instant Share is a CLI tool for sharing files instantly over your local network.
Simply run 'share <file>' and scan the QR code from any device to download.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runShare,
}

var clipCmd = &cobra.Command{
	Use:   "clip",
	Short: "Share clipboard text to mobile (push mode)",
	Run:   runClipMode,
}

var pasteCmd = &cobra.Command{
	Use:   "paste",
	Short: "Receive text from mobile to clipboard (pull mode)",
	Run:   runPasteMode,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to serve on")
	rootCmd.Flags().BoolVarP(&receiveMode, "receive", "r", false, "Enable receive mode (accept file uploads)")
	rootCmd.Flags().BoolVar(&oneShot, "one-shot", false, "Auto-close after first download")
	rootCmd.Flags().BoolVar(&noEncrypt, "no-encrypt", false, "Disable end-to-end encryption")

	clipCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to serve on")
	pasteCmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to serve on")

	rootCmd.AddCommand(clipCmd)
	rootCmd.AddCommand(pasteCmd)
}

func runShare(cmd *cobra.Command, args []string) {
	localIP, err := network.GetLocalIP()
	if err != nil {
		log.Fatalf("[ERROR] Failed to get local IP: %v", err)
	}

	availablePort, err := network.GetAvailablePort(port)
	if err != nil {
		log.Fatalf("[ERROR] Failed to get available port: %v", err)
	}

	tok := token.Generate()

	if receiveMode {
		runReceiveMode(localIP, availablePort, tok)
	} else {
		if len(args) == 0 {
			fmt.Println("Error: Please specify a file or folder to share")
			fmt.Println("Usage: share <file or folder>")
			fmt.Println("       share --receive")
			fmt.Println("       share clip")
			fmt.Println("       share paste")
			os.Exit(1)
		}
		runSendMode(args[0], localIP, availablePort, tok)
	}
}

func runSendMode(filePath string, localIP string, port int, tok string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Fatalf("[ERROR] Invalid path: %v", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		log.Fatalf("[ERROR] File not found: %s", filePath)
	}

	var sizeStr string
	if info.IsDir() {
		sizeStr = "folder"
	} else {
		sizeStr = formatSize(info.Size())
	}

	if noEncrypt {
		runUnencryptedSendMode(absPath, sizeStr, localIP, port, tok)
	} else {
		runEncryptedSendMode(absPath, sizeStr, localIP, port, tok)
	}
}

func runEncryptedSendMode(absPath, sizeStr, localIP string, port int, tok string) {
	encKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("[ERROR] Failed to generate encryption key: %v", err)
	}

	keyB64 := crypto.KeyToBase64(encKey)
	url := fmt.Sprintf("http://%s:%d/%s#%s", localIP, port, tok, keyB64)

	fmt.Printf("\n[INFO] Preparing to share '%s' (%s)\n", filepath.Base(absPath), sizeStr)
	fmt.Println("[INFO] 🔐 End-to-end encryption enabled")
	fmt.Printf("[INFO] Server started at http://%s:%d/%s\n\n", localIP, port, tok)

	qrCode, err := qr.GenerateASCIICompact(url)
	if err != nil {
		log.Fatalf("[ERROR] Failed to generate QR code: %v", err)
	}
	fmt.Println(qrCode)

	fmt.Println("Scan this QR code to download (encrypted).")
	if oneShot {
		fmt.Println("Server will stop after the first download.")
	}
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	srv := server.NewEncryptedServer(server.EncryptedConfig{
		Port:          port,
		FilePath:      absPath,
		Token:         tok,
		EncryptionKey: encKey,
		OneShot:       oneShot,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("[ERROR] Server error: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\n[INFO] Shutting down...")
	case <-srv.Done():
		fmt.Println("\n[INFO] Download completed, shutting down...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Shutdown error: %v", err)
	}

	fmt.Println("[INFO] Server stopped.")
}

func runUnencryptedSendMode(absPath, sizeStr, localIP string, port int, tok string) {
	url := fmt.Sprintf("http://%s:%d/%s", localIP, port, tok)

	fmt.Printf("\n[INFO] Preparing to share '%s' (%s)\n", filepath.Base(absPath), sizeStr)
	fmt.Printf("[INFO] Server started at %s\n\n", url)

	qrCode, err := qr.GenerateASCIICompact(url)
	if err != nil {
		log.Fatalf("[ERROR] Failed to generate QR code: %v", err)
	}
	fmt.Println(qrCode)

	fmt.Println("Scan this QR code to download.")
	if oneShot {
		fmt.Println("Server will stop after the first download.")
	}
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	srv := server.New(server.Config{
		Port:     port,
		FilePath: absPath,
		Token:    tok,
		OneShot:  oneShot,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("[ERROR] Server error: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\n[INFO] Shutting down...")
	case <-srv.Done():
		fmt.Println("\n[INFO] Download completed, shutting down...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Shutdown error: %v", err)
	}

	fmt.Println("[INFO] Server stopped.")
}

func runReceiveMode(localIP string, port int, tok string) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("[ERROR] Failed to get current directory: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d/%s", localIP, port, tok)

	fmt.Println("\n[INFO] Receive Mode Enabled")
	fmt.Printf("[INFO] Files will be saved to: %s\n", cwd)
	fmt.Printf("[INFO] Upload page: %s\n\n", url)

	qrCode, err := qr.GenerateASCIICompact(url)
	if err != nil {
		log.Fatalf("[ERROR] Failed to generate QR code: %v", err)
	}
	fmt.Println(qrCode)

	fmt.Println("Scan this QR code to upload files.")
	fmt.Println("Waiting for incoming files...")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	srv := server.NewReceiveServer(server.ReceiveConfig{
		Port:    port,
		SaveDir: cwd,
		Token:   tok,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("[ERROR] Server error: %v", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\n[INFO] Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Shutdown error: %v", err)
	}

	fmt.Println("[INFO] Server stopped.")
}

func runClipMode(cmd *cobra.Command, args []string) {
	localIP, err := network.GetLocalIP()
	if err != nil {
		log.Fatalf("[ERROR] Failed to get local IP: %v", err)
	}

	availablePort, err := network.GetAvailablePort(port)
	if err != nil {
		log.Fatalf("[ERROR] Failed to get available port: %v", err)
	}

	tok := token.Generate()

	content, err := server.GetMacClipboard()
	if err != nil {
		log.Fatalf("[ERROR] Failed to read clipboard: %v", err)
	}

	if content == "" {
		log.Fatalf("[ERROR] Clipboard is empty")
	}

	url := fmt.Sprintf("http://%s:%d/%s", localIP, availablePort, tok)

	fmt.Println("\n[INFO] Clipboard Share Mode (Push)")
	fmt.Printf("[INFO] Sharing %d characters\n", len(content))
	fmt.Printf("[INFO] Page: %s\n\n", url)

	qrCode, err := qr.GenerateASCIICompact(url)
	if err != nil {
		log.Fatalf("[ERROR] Failed to generate QR code: %v", err)
	}
	fmt.Println(qrCode)

	fmt.Println("Scan to copy text to your device.")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	srv := server.NewClipboardServer(server.ClipboardConfig{
		Port:    availablePort,
		Token:   tok,
		Mode:    "clip",
		Content: content,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("[ERROR] Server error: %v", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\n[INFO] Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Shutdown error: %v", err)
	}

	fmt.Println("[INFO] Server stopped.")
}

func runPasteMode(cmd *cobra.Command, args []string) {
	localIP, err := network.GetLocalIP()
	if err != nil {
		log.Fatalf("[ERROR] Failed to get local IP: %v", err)
	}

	availablePort, err := network.GetAvailablePort(port)
	if err != nil {
		log.Fatalf("[ERROR] Failed to get available port: %v", err)
	}

	tok := token.Generate()
	url := fmt.Sprintf("http://%s:%d/%s", localIP, availablePort, tok)

	fmt.Println("\n[INFO] Clipboard Receive Mode (Pull)")
	fmt.Printf("[INFO] Page: %s\n\n", url)

	qrCode, err := qr.GenerateASCIICompact(url)
	if err != nil {
		log.Fatalf("[ERROR] Failed to generate QR code: %v", err)
	}
	fmt.Println(qrCode)

	fmt.Println("Scan to send text to Mac clipboard.")
	fmt.Println("Waiting for text...")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	srv := server.NewClipboardServer(server.ClipboardConfig{
		Port:  availablePort,
		Token: tok,
		Mode:  "paste",
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Printf("[ERROR] Server error: %v", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\n[INFO] Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Shutdown error: %v", err)
	}

	fmt.Println("[INFO] Server stopped.")
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
