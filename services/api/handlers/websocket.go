package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for MVP
	},
}

type Hub struct {
	clients        map[*websocket.Conn]bool
	broadcast      chan []byte
	register       chan *websocket.Conn
	unregister     chan *websocket.Conn
	db             *sql.DB
	pollInterval   time.Duration
	lastEventID    int64
	lastAlertID    int64
	mu             sync.Mutex
	logger         *slog.Logger
}

func NewHub(db *sql.DB, pollIntervalMs int, logger *slog.Logger) *Hub {
	return &Hub{
		clients:      make(map[*websocket.Conn]bool),
		broadcast:    make(chan []byte, 256),
		register:     make(chan *websocket.Conn),
		unregister:   make(chan *websocket.Conn),
		db:           db,
		pollInterval: time.Duration(pollIntervalMs) * time.Millisecond,
		logger:       logger,
	}
}

func (h *Hub) Run(ctx context.Context) {
	// Initialize last IDs
	h.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM events").Scan(&h.lastEventID)
	h.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM alerts").Scan(&h.lastAlertID)

	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("websocket hub stopping")
			// Close all connections
			h.mu.Lock()
			for client := range h.clients {
				client.Close()
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		case <-ticker.C:
			h.pollEvents(ctx)
			h.pollAlerts(ctx)
		}
	}
}

func (h *Hub) pollEvents(ctx context.Context) {
	query := "SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events WHERE id > $1 ORDER BY id ASC LIMIT 50"
	rows, err := h.db.QueryContext(ctx, query, h.lastEventID)
	if err != nil {
		h.logger.Error("failed to poll events", "error", err)
		return
	}
	defer rows.Close()

	var maxID int64
	for rows.Next() {
		var e Event
		var meta []byte
		var raw sql.NullString
		if err := rows.Scan(&e.ID, &e.SourceID, &e.Timestamp, &e.Level, &e.Source, &e.Message, &raw, &meta, &e.CreatedAt); err != nil {
			continue
		}
		e.Raw = raw.String
		e.Metadata = meta

		if e.ID > maxID {
			maxID = e.ID
		}

		msg := map[string]interface{}{
			"type": "new_event",
			"data": e,
		}
		bytes, _ := json.Marshal(msg)
		h.broadcast <- bytes
	}

	if maxID > h.lastEventID {
		h.lastEventID = maxID
	}
}

func (h *Hub) pollAlerts(ctx context.Context) {
	query := "SELECT id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at FROM alerts WHERE id > $1 ORDER BY id ASC LIMIT 50"
	rows, err := h.db.QueryContext(ctx, query, h.lastAlertID)
	if err != nil {
		h.logger.Error("failed to poll alerts", "error", err)
		return
	}
	defer rows.Close()

	var maxID int64
	for rows.Next() {
		var a Alert
		var meta []byte
		if err := rows.Scan(&a.ID, &a.RuleID, &a.EventID, &a.Severity, &a.Status, &a.Message, &meta, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		a.Metadata = meta

		if a.ID > maxID {
			maxID = a.ID
		}

		msg := map[string]interface{}{
			"type": "new_alert",
			"data": a,
		}
		bytes, _ := json.Marshal(msg)
		h.broadcast <- bytes
	}

	if maxID > h.lastAlertID {
		h.lastAlertID = maxID
	}
}

func (h *Hub) HandleWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			h.logger.Error("failed to upgrade websocket", "error", err)
			return
		}
		h.register <- conn

		// Read loop to detect disconnects
		go func() {
			defer func() {
				h.unregister <- conn
			}()
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					break
				}
			}
		}()
	}
}
