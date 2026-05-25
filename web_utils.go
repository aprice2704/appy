package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/aprice2704/fdm/code/patcheng"
)

func getFileMeta(prof *patcheng.LanguageProfile) (string, string) {
	if prof == nil {
		return "Text", "📄"
	}
	switch prof.ID {
	case "golang":
		return "Go", "🐹"
	case "javascript":
		return "JS", "🟨"
	case "typescript":
		return "TS", "🟦"
	case "python":
		return "Python", "🐍"
	case "markdown":
		return "Markdown", "📝"
	case "neuroscript":
		return "NeuroScript", "🧠"
	case "html":
		return "HTML", "🌐"
	case "css":
		return "CSS", "🎨"
	case "json":
		return "JSON", "📦"
	case "yaml":
		return "YAML", "⚙️"
	case "shell":
		return "Shell", "🐚"
	case "java":
		return "Java", "☕"
	case "cpp":
		return "C++", "⚙️"
	case "astro":
		return "Astro", "🚀"
	case "antlr":
		return "ANTLR", "🛠️"
	default:
		return prof.ID, "📄"
	}
}

func withRecoveryAndCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC: %v", rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				err := json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Server Panic: %v", rec)})
				if err != nil {
					log.Printf("[DEBUG] Failed to encode panic response: %v", err)
				}
			}
		}()
		h(w, r)
	}
}
