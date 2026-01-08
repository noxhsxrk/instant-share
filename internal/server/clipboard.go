package server

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

type ClipboardServer struct {
	httpServer *http.Server
	token      string
	mode       string
	content    string
}

type ClipboardConfig struct {
	Port    int
	Token   string
	Mode    string
	Content string
}

func NewClipboardServer(cfg ClipboardConfig) *ClipboardServer {
	s := &ClipboardServer{
		token:   cfg.Token,
		mode:    cfg.Mode,
		content: cfg.Content,
	}

	mux := http.NewServeMux()

	if cfg.Mode == "clip" {
		mux.HandleFunc(fmt.Sprintf("/%s", cfg.Token), s.handleClipPage)
	} else {
		mux.HandleFunc(fmt.Sprintf("/%s", cfg.Token), s.handlePastePage)
		mux.HandleFunc(fmt.Sprintf("/%s/send", cfg.Token), s.handlePasteSubmit)
	}

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	return s
}

func (s *ClipboardServer) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *ClipboardServer) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

const clipPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Instant Share - Clipboard</title>
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
            max-width: 500px;
            width: 100%;
            border: 1px solid rgba(255, 255, 255, 0.1);
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
        }
        h1 { color: #fff; font-size: 1.8rem; margin-bottom: 10px; text-align: center; }
        .subtitle { color: rgba(255, 255, 255, 0.6); text-align: center; margin-bottom: 30px; font-size: 0.9rem; }
        .content-box {
            background: rgba(0, 0, 0, 0.3);
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 20px;
            max-height: 300px;
            overflow-y: auto;
        }
        .content-text {
            color: #fff;
            font-family: 'SF Mono', Monaco, 'Courier New', monospace;
            font-size: 0.9rem;
            white-space: pre-wrap;
            word-break: break-all;
            line-height: 1.6;
        }
        .btn {
            width: 100%;
            padding: 15px;
            background: linear-gradient(135deg, #10b981 0%, #059669 100%);
            border: none;
            border-radius: 10px;
            color: #fff;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 10px 30px -10px rgba(16, 185, 129, 0.5); }
        .btn:active { transform: scale(0.98); }
        .btn.copied { background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); }
        .message { padding: 15px; border-radius: 10px; margin-top: 20px; text-align: center; display: none; }
        .message.success { background: rgba(34, 197, 94, 0.2); color: #22c55e; display: block; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📋 Clipboard</h1>
        <p class="subtitle">Text from Mac</p>
        <div class="content-box">
            <p class="content-text" id="content">{{.Content}}</p>
        </div>
        <button class="btn" id="copyBtn" onclick="copyText()">📋 Copy to Clipboard</button>
        <div class="message" id="message"></div>
    </div>
    <script>
        function copyText() {
            const text = document.getElementById('content').textContent;
            const btn = document.getElementById('copyBtn');
            const msg = document.getElementById('message');
            
            // Create textarea for fallback copy (works over HTTP)
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.left = '-9999px';
            document.body.appendChild(textarea);
            textarea.select();
            textarea.setSelectionRange(0, 99999);
            
            let success = false;
            try {
                success = document.execCommand('copy');
            } catch (err) {
                console.error('Copy failed:', err);
            }
            
            document.body.removeChild(textarea);
            
            if (success) {
                btn.textContent = '✓ Copied!';
                btn.classList.add('copied');
                msg.className = 'message success';
                msg.textContent = 'Text copied to clipboard!';
                setTimeout(() => {
                    btn.textContent = '📋 Copy to Clipboard';
                    btn.classList.remove('copied');
                }, 2000);
            } else {
                msg.className = 'message';
                msg.style.display = 'block';
                msg.style.background = 'rgba(239, 68, 68, 0.2)';
                msg.style.color = '#ef4444';
                msg.textContent = 'Copy failed. Please select and copy manually.';
            }
        }
    </script>
</body>
</html>`

const pastePageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Instant Share - Paste</title>
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
            max-width: 500px;
            width: 100%;
            border: 1px solid rgba(255, 255, 255, 0.1);
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
        }
        h1 { color: #fff; font-size: 1.8rem; margin-bottom: 10px; text-align: center; }
        .subtitle { color: rgba(255, 255, 255, 0.6); text-align: center; margin-bottom: 30px; font-size: 0.9rem; }
        textarea {
            width: 100%;
            height: 200px;
            background: rgba(0, 0, 0, 0.3);
            border: 2px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            padding: 15px;
            color: #fff;
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            font-size: 1rem;
            resize: vertical;
            margin-bottom: 20px;
        }
        textarea:focus { outline: none; border-color: #6366f1; }
        textarea::placeholder { color: rgba(255, 255, 255, 0.4); }
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
        .message { padding: 15px; border-radius: 10px; margin-top: 20px; text-align: center; display: none; }
        .message.success { background: rgba(34, 197, 94, 0.2); color: #22c55e; display: block; }
        .message.error { background: rgba(239, 68, 68, 0.2); color: #ef4444; display: block; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📝 Send to Mac</h1>
        <p class="subtitle">Text will be copied to Mac clipboard</p>
        <form id="pasteForm">
            <textarea id="textInput" name="text" placeholder="Type or paste text here..." required></textarea>
            <button type="submit" class="btn" id="sendBtn">📤 Send to Mac</button>
        </form>
        <div class="message" id="message"></div>
    </div>
    <script>
        document.getElementById('pasteForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = document.getElementById('sendBtn');
            const msg = document.getElementById('message');
            const text = document.getElementById('textInput').value;
            
            btn.disabled = true;
            btn.textContent = 'Sending...';
            
            try {
                const resp = await fetch('/{{.Token}}/send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: 'text=' + encodeURIComponent(text)
                });
                
                if (resp.ok) {
                    msg.className = 'message success';
                    msg.textContent = '✓ Text sent to Mac clipboard!';
                    document.getElementById('textInput').value = '';
                } else {
                    throw new Error('Failed');
                }
            } catch (err) {
                msg.className = 'message error';
                msg.textContent = '✗ Failed to send. Try again.';
            }
            
            btn.disabled = false;
            btn.textContent = '📤 Send to Mac';
        });
    </script>
</body>
</html>`

func (s *ClipboardServer) handleClipPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("clip").Parse(clipPageHTML))
	data := struct{ Content string }{Content: s.content}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

func (s *ClipboardServer) handlePastePage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("paste").Parse(pastePageHTML))
	data := struct{ Token string }{Token: s.token}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

func (s *ClipboardServer) handlePasteSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "No text provided", http.StatusBadRequest)
		return
	}

	clientIP := r.RemoteAddr
	log.Printf("[INFO] Receiving clipboard from %s (%d chars)", clientIP, len(text))

	cmd := exec.Command("pbcopy")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[ERROR] Failed to open pbcopy: %v", err)
		http.Error(w, "Failed to copy", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[ERROR] Failed to start pbcopy: %v", err)
		http.Error(w, "Failed to copy", http.StatusInternalServerError)
		return
	}

	io.WriteString(stdin, text)
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		log.Printf("[ERROR] pbcopy failed: %v", err)
		http.Error(w, "Failed to copy", http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Clipboard updated: %d chars", len(text))
	w.WriteHeader(http.StatusOK)
}

func GetMacClipboard() (string, error) {
	cmd := exec.Command("pbpaste")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
