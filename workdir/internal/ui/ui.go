package ui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

// Handler returns an http.Handler that serves the embedded dashboard at /ui.
// If apiKey is non-empty, it is injected into index.html as window.__RUPTURA_KEY__
// so the browser auto-configures authentication without manual entry.
func Handler(apiKey string) http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("ui: embed fs sub failed: " + err.Error())
	}
	fileServer := http.StripPrefix("/ui", http.FileServer(http.FS(sub)))
	if apiKey == "" {
		return fileServer
	}
	inject := []byte("<script>window.__RUPTURA_KEY__=" + jsonString(apiKey) + "</script></head>")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only intercept the main HTML page; all other assets (JS, CSS) go through unchanged.
		p := r.URL.Path
		if p != "/ui/" && p != "/ui/index.html" {
			fileServer.ServeHTTP(w, r)
			return
		}
		raw, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "ui: index.html not found", http.StatusInternalServerError)
			return
		}
		raw = bytes.Replace(raw, []byte("</head>"), inject, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(raw)
	})
}

// jsonString returns a JSON-encoded string literal (with quotes).
// Only handles printable ASCII — sufficient for API keys.
func jsonString(s string) string {
	var b bytes.Buffer
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
