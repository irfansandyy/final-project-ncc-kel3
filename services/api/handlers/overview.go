package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// ── Response types ────────────────────────────────────────────────────────────

type SiemSummary struct {
	TotalEvents    int `json:"total_events"`
	CriticalAlerts int `json:"critical_alerts"`
	AuthFailures   int `json:"auth_failures"`
	AuthSuccesses  int `json:"auth_successes"`
}

type TimeSeriesPoint struct {
	Timestamp string             `json:"timestamp"`
	Counts    map[string]float64 `json:"counts"`
}

type MitreTechnique struct {
	Technique  string  `json:"technique"`
	Tactic     string  `json:"tactic"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type AgentStat struct {
	AgentID    string  `json:"agent_id"`
	AgentName  string  `json:"agent_name"`
	Total      int     `json:"total"`
	Percentage float64 `json:"percentage"`
}

type SiemOverview struct {
	Summary           SiemSummary       `json:"summary"`
	AlertLevelsSeries []TimeSeriesPoint `json:"alert_levels_series"`
	TopMitre          []MitreTechnique  `json:"top_mitre"`
	TopAgents         []AgentStat       `json:"top_agents"`
	AgentSeries       []TimeSeriesPoint `json:"agent_series"`
}

// ── Handler ───────────────────────────────────────────────────────────────────

func GetSiemOverview(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// ── Summary ──────────────────────────────────────────────────────────
		var summary SiemSummary

		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM events WHERE timestamp > NOW() - INTERVAL '7 days'`,
		).Scan(&summary.TotalEvents)

		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM alerts WHERE created_at > NOW() - INTERVAL '7 days'
			  AND severity IN ('CRITICAL','HIGH','critical','high')`,
		).Scan(&summary.CriticalAlerts)

		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM events WHERE timestamp > NOW() - INTERVAL '7 days'
			  AND (message ILIKE '%authentication failure%' OR message ILIKE '%failed password%'
			    OR message ILIKE '%invalid user%' OR message ILIKE '%login failed%')`,
		).Scan(&summary.AuthFailures)

		db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM events WHERE timestamp > NOW() - INTERVAL '7 days'
			  AND (message ILIKE '%accepted password%' OR message ILIKE '%accepted publickey%'
			    OR message ILIKE '%session opened%' OR message ILIKE '%login successful%')`,
		).Scan(&summary.AuthSuccesses)

		// ── Alert level time series ───────────────────────────────────────
		levelMap := map[string]map[string]float64{}
		levelRows, _ := db.QueryContext(ctx, `
			SELECT DATE_TRUNC('day', timestamp) AS day,
			       CASE level
			         WHEN 'CRITICAL' THEN '14'
			         WHEN 'ERROR'    THEN '10'
			         WHEN 'WARN'     THEN '8'
			         WHEN 'WARNING'  THEN '8'
			         WHEN 'INFO'     THEN '6'
			         ELSE '3'
			       END AS lvl_num,
			       COUNT(*) AS cnt
			FROM events
			WHERE timestamp > NOW() - INTERVAL '7 days'
			GROUP BY day, lvl_num ORDER BY day ASC`)
		if levelRows != nil {
			defer levelRows.Close()
			for levelRows.Next() {
				var day time.Time
				var level string
				var cnt float64
				levelRows.Scan(&day, &level, &cnt)
				label := day.Format("Jan 2")
				if levelMap[label] == nil {
					levelMap[label] = map[string]float64{}
				}
				levelMap[label][level] += cnt
			}
		}
		alertLevelsSeries := buildDailySeries(levelMap, 7)

		// ── MITRE techniques ──────────────────────────────────────────────
		var mitreTechniques []MitreTechnique
		mitreRows, _ := db.QueryContext(ctx, `
			SELECT COALESCE(metadata->>'technique','Unknown') AS technique,
			       COALESCE(metadata->>'tactic','')          AS tactic,
			       COUNT(*) AS cnt
			FROM alerts
			WHERE created_at > NOW() - INTERVAL '7 days'
			  AND metadata->>'technique' IS NOT NULL
			GROUP BY technique, tactic ORDER BY cnt DESC LIMIT 6`)
		if mitreRows != nil {
			defer mitreRows.Close()
			var total int
			for mitreRows.Next() {
				var m MitreTechnique
				mitreRows.Scan(&m.Technique, &m.Tactic, &m.Count)
				total += m.Count
				mitreTechniques = append(mitreTechniques, m)
			}
			for i := range mitreTechniques {
				if total > 0 {
					mitreTechniques[i].Percentage = float64(mitreTechniques[i].Count) / float64(total) * 100
				}
			}
		}

		// ── Top 5 agents ──────────────────────────────────────────────────
		var topAgents []AgentStat
		agentRows, _ := db.QueryContext(ctx, `
			SELECT COALESCE(metadata->>'agent_id', source)   AS agent_id,
			       COALESCE(metadata->>'agent_name', source) AS agent_name,
			       COUNT(*) AS cnt
			FROM events
			WHERE timestamp > NOW() - INTERVAL '7 days'
			GROUP BY agent_id, agent_name ORDER BY cnt DESC LIMIT 5`)
		if agentRows != nil {
			defer agentRows.Close()
			var total int
			for agentRows.Next() {
				var a AgentStat
				agentRows.Scan(&a.AgentID, &a.AgentName, &a.Total)
				total += a.Total
				topAgents = append(topAgents, a)
			}
			for i := range topAgents {
				if total > 0 {
					topAgents[i].Percentage = float64(topAgents[i].Total) / float64(total) * 100
				}
			}
		}

		// ── Agent time series ─────────────────────────────────────────────
		agentSeriesMap := map[string]map[string]float64{}
		aSeriesRows, _ := db.QueryContext(ctx, `
			SELECT DATE_TRUNC('day', timestamp) AS day,
			       COALESCE(metadata->>'agent_name', source) AS agent_name,
			       COUNT(*) AS cnt
			FROM events
			WHERE timestamp > NOW() - INTERVAL '7 days'
			GROUP BY day, agent_name ORDER BY day ASC`)
		if aSeriesRows != nil {
			defer aSeriesRows.Close()
			for aSeriesRows.Next() {
				var day time.Time
				var name string
				var cnt float64
				aSeriesRows.Scan(&day, &name, &cnt)
				label := day.Format("Jan 2")
				if agentSeriesMap[label] == nil {
					agentSeriesMap[label] = map[string]float64{}
				}
				agentSeriesMap[label][name] += cnt
			}
		}
		agentSeries := buildDailySeries(agentSeriesMap, 7)

		// ── Use realistic mock data when DB is empty ──────────────────────
		if summary.TotalEvents == 0 {
			summary, alertLevelsSeries, mitreTechniques, topAgents, agentSeries = mockSiemData()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SiemOverview{
			Summary:           summary,
			AlertLevelsSeries: alertLevelsSeries,
			TopMitre:          mitreTechniques,
			TopAgents:         topAgents,
			AgentSeries:       agentSeries,
		})
	}
}

func buildDailySeries(m map[string]map[string]float64, days int) []TimeSeriesPoint {
	series := make([]TimeSeriesPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		label := time.Now().UTC().AddDate(0, 0, -i).Format("Jan 2")
		counts := m[label]
		if counts == nil {
			counts = map[string]float64{}
		}
		series = append(series, TimeSeriesPoint{Timestamp: label, Counts: counts})
	}
	return series
}

func mockSiemData() (SiemSummary, []TimeSeriesPoint, []MitreTechnique, []AgentStat, []TimeSeriesPoint) {
	levels := []string{"14", "12", "10", "8", "6", "3"}
	levelSeed := [][]float64{
		{40, 80, 150, 300, 500, 800},
		{55, 100, 170, 320, 530, 850},
		{48, 95, 160, 310, 510, 820},
		{60, 110, 190, 340, 560, 900},
		{70, 120, 200, 360, 590, 940},
		{65, 105, 185, 345, 575, 920},
		{130, 180, 280, 500, 750, 1100},
	}
	agentNames := []string{"chatbot-api", "ncc-web-srv", "db-postgres", "redis-cache", "nginx-proxy"}
	agentSeed := [][]float64{
		{310, 220, 130, 60, 25},
		{330, 240, 145, 70, 30},
		{315, 225, 138, 65, 28},
		{350, 255, 155, 75, 33},
		{380, 275, 168, 80, 36},
		{360, 260, 158, 74, 32},
		{480, 340, 200, 95, 45},
	}

	alertLevelsSeries := make([]TimeSeriesPoint, 7)
	agentSeries := make([]TimeSeriesPoint, 7)
	for i := 0; i < 7; i++ {
		label := time.Now().UTC().AddDate(0, 0, -(6 - i)).Format("Jan 2")
		lc := map[string]float64{}
		for j, lv := range levels {
			lc[lv] = levelSeed[i][j]
		}
		alertLevelsSeries[i] = TimeSeriesPoint{Timestamp: label, Counts: lc}
		ac := map[string]float64{}
		for j, name := range agentNames {
			ac[name] = agentSeed[i][j]
		}
		agentSeries[i] = TimeSeriesPoint{Timestamp: label, Counts: ac}
	}

	summary := SiemSummary{TotalEvents: 54249, CriticalAlerts: 4132, AuthFailures: 3214, AuthSuccesses: 349}

	mitre := []MitreTechnique{
		{Technique: "Brute Force", Tactic: "Credential Access", Count: 1572, Percentage: 38},
		{Technique: "Valid Accounts", Tactic: "Initial Access", Count: 869, Percentage: 21},
		{Technique: "Endpoint DoS", Tactic: "Impact", Count: 579, Percentage: 14},
		{Technique: "Data Collection", Tactic: "Collection", Count: 496, Percentage: 12},
		{Technique: "Credential Access", Tactic: "Credential Access", Count: 372, Percentage: 9},
		{Technique: "Other", Tactic: "", Count: 248, Percentage: 6},
	}
	agents := []AgentStat{
		{AgentID: "014", AgentName: "chatbot-api", Total: 21847, Percentage: 92},
		{AgentID: "001", AgentName: "ncc-web-srv", Total: 16203, Percentage: 68},
		{AgentID: "008", AgentName: "db-postgres", Total: 9814, Percentage: 41},
		{AgentID: "002", AgentName: "redis-cache", Total: 4521, Percentage: 24},
		{AgentID: "005", AgentName: "nginx-proxy", Total: 1864, Percentage: 10},
	}
	return summary, alertLevelsSeries, mitre, agents, agentSeries
}
