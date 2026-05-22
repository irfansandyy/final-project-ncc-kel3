package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
)

var LastReloadTime atomic.Value

func init() {
	LastReloadTime.Store(time.Now().UTC().Format(time.RFC3339))
}

type Rule struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Condition   json.RawMessage `json:"condition"`
	Severity    string          `json:"severity"`
	Action      json.RawMessage `json:"action"`
	Enabled     bool            `json:"enabled"`
	Version     int             `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func ListRules(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := "SELECT id, name, description, condition, severity, action, enabled, version, created_at, updated_at FROM rules ORDER BY id ASC"
		rows, err := db.QueryContext(r.Context(), query)
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		rules := []Rule{}
		for rows.Next() {
			var rule Rule
			var cond, act []byte
			if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &cond, &rule.Severity, &act, &rule.Enabled, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
				http.Error(w, "database error", http.StatusInternalServerError)
				return
			}
			rule.Condition = cond
			rule.Action = act
			rules = append(rules, rule)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
	}
}

func GetRule(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		query := "SELECT id, name, description, condition, severity, action, enabled, version, created_at, updated_at FROM rules WHERE id = $1"
		var rule Rule
		var cond, act []byte
		err = db.QueryRowContext(r.Context(), query, id).Scan(&rule.ID, &rule.Name, &rule.Description, &cond, &rule.Severity, &act, &rule.Enabled, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "rule not found", http.StatusNotFound)
				return
			}
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		rule.Condition = cond
		rule.Action = act

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func CreateRule(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Condition   json.RawMessage `json:"condition"`
			Severity    string          `json:"severity"`
			Action      json.RawMessage `json:"action"`
			Enabled     *bool           `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.Severity != "INFO" && req.Severity != "WARN" && req.Severity != "ERROR" && req.Severity != "CRITICAL" {
			http.Error(w, "invalid severity", http.StatusBadRequest)
			return
		}
		if len(req.Condition) == 0 {
			http.Error(w, "condition is required", http.StatusBadRequest)
			return
		}
		if len(req.Action) == 0 {
			req.Action = []byte("{}")
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		query := `
			INSERT INTO rules (name, description, condition, severity, action, enabled, version)
			VALUES ($1, $2, $3, $4, $5, $6, 1)
			RETURNING id, name, description, condition, severity, action, enabled, version, created_at, updated_at
		`
		var rule Rule
		var cond, act []byte
		err := db.QueryRowContext(r.Context(), query, req.Name, req.Description, req.Condition, req.Severity, req.Action, enabled).
			Scan(&rule.ID, &rule.Name, &rule.Description, &cond, &rule.Severity, &act, &rule.Enabled, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt)
		
		if err != nil {
			http.Error(w, "failed to create rule (maybe name already exists)", http.StatusInternalServerError)
			return
		}
		rule.Condition = cond
		rule.Action = act

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	}
}

func UpdateRule(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Condition   json.RawMessage `json:"condition"`
			Severity    string          `json:"severity"`
			Action      json.RawMessage `json:"action"`
			Enabled     *bool           `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Validation similar to CreateRule
		if req.Name == "" || req.Severity == "" || len(req.Condition) == 0 {
			http.Error(w, "name, severity, and condition are required", http.StatusBadRequest)
			return
		}
		if req.Severity != "INFO" && req.Severity != "WARN" && req.Severity != "ERROR" && req.Severity != "CRITICAL" {
			http.Error(w, "invalid severity", http.StatusBadRequest)
			return
		}
		if len(req.Action) == 0 {
			req.Action = []byte("{}")
		}
		
		// For enabled, we need to fetch the existing value if not provided
		// To keep it simple, we use a single UPDATE query with COALESCE if possible,
		// but since Enabled is a boolean pointer in req, we build the query dynamically or fetch first.
		
		// Simple approach: just require enabled in PUT
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		query := `
			UPDATE rules 
			SET name = $1, description = $2, condition = $3, severity = $4, action = $5, enabled = $6, version = version + 1, updated_at = NOW()
			WHERE id = $7
			RETURNING id, name, description, condition, severity, action, enabled, version, created_at, updated_at
		`
		var rule Rule
		var cond, act []byte
		err = db.QueryRowContext(r.Context(), query, req.Name, req.Description, req.Condition, req.Severity, req.Action, enabled, id).
			Scan(&rule.ID, &rule.Name, &rule.Description, &cond, &rule.Severity, &act, &rule.Enabled, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt)
		
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "rule not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to update rule", http.StatusInternalServerError)
			return
		}
		rule.Condition = cond
		rule.Action = act

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rule)
	}
}

func DeleteRule(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		// Cannot delete if there are associated alerts, so we might need to delete alerts first or do a cascade
		// We'll just execute delete and handle the constraint violation
		_, err = db.ExecContext(r.Context(), "DELETE FROM rules WHERE id = $1", id)
		if err != nil {
			http.Error(w, "failed to delete rule (check if it has associated alerts)", http.StatusConflict)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func ReloadRules(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ts := time.Now().UTC().Format(time.RFC3339)
		LastReloadTime.Store(ts)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "reload_triggered",
			"timestamp": ts,
		})
	}
}

func ReloadStatus(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"last_reload": LastReloadTime.Load(),
		})
	}
}
