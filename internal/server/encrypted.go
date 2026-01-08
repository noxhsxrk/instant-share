package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/saharat-wae/instant-share/internal/crypto"
)

type EncryptedServer struct {
	httpServer    *http.Server
	filePath      string
	token         string
	encryptionKey []byte
	oneShot       bool
	downloadCount int
	mu            sync.Mutex
	done          chan struct{}
}

type EncryptedConfig struct {
	Port          int
	FilePath      string
	Token         string
	EncryptionKey []byte
	OneShot       bool
}

func NewEncryptedServer(cfg EncryptedConfig) *EncryptedServer {
	s := &EncryptedServer{
		filePath:      cfg.FilePath,
		token:         cfg.Token,
		encryptionKey: cfg.EncryptionKey,
		oneShot:       cfg.OneShot,
		done:          make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/%s", cfg.Token), s.handleDownloadPage)
	mux.HandleFunc(fmt.Sprintf("/%s/data", cfg.Token), s.handleEncryptedData)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return s
}

func (s *EncryptedServer) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *EncryptedServer) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *EncryptedServer) Done() <-chan struct{} {
	return s.done
}

const encryptedDownloadPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Instant Share - Encrypted Download</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            min-height: 100vh;
            display: flex;
            justify-content: center;
            align-items: center;
            padding: 20px;
        }
        .container {
            background: rgba(255, 255, 255, 0.05);
            backdrop-filter: blur(10px);
            border-radius: 20px;
            padding: 40px;
            max-width: 400px;
            width: 100%;
            border: 1px solid rgba(255, 255, 255, 0.1);
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
            text-align: center;
        }
        h1 { color: #fff; font-size: 1.8rem; margin-bottom: 10px; }
        .subtitle { color: rgba(255, 255, 255, 0.6); margin-bottom: 30px; font-size: 0.9rem; }
        .file-info {
            background: rgba(0, 0, 0, 0.3);
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 20px;
        }
        .file-name { color: #fff; font-size: 1.1rem; font-weight: 600; word-break: break-all; }
        .file-size { color: rgba(255, 255, 255, 0.5); font-size: 0.85rem; margin-top: 5px; }
        .secure-badge {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            background: rgba(16, 185, 129, 0.2);
            color: #10b981;
            padding: 8px 16px;
            border-radius: 20px;
            font-size: 0.85rem;
            margin-bottom: 20px;
        }
        .btn {
            width: 100%;
            padding: 15px;
            background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
            border: none;
            border-radius: 10px;
            color: #fff;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 10px 30px -10px rgba(99, 102, 241, 0.5); }
        .btn:disabled { opacity: 0.5; cursor: not-allowed; transform: none; }
        .progress { margin-top: 20px; display: none; }
        .progress.active { display: block; }
        .progress-bar {
            height: 6px;
            background: rgba(255, 255, 255, 0.2);
            border-radius: 3px;
            overflow: hidden;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
            width: 0%;
            transition: width 0.3s;
        }
        .progress-text { color: rgba(255, 255, 255, 0.6); font-size: 0.85rem; margin-top: 10px; }
        .message { padding: 15px; border-radius: 10px; margin-top: 20px; display: none; }
        .message.success { background: rgba(34, 197, 94, 0.2); color: #22c55e; display: block; }
        .message.error { background: rgba(239, 68, 68, 0.2); color: #ef4444; display: block; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 Secure Download</h1>
        <p class="subtitle">End-to-end encrypted</p>
        
        <div class="secure-badge">🛡️ AES-256 Encrypted</div>
        
        <div class="file-info">
            <div class="file-name">{{.FileName}}</div>
            <div class="file-size">{{.FileSize}}</div>
        </div>
        
        <button class="btn" id="downloadBtn" onclick="startDownload()">📥 Download & Decrypt</button>
        
        <div class="progress" id="progress">
            <div class="progress-bar"><div class="progress-fill" id="progressFill"></div></div>
            <div class="progress-text" id="progressText">Downloading...</div>
        </div>
        
        <div class="message" id="message"></div>
    </div>

    <script>
        async function startDownload() {
            const btn = document.getElementById('downloadBtn');
            const progress = document.getElementById('progress');
            const progressFill = document.getElementById('progressFill');
            const progressText = document.getElementById('progressText');
            const message = document.getElementById('message');
            
            // Get key from URL fragment
            const keyB64 = window.location.hash.substring(1);
            if (!keyB64) {
                message.className = 'message error';
                message.textContent = 'Missing encryption key';
                return;
            }
            
            btn.disabled = true;
            progress.classList.add('active');
            
            try {
                // Fetch encrypted data
                progressText.textContent = 'Downloading encrypted data...';
                const response = await fetch('/{{.Token}}/data');
                if (!response.ok) throw new Error('Download failed');
                
                progressFill.style.width = '50%';
                progressText.textContent = 'Decrypting...';
                
                const encryptedB64 = await response.text();
                const encryptedData = Uint8Array.from(atob(encryptedB64), c => c.charCodeAt(0));
                
                // Import key
                const keyData = Uint8Array.from(atob(keyB64.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0));
                const key = await crypto.subtle.importKey('raw', keyData, 'AES-GCM', false, ['decrypt']);
                
                // Extract nonce (first 12 bytes) and ciphertext
                const nonce = encryptedData.slice(0, 12);
                const ciphertext = encryptedData.slice(12);
                
                // Decrypt
                const decrypted = await crypto.subtle.decrypt(
                    { name: 'AES-GCM', iv: nonce },
                    key,
                    ciphertext
                );
                
                progressFill.style.width = '100%';
                progressText.textContent = 'Complete!';
                
                // Trigger download
                const blob = new Blob([decrypted]);
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = '{{.FileName}}';
                a.click();
                URL.revokeObjectURL(url);
                
                message.className = 'message success';
                message.textContent = '✓ File decrypted and downloaded!';
                
            } catch (err) {
                console.error(err);
                message.className = 'message error';
                message.textContent = '✗ Decryption failed: ' + err.message;
            }
            
            btn.disabled = false;
        }
    </script>
</body>
</html>`

func (s *EncryptedServer) handleDownloadPage(w http.ResponseWriter, r *http.Request) {
	info, err := os.Stat(s.filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	fileName := filepath.Base(s.filePath)
	if info.IsDir() {
		fileName += ".zip"
	}

	tmpl := template.Must(template.New("download").Parse(encryptedDownloadPageHTML))
	data := struct {
		FileName string
		FileSize string
		Token    string
	}{
		FileName: fileName,
		FileSize: formatFileSize(info.Size()),
		Token:    s.token,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

func (s *EncryptedServer) handleEncryptedData(w http.ResponseWriter, r *http.Request) {
	clientIP := r.RemoteAddr
	log.Printf("[INFO] Encrypted download started from %s", clientIP)

	info, err := os.Stat(s.filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	var fileData []byte
	startTime := time.Now()

	if info.IsDir() {
		fileData, err = s.zipFolder()
	} else {
		fileData, err = os.ReadFile(s.filePath)
	}

	if err != nil {
		log.Printf("[ERROR] Failed to read file: %v", err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	encrypted, err := crypto.Encrypt(fileData, s.encryptionKey)
	if err != nil {
		log.Printf("[ERROR] Encryption failed: %v", err)
		http.Error(w, "Encryption failed", http.StatusInternalServerError)
		return
	}

	duration := time.Since(startTime)
	log.Printf("[INFO] Encrypted download completed: %s (%d bytes encrypted in %v)",
		filepath.Base(s.filePath), len(encrypted), duration.Round(time.Millisecond))

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(base64.StdEncoding.EncodeToString(encrypted)))

	s.handleDownloadComplete()
}

func (s *EncryptedServer) zipFolder() ([]byte, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	folderName := filepath.Base(s.filePath)

	err := filepath.Walk(s.filePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(s.filePath, path)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.Join(folderName, relPath)
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
	if err != nil {
		return nil, err
	}

	zipWriter.Close()
	return buf.Bytes(), nil
}

func (s *EncryptedServer) handleDownloadComplete() {
	if s.oneShot {
		s.mu.Lock()
		s.downloadCount++
		if s.downloadCount == 1 {
			close(s.done)
		}
		s.mu.Unlock()
	}
}

func formatFileSize(bytes int64) string {
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
