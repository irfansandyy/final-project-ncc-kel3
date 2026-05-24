package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// ─── Response types (match frontend lib/siem.ts) ────────────────────────────

type siemSummary struct {
	TotalEvents    int `json:"total_events"`
	CriticalAlerts int `json:"critical_alerts"`
	AuthFailures   int `json:"auth_failures"`
	AuthSuccesses  int `json:"auth_successes"`
}

type siemTimeSeriesPoint struct {
	Timestamp string             `json:"timestamp"`
	Counts    map[string]float64 `json:"counts"`
}

type siemMitreTechnique struct {
	Technique  string  `json:"technique"`
	Tactic     string  `json:"tactic"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type siemAgentStat struct {
	AgentID    string  `json:"agent_id"`
	AgentName  string  `json:"agent_name"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
}

type siemOverviewResponse struct {
	Summary           siemSummary           `json:"summary"`
	AlertLevelsSeries []siemTimeSeriesPoint  `json:"alert_levels_series"`
	TopMitre          []siemMitreTechnique   `json:"top_mitre"`
	TopAgents         []siemAgentStat        `json:"top_agents"`
	AgentSeries       []siemTimeSeriesPoint  `json:"agent_series"`
}

// ─── Handler ─────────────────────────────────────────────────────────────────

// GetSiemOverview serves GET /api/siem/overview
// It returns aggregated SIEM statistics for the last 7 days.
func GetSiemOverview(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// ── 1. Summary ────────────────────────────────────────────────────────
		var summary siemSummary

		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE timestamp >= NOW() - INTERVAL '7 days'`).
			Scan(&summary.TotalEvents); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts WHERE severity IN ('CRITICAL','ERROR') AND created_at >= NOW() - INTERVAL '7 days'`).
			Scan(&summary.CriticalAlerts); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE level IN ('ERROR','CRITICAL') AND message ILIKE '%auth%fail%' AND timestamp >= NOW() - INTERVAL '7 days'`).
			Scan(&summary.AuthFailures); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE level = 'INFO' AND message ILIKE '%auth%success%' AND timestamp >= NOW() - INTERVAL '7 days'`).
			Scan(&summary.AuthSuccesses); err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		// ── 2. Alert levels time series (daily, last 7 days) ─────────────────
		// We bucket events by day and level, then pivot into the TimeSeriesPoint format.
		alertLevelsRows, err := db.QueryContext(ctx, `
			SELECT
				to_char(DATE_TRUNC('day', timestamp), 'Mon DD') AS day,
				level,
				COUNT(*) AS cnt
			FROM events
			WHERE timestamp >= NOW() - INTERVAL '7 days'
			GROUP BY day, level
			ORDER BY MIN(timestamp) ASC, level
		`)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer alertLevelsRows.Close()

		// collect into ordered map[day]map[level]count
		dayOrder := []string{}
		levelsByDay := map[string]map[string]float64{}
		for alertLevelsRows.Next() {
			var day, level string
			var cnt float64
			if err := alertLevelsRows.Scan(&day, &level, &cnt); err != nil {
				continue
			}
			if _, ok := levelsByDay[day]; !ok {
				levelsByDay[day] = map[string]float64{}
				dayOrder = append(dayOrder, day)
			}
			levelsByDay[day][level] = cnt
		}

		alertLevelsSeries := make([]siemTimeSeriesPoint, 0, len(dayOrder))
		for _, day := range dayOrder {
			alertLevelsSeries = append(alertLevelsSeries, siemTimeSeriesPoint{
				Timestamp: day,
				Counts:    levelsByDay[day],
			})
		}

		// ── 3. Top MITRE ATT&CK techniques (from alerts metadata) ────────────
		mitreRows, err := db.QueryContext(ctx, `
			SELECT
				COALESCE(metadata->>'technique', 'Unknown') AS technique,
				COALESCE(metadata->>'tactic', '')            AS tactic,
				COUNT(*)                                     AS cnt
			FROM alerts
			WHERE created_at >= NOW() - INTERVAL '7 days'
			  AND metadata->>'technique' IS NOT NULL
			GROUP BY technique, tactic
			ORDER BY cnt DESC
			LIMIT 6
		`)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer mitreRows.Close()

		var mitreItems []siemMitreTechnique
		var mitreTotal int
		for mitreRows.Next() {
			var item siemMitreTechnique
			if err := mitreRows.Scan(&item.Technique, &item.Tactic, &item.Count); err != nil {
				continue
			}
			mitreTotal += item.Count
			mitreItems = append(mitreItems, item)
		}
		for i := range mitreItems {
			if mitreTotal > 0 {
				mitreItems[i].Percentage = math_round((float64(mitreItems[i].Count) / float64(mitreTotal)) * 100)
			}
		}

		// ── 4. Top 5 agents by alert count ───────────────────────────────────
		agentRows, err := db.QueryContext(ctx, `
			SELECT
				COALESCE(metadata->>'agent_id', 'unknown')   AS agent_id,
				COALESCE(metadata->>'agent_name', source)    AS agent_name,
				COUNT(*)                                     AS cnt
			FROM alerts
			WHERE created_at >= NOW() - INTERVAL '7 days'
			GROUP BY agent_id, agent_name
			ORDER BY cnt DESC
			LIMIT 5
		`)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer agentRows.Close()

		var topAgents []siemAgentStat
		var agentTotal int
		for agentRows.Next() {
			var s siemAgentStat
			if err := agentRows.Scan(&s.AgentID, &s.AgentName, &s.Total); err != nil {
				continue
			}
			agentTotal += s.Total
			topAgents = append(topAgents, s)
		}
		for i := range topAgents {
			if agentTotal > 0 {
				topAgents[i].Percentage = math_round((float64(topAgents[i].Total) / float64(agentTotal)) * 100)
			}
		}

		// ── 5. Agent time series (daily, top 5 agents) ───────────────────────
		// Build a list of top agent names for use in the query
		agentNames := make([]string, 0, len(topAgents))
		for _, a := range topAgents {
			agentNames = append(agentNames, a.AgentName)
		}

		agentSeries := []siemTimeSeriesPoint{}
		if len(agentNames) > 0 {
			agentSeriesRows, err := db.QueryContext(ctx, `
				SELECT
					to_char(DATE_TRUNC('day', created_at), 'Mon DD') AS day,
					COALESCE(metadata->>'agent_name', source)         AS agent_name,
					COUNT(*)                                          AS cnt
				FROM alerts
				WHERE created_at >= NOW() - INTERVAL '7 days'
				  AND COALESCE(metadata->>'agent_name', source) = ANY($1)
				GROUP BY day, agent_name
				ORDER BY MIN(created_at) ASC, agent_name
			`, pqStringArray(agentNames))
			if err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			defer agentSeriesRows.Close()

			agentDayOrder := []string{}
			agentByDay := map[string]map[string]float64{}
			for agentSeriesRows.Next() {
				var day, agentName string
				var cnt float64
				if err := agentSeriesRows.Scan(&day, &agentName, &cnt); err != nil {
					continue
				}
				if _, ok := agentByDay[day]; !ok {
					agentByDay[day] = map[string]float64{}
					agentDayOrder = append(agentDayOrder, day)
				}
				agentByDay[day][agentName] = cnt
			}

			for _, day := range agentDayOrder {
				agentSeries = append(agentSeries, siemTimeSeriesPoint{
					Timestamp: day,
					Counts:    agentByDay[day],
				})
			}
		}

		// ── Assemble and respond ──────────────────────────────────────────────
		resp := siemOverviewResponse{
			Summary:           summary,
			AlertLevelsSeries: alertLevelsSeries,
			TopMitre:          mitreItems,
			TopAgents:         topAgents,
			AgentSeries:       agentSeries,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// math_round rounds a float64 to two decimal places for percentage display.
func math_round(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// pqStringArray converts a Go []string into a format compatible with PostgreSQL ANY($1).
// We return an interface that the pgx driver can handle as a text array.
func pqStringArray(s []string) interface{} {
	return s
}
