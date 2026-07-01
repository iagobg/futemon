package app

import "net/http"

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/static/app.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		data, err := embeddedFiles.ReadFile("static/app.css")
		if err != nil {
			http.Error(w, "asset not found", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	case "/static/app.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		data, err := embeddedFiles.ReadFile("static/app.js")
		if err != nil {
			http.Error(w, "asset not found", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	default:
		http.NotFound(w, r)
	}
}
