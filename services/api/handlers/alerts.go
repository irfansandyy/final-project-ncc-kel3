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
		if limit < 1 {
			// frontend sends page_size
			limit, _ = strconv.Atoi(r.URL.Query().Get("page_size"))
		}
		if limit < 1 || limit > 200 {
			limit = 10
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

		// Map DB alerts to the SecurityAlert shape expected by the frontend
		type SecurityAlert struct {
			ID          int64           `json:"id"`
			Timestamp   string          `json:"timestamp"`
			AgentID     string          `json:"agent_id"`
			AgentName   string          `json:"agent_name"`
			Technique   *string         `json:"technique"`
			Tactic      *string         `json:"tactic"`
			Description string          `json:"description"`
			Level       int             `json:"level"`
			RuleID      string          `json:"rule_id"`
		}

		items := make([]SecurityAlert, 0, len(alerts))
		for _, a := range alerts {
			var technique, tactic *string
			if a.Metadata != nil {
				var meta map[string]interface{}
				if json.Unmarshal(a.Metadata, &meta) == nil {
					if v, ok := meta["technique"].(string); ok && v != "" {
						technique = &v
					}
					if v, ok := meta["tactic"].(string); ok && v != "" {
						tactic = &v
					}
				}
			}
			agentID := ""
			agentName := ""
			levelNum := 0
			if a.Metadata != nil {
				var meta map[string]interface{}
				if json.Unmarshal(a.Metadata, &meta) == nil {
					if v, ok := meta["agent_id"].(string); ok {
						agentID = v
					}
					if v, ok := meta["agent_name"].(string); ok {
						agentName = v
					}
					if v, ok := meta["level"].(float64); ok {
						levelNum = int(v)
					}
				}
			}
			// Map severity to numeric level if not in metadata
			if levelNum == 0 {
				switch a.Severity {
				case "CRITICAL", "critical":
					levelNum = 14
				case "HIGH", "high":
					levelNum = 10
				case "WARN", "warn", "WARNING":
					levelNum = 7
				default:
					levelNum = 3
				}
			}
			items = append(items, SecurityAlert{
				ID:          a.ID,
				Timestamp:   a.CreatedAt.UTC().Format("2006-01-02T15:04:05.999Z"),
				AgentID:     agentID,
				AgentName:   agentName,
				Technique:   technique,
				Tactic:      tactic,
				Description: a.Message,
				Level:       levelNum,
				RuleID:      strconv.FormatInt(a.RuleID, 10),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": limit,
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
