package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static/*
var staticFS embed.FS

// StaticHandler serves the embedded SPA. App routes (/devices/, etc.) must not go
// through http.FileServer — it issues 301 Location: ./ and causes redirect loops.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return http.NotFoundHandler()
	}
	indexHTML, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return http.NotFoundHandler()
	}
	assetsSub, err := fs.Sub(sub, "assets")
	if err != nil {
		return http.NotFoundHandler()
	}
	assetHandler := http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSub)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			assetHandler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
}
