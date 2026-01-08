package server

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type ReceiveServer struct {
	httpServer *http.Server
	saveDir    string
	token      string
}

type ReceiveConfig struct {
	Port    int
	SaveDir string
	Token   string
}

func NewReceiveServer(cfg ReceiveConfig) *ReceiveServer {
	s := &ReceiveServer{
		saveDir: cfg.SaveDir,
		token:   cfg.Token,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/%s", cfg.Token), s.handleUploadPage)
	mux.HandleFunc(fmt.Sprintf("/%s/upload", cfg.Token), s.handleUpload)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return s
}

func (s *ReceiveServer) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *ReceiveServer) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

const uploadPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Instant Share - Upload</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
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
        }
        h1 {
            color: #fff;
            font-size: 1.8rem;
            margin-bottom: 10px;
            text-align: center;
        }
        .subtitle {
            color: rgba(255, 255, 255, 0.6);
            text-align: center;
            margin-bottom: 30px;
            font-size: 0.9rem;
        }
        .upload-area {
            border: 2px dashed rgba(255, 255, 255, 0.3);
            border-radius: 15px;
            padding: 40px 20px;
            text-align: center;
            cursor: pointer;
            transition: all 0.3s ease;
            margin-bottom: 20px;
        }
        .upload-area:hover {
            border-color: #6366f1;
            background: rgba(99, 102, 241, 0.1);
        }
        .upload-area.dragover {
            border-color: #6366f1;
            background: rgba(99, 102, 241, 0.2);
        }
        .upload-icon {
            font-size: 3rem;
            margin-bottom: 15px;
        }
        .upload-text {
            color: rgba(255, 255, 255, 0.8);
            font-size: 1rem;
        }
        .upload-hint {
            color: rgba(255, 255, 255, 0.4);
            font-size: 0.8rem;
            margin-top: 10px;
        }
        input[type="file"] {
            display: none;
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
        .btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 30px -10px rgba(99, 102, 241, 0.5);
        }
        .btn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
            transform: none;
        }
        .file-name {
            color: #6366f1;
            font-size: 0.9rem;
            margin-top: 15px;
            word-break: break-all;
        }
        .message {
            padding: 15px;
            border-radius: 10px;
            margin-top: 20px;
            text-align: center;
        }
        .message.success {
            background: rgba(34, 197, 94, 0.2);
            color: #22c55e;
        }
        .message.error {
            background: rgba(239, 68, 68, 0.2);
            color: #ef4444;
        }
        .progress-bar {
            height: 4px;
            background: rgba(255, 255, 255, 0.2);
            border-radius: 2px;
            margin-top: 20px;
            overflow: hidden;
            display: none;
        }
        .progress-bar.active {
            display: block;
        }
        .progress-bar-fill {
            height: 100%;
            background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
            width: 0%;
            transition: width 0.3s ease;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>📤 Instant Share</h1>
        <p class="subtitle">Upload files to {{.SaveDir}}</p>
        
        <form id="uploadForm" action="/{{.Token}}/upload" method="post" enctype="multipart/form-data">
            <div class="upload-area" id="dropZone">
                <div class="upload-icon">📁</div>
                <p class="upload-text">Drop file here or tap to select</p>
                <p class="upload-hint">Any file type supported</p>
                <input type="file" name="file" id="fileInput" required>
                <p class="file-name" id="fileName"></p>
            </div>
            <div class="progress-bar" id="progressBar">
                <div class="progress-bar-fill" id="progressFill"></div>
            </div>
            <button type="submit" class="btn" id="uploadBtn" disabled>Upload</button>
        </form>
        <div class="message" id="message" style="display: none;"></div>
    </div>

    <script>
        const dropZone = document.getElementById('dropZone');
        const fileInput = document.getElementById('fileInput');
        const fileName = document.getElementById('fileName');
        const uploadBtn = document.getElementById('uploadBtn');
        const uploadForm = document.getElementById('uploadForm');
        const message = document.getElementById('message');
        const progressBar = document.getElementById('progressBar');
        const progressFill = document.getElementById('progressFill');

        dropZone.addEventListener('click', () => fileInput.click());
        
        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.classList.add('dragover');
        });
        
        dropZone.addEventListener('dragleave', () => {
            dropZone.classList.remove('dragover');
        });
        
        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropZone.classList.remove('dragover');
            if (e.dataTransfer.files.length) {
                fileInput.files = e.dataTransfer.files;
                updateFileName();
            }
        });
        
        fileInput.addEventListener('change', updateFileName);
        
        function updateFileName() {
            if (fileInput.files.length) {
                fileName.textContent = fileInput.files[0].name;
                uploadBtn.disabled = false;
            }
        }
        
        uploadForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const formData = new FormData(uploadForm);
            uploadBtn.disabled = true;
            uploadBtn.textContent = 'Uploading...';
            progressBar.classList.add('active');
            
            try {
                const xhr = new XMLHttpRequest();
                xhr.open('POST', uploadForm.action, true);
                
                xhr.upload.onprogress = (e) => {
                    if (e.lengthComputable) {
                        const percent = (e.loaded / e.total) * 100;
                        progressFill.style.width = percent + '%';
                    }
                };
                
                xhr.onload = () => {
                    if (xhr.status === 200) {
                        message.className = 'message success';
                        message.textContent = '✓ File uploaded successfully!';
                        message.style.display = 'block';
                        fileInput.value = '';
                        fileName.textContent = '';
                    } else {
                        throw new Error('Upload failed');
                    }
                    resetForm();
                };
                
                xhr.onerror = () => {
                    throw new Error('Network error');
                };
                
                xhr.send(formData);
            } catch (err) {
                message.className = 'message error';
                message.textContent = '✗ Upload failed. Please try again.';
                message.style.display = 'block';
                resetForm();
            }
        });
        
        function resetForm() {
            uploadBtn.disabled = true;
            uploadBtn.textContent = 'Upload';
            progressBar.classList.remove('active');
            progressFill.style.width = '0%';
        }
    </script>
</body>
</html>`

func (s *ReceiveServer) handleUploadPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("upload").Parse(uploadPageHTML))

	data := struct {
		SaveDir string
		Token   string
	}{
		SaveDir: s.saveDir,
		Token:   s.token,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

func (s *ReceiveServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	clientIP := r.RemoteAddr
	log.Printf("[INFO] Upload started from %s: %s", clientIP, header.Filename)

	destPath := filepath.Join(s.saveDir, header.Filename)

	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(header.Filename)
		name := header.Filename[:len(header.Filename)-len(ext)]
		for i := 1; ; i++ {
			destPath = filepath.Join(s.saveDir, fmt.Sprintf("%s_%d%s", name, i, ext))
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				break
			}
		}
	}

	dest, err := os.Create(destPath)
	if err != nil {
		log.Printf("[ERROR] Failed to create file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dest.Close()

	startTime := time.Now()
	written, err := io.Copy(dest, file)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[ERROR] Failed to save file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Upload completed: %s (%d bytes in %v)", filepath.Base(destPath), written, duration.Round(time.Millisecond))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File uploaded: %s", filepath.Base(destPath))
}
