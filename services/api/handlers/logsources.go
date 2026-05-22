package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type LogSource struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	FilePath  string    `json:"file_path"`
	Format    string    `json:"format"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ListLogSources(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := "SELECT id, name, file_path, format, enabled, created_at, updated_at FROM log_sources ORDER BY id ASC"
		rows, err := db.QueryContext(r.Context(), query)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		sources := []LogSource{}
		for rows.Next() {
			var s LogSource
			if err := rows.Scan(&s.ID, &s.Name, &s.FilePath, &s.Format, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			sources = append(sources, s)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sources)
	}
}

func CreateLogSource(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name     string `json:"name"`
			FilePath string `json:"file_path"`
			Format   string `json:"format"`
			Enabled  *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" || req.FilePath == "" {
			http.Error(w, "name and file_path are required", http.StatusBadRequest)
			return
		}
		
		if req.Format == "" {
			req.Format = "auto"
		} else if req.Format != "syslog" && req.Format != "nginx" && req.Format != "json" && req.Format != "auto" {
			http.Error(w, "invalid format", http.StatusBadRequest)
			return
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		query := `
			INSERT INTO log_sources (name, file_path, format, enabled)
			VALUES ($1, $2, $3, $4)
			RETURNING id, name, file_path, format, enabled, created_at, updated_at
		`
		var s LogSource
		err := db.QueryRowContext(r.Context(), query, req.Name, req.FilePath, req.Format, enabled).
			Scan(&s.ID, &s.Name, &s.FilePath, &s.Format, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
		
		if err != nil {
			http.Error(w, "failed to create log source (maybe file_path already exists)", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(s)
	}
}

func DeleteLogSource(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		_, err = db.ExecContext(r.Context(), "DELETE FROM log_sources WHERE id = $1", id)
		if err != nil {
			http.Error(w, "failed to delete log source (check if it has associated events/raw_logs)", http.StatusConflict)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
