package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type Alert struct {
	ID        int64           `json:"id"`
	RuleID    int64           `json:"rule_id"`
	EventID   int64           `json:"event_id"`
	Severity  string          `json:"severity"`
	Status    string          `json:"status"`
	Message   string          `json:"message"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func ListAlerts(db *sql.DB) http.HandlerFunc {
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

		severity := r.URL.Query().Get("severity")
		status := r.URL.Query().Get("status")

		query := "SELECT id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at FROM alerts WHERE 1=1"
		args := []interface{}{}
		argCount := 1

		if severity != "" {
			query += " AND severity = $" + strconv.Itoa(argCount)
			args = append(args, severity)
			argCount++
		}
		if status != "" {
			query += " AND status = $" + strconv.Itoa(argCount)
			args = append(args, status)
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
		query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argCount) + " OFFSET $" + strconv.Itoa(argCount+1)
		args = append(args, limit, offset)

		rows, err := db.QueryContext(r.Context(), query, args...)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		alerts := []Alert{}
		for rows.Next() {
			var a Alert
			var meta []byte
			if err := rows.Scan(&a.ID, &a.RuleID, &a.EventID, &a.Severity, &a.Status, &a.Message, &meta, &a.CreatedAt, &a.UpdatedAt); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			a.Metadata = meta
			alerts = append(alerts, a)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  alerts,
			"total": total,
			"page":  page,
			"limit": limit,
		})
	}
}

func UpdateAlertStatus(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Status != "acknowledged" && req.Status != "resolved" && req.Status != "open" {
			http.Error(w, "invalid status", http.StatusBadRequest)
			return
		}

		query := "UPDATE alerts SET status = $1, updated_at = NOW() WHERE id = $2 RETURNING id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at"
		var a Alert
		var meta []byte
		err = db.QueryRowContext(r.Context(), query, req.Status, id).Scan(&a.ID, &a.RuleID, &a.EventID, &a.Severity, &a.Status, &a.Message, &meta, &a.CreatedAt, &a.UpdatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "alert not found", http.StatusNotFound)
				return
			}
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		a.Metadata = meta

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a)
	}
}
