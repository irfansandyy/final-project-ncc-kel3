package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)


type RuleCondition struct {
	Type          string          `json:"type"` // threshold, pattern, compound
	Field         string          `json:"field,omitempty"`
	Pattern       string          `json:"pattern,omitempty"`
	Threshold     int             `json:"threshold,omitempty"`
	WindowSeconds int             `json:"window_seconds,omitempty"`
	Operator      string          `json:"operator,omitempty"` // AND, OR
	Conditions    []RuleCondition `json:"conditions,omitempty"`
}

type Rule struct {
	ID          int64
	Name        string
	Description string
	Condition   RuleCondition
	Severity    string
}


type Event struct {
	ID        int64
	SourceID  int64
	Timestamp time.Time
	Level     string
	Source    string
	Message   string
	Metadata  string 
}

const connStr = "postgres://postgres:postgres@127.0.0.1:5432/chatdb?sslmode=disable"

func main() {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("rule engine jalan....")

	
	for {
		rules, err := fetchActiveRules(db)
		if err != nil {
			log.Println("Gagal mengambil rules:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, rule := range rules {
			evaluateRule(db, rule)
		}

		time.Sleep(10 * time.Second) 
	}
}


func fetchActiveRules(db *sql.DB) ([]Rule, error) {
	rows, err := db.Query("SELECT id, name, description, condition, severity FROM rules WHERE enabled = true")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		var conditionRaw []byte
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &conditionRaw, &r.Severity); err != nil {
			return nil, err
		}
	
		if err := json.Unmarshal(conditionRaw, &r.Condition); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

//eval log berdasarkan tipe rule
func evaluateRule(db *sql.DB, rule Rule) {
	switch rule.Condition.Type {
	case "pattern":
		query := fmt.Sprintf("SELECT id, message, level FROM events WHERE %s ~* $1 AND created_at >= NOW() - INTERVAL '24 hour'", rule.Condition.Field)
		rows, err := db.Query(query, rule.Condition.Pattern)
		if err != nil {
			log.Printf("Gagal evaluasi pattern rule [%s]: %v", rule.Name, err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var eventID int64
			var msg, lvl string
			rows.Scan(&eventID, &msg, &lvl)
			triggerAlert(db, rule.ID, eventID, rule.Severity, fmt.Sprintf("Pola serangan terdeteksi pada log: %s", rule.Name))
		}

	case "threshold":
		//hitung frekuensi
		var count int
		var latestEventID sql.NullInt64

		query := fmt.Sprintf(`
			SELECT COUNT(*), MAX(id) 
			FROM events 
			WHERE %s = $1 AND timestamp >= NOW() - CAST($2 || ' hour' AS INTERVAL)`, 
			rule.Condition.Field)

		err := db.QueryRow(query, rule.Condition.Pattern, rule.Condition.WindowSeconds).Scan(&count, &latestEventID)
		if err != nil {
			log.Printf("Gagal evaluasi threshold rule [%s]: %v", rule.Name, err)
			return
		}

		if count >= rule.Condition.Threshold && latestEventID.Valid {
			triggerAlert(db, rule.ID, latestEventID.Int64, rule.Severity, fmt.Sprintf("Aktivitas mencurigakan melampaui batas: %s (Terjadi %d kali)", rule.Name, count))
		}

	case "compound":
		var patternCond, thresholdCond RuleCondition
		for _, subCond := range rule.Condition.Conditions {
			if subCond.Type == "pattern" {
				patternCond = subCond
			} else if subCond.Type == "threshold" {
				thresholdCond = subCond
			}
		}

		var count int
		var latestEventID sql.NullInt64

		query := fmt.Sprintf(`
			SELECT COUNT(*), MAX(id) 
			FROM events 
			WHERE %s ~* $1 AND timestamp >= NOW() - CAST($2 || ' hour' AS INTERVAL)`, 
			patternCond.Field, thresholdCond.WindowSeconds)

		err := db.QueryRow(query, patternCond.Pattern, thresholdCond.Threshold).Scan(&count, &latestEventID)
		if err != nil {
			log.Printf("Gagal evaluasi compound rule [%s]: %v", rule.Name, err)
			return
		}

		if count >= thresholdCond.Threshold && latestEventID.Valid {
			triggerAlert(db, rule.ID, latestEventID.Int64, rule.Severity, fmt.Sprintf("Serangan berulang terdeteksi: %s (Terjadi %d kali)", rule.Name, count))
		}
	}
}

func triggerAlert(db *sql.DB, ruleID int64, eventID int64, severity string, message string) {
	var exists bool
	db.QueryRow("SELECT EXISTS(SELECT 1 FROM alerts WHERE rule_id=$1 AND event_id=$2)", ruleID, eventID).Scan(&exists)
	if exists {
		return
	}

	query := `
		INSERT INTO alerts (rule_id, event_id, severity, status, message, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, 'open', $4, '{}', NOW(), NOW())`
	
	_, err := db.Exec(query, ruleID, eventID, severity, message)
	if err != nil {
		log.Println("Gagal membuat alert ke database:", err)
		return
	}
	fmt.Printf("[ALERT %s] %s\n", severity, message)
}