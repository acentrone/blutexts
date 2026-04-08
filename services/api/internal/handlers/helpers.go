package handlers

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, v interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, message string, status int) {
	writeJSON(w, map[string]string{"error": message}, status)
}
