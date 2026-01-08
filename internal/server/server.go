package server

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Server struct {
	httpServer    *http.Server
	filePath      string
	token         string
	oneShot       bool
	downloadCount int
	mu            sync.Mutex
	done          chan struct{}
}

type Config struct {
	Port     int
	FilePath string
	Token    string
	OneShot  bool
}

func New(cfg Config) *Server {
	s := &Server{
		filePath: cfg.FilePath,
		token:    cfg.Token,
		oneShot:  cfg.OneShot,
		done:     make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/%s", cfg.Token), s.handleDownload)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return s
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Done() <-chan struct{} {
	return s.done
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	info, err := os.Stat(s.filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	clientIP := r.RemoteAddr
	log.Printf("[INFO] Download started from %s", clientIP)

	if info.IsDir() {
		s.serveFolder(w, r)
	} else {
		s.serveFile(w, r)
	}
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open(s.filePath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, _ := file.Stat()
	filename := filepath.Base(s.filePath)

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	startTime := time.Now()
	written, err := io.Copy(w, file)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[ERROR] Download failed: %v", err)
		return
	}

	log.Printf("[INFO] Download completed: %s (%d bytes in %v)", filename, written, duration.Round(time.Millisecond))

	s.handleDownloadComplete()
}

func (s *Server) serveFolder(w http.ResponseWriter, r *http.Request) {
	folderName := filepath.Base(s.filePath)
	zipFilename := folderName + ".zip"

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, zipFilename))
	w.Header().Set("Content-Type", "application/zip")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	startTime := time.Now()
	var totalBytes int64

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

		written, err := io.Copy(writer, file)
		totalBytes += written

		return err
	})

	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[ERROR] Folder zip failed: %v", err)
		return
	}

	log.Printf("[INFO] Folder download completed: %s (%d bytes in %v)", zipFilename, totalBytes, duration.Round(time.Millisecond))

	s.handleDownloadComplete()
}

func (s *Server) handleDownloadComplete() {
	if s.oneShot {
		s.mu.Lock()
		s.downloadCount++
		if s.downloadCount == 1 {
			close(s.done)
		}
		s.mu.Unlock()
	}
}
