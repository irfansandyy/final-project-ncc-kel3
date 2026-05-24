package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// siemAlertItem matches the frontend SecurityAlert type in lib/siem.ts exactly.
type siemAlertItem struct {
	ID          int64      `json:"id"`
	Timestamp   time.Time  `json:"timestamp"`
	AgentID     string     `json:"agent_id"`
	AgentName   string     `json:"agent_name"`
	Technique   *string    `json:"technique"`
	Tactic      *string    `json:"tactic"`
	Description string     `json:"description"`
	Level       int        `json:"level"`
	RuleID      string     `json:"rule_id"`
}

type siemAlertsPage struct {
	Items    []siemAlertItem `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// GetSiemAlerts serves GET /api/siem/alerts?page=1&page_size=10
// It maps the raw alerts table to the frontend SecurityAlert shape by
// extracting agent_id, agent_name, technique, tactic from the metadata JSONB
// column and deriving a numeric level from the alert severity.
func GetSiemAlerts(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 10
		}
		offset := (page - 1) * pageSize

		// Count total
		var total int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts`).Scan(&total); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		rows, err := db.QueryContext(ctx, `
			SELECT
				a.id,
				COALESCE(e.timestamp, a.created_at)                AS timestamp,
				COALESCE(a.metadata->>'agent_id',   'unknown')     AS agent_id,
				COALESCE(a.metadata->>'agent_name',
				         e.source,
				         'unknown')                                 AS agent_name,
				a.metadata->>'technique'                           AS technique,
				a.metadata->>'tactic'                              AS tactic,
				a.message                                          AS description,
				a.severity,
				COALESCE(a.rule_id::text, '0')                     AS rule_id_str
			FROM alerts a
			LEFT JOIN events e ON e.id = a.event_id
			ORDER BY a.created_at DESC
			LIMIT $1 OFFSET $2
		`, pageSize, offset)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		items := []siemAlertItem{}
		for rows.Next() {
			var item siemAlertItem
			var severity string
			if err := rows.Scan(
				&item.ID,
				&item.Timestamp,
				&item.AgentID,
				&item.AgentName,
				&item.Technique,
				&item.Tactic,
				&item.Description,
				&severity,
				&item.RuleID,
			); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			item.Level = severityToLevel(severity)
			items = append(items, item)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(siemAlertsPage{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		})
	}
}

// severityToLevel maps a text severity to a Wazuh-style numeric level so
// the frontend level colour/pill logic works correctly.
func severityToLevel(severity string) int {
	switch severity {
	case "CRITICAL":
		return 14
	case "ERROR":
		return 10
	case "WARN":
		return 6
	default: // INFO
		return 3
	}
}
