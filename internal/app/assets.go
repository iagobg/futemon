package app

import "net/http"

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/static/app.js" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	data, err := embeddedFiles.ReadFile("static/app.js")
	if err != nil {
		http.Error(w, "asset not found", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}
