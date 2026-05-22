package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type Event struct {
	ID        int64           `json:"id"`
	SourceID  int64           `json:"source_id"`
	Timestamp time.Time       `json:"timestamp"`
	Level     string          `json:"level"`
	Source    string          `json:"source"`
	Message   string          `json:"message"`
	Raw       string          `json:"raw"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

func ListEvents(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit < 1 || limit > 200 {
			limit = 50
		}
		offset := (page - 1) * limit

		level := r.URL.Query().Get("level")
		source := r.URL.Query().Get("source")

		query := "SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events WHERE 1=1"
		args := []interface{}{}
		argCount := 1

		if level != "" {
			query += " AND level = $" + strconv.Itoa(argCount)
			args = append(args, level)
			argCount++
		}
		if source != "" {
			query += " AND source = $" + strconv.Itoa(argCount)
			args = append(args, source)
			argCount++
		}

		// Count total
		countQuery := "SELECT COUNT(*) FROM (" + query + ") AS q"
		var total int
		if err := db.QueryRowContext(r.Context(), countQuery, args...).Scan(&total); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		// Fetch data
		query += " ORDER BY timestamp DESC LIMIT $" + strconv.Itoa(argCount) + " OFFSET $" + strconv.Itoa(argCount+1)
		args = append(args, limit, offset)

		rows, err := db.QueryContext(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		events := []Event{}
		for rows.Next() {
			var e Event
			var meta []byte
			var raw sql.NullString
			if err := rows.Scan(&e.ID, &e.SourceID, &e.Timestamp, &e.Level, &e.Source, &e.Message, &raw, &meta, &e.CreatedAt); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			e.Raw = raw.String
			e.Metadata = meta
			events = append(events, e)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  events,
			"total": total,
			"page":  page,
			"limit": limit,
		})
	}
}

func GetEvent(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		query := "SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events WHERE id = $1"
		var e Event
		var meta []byte
		var raw sql.NullString
		err = db.QueryRowContext(r.Context(), query, id).Scan(&e.ID, &e.SourceID, &e.Timestamp, &e.Level, &e.Source, &e.Message, &raw, &meta, &e.CreatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "event not found", http.StatusNotFound)
				return
			}
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		e.Raw = raw.String
		e.Metadata = meta

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	}
}
