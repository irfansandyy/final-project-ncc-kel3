# SIEM Subsystem Documentation

This document outlines the architecture, database design, and API reference for the **Security Information and Event Management (SIEM)** subsystem, specifically covering the components built by Engineer 1 (E1).

## 1. Architectural Overview

The SIEM pipeline is broken into three distinct Go microservices that use PostgreSQL as both a persistent datastore and an asynchronous message bus.

1. **Log Collector (`services/log-collector`)**
   - **Role:** Watches local log files dynamically (via `fsnotify`).
   - **Behavior:** As new lines are appended to monitored files (e.g., backend app logs, Nginx access logs), it batches the raw text lines and inserts them into the `raw_logs` database table.

2. **Log Parser (`services/log-parser`)**
   - **Role:** Asynchronously polls the `raw_logs` table for unprocessed lines.
   - **Behavior:** Parses the raw strings using format-specific plugins (Syslog, NGINX, generic JSON). It extracts the core fields (timestamp, severity level, message) and dumps any custom/extra fields into a flexible `metadata` JSONB column.
   - **Storage:** Successfully parsed logs are persisted into the final `events` table.

3. **SIEM REST API (`services/api`)**
   - **Role:** The gateway for the E3 Frontend Dashboard and the E2 Rule Evaluator.
   - **Behavior:** Provides CRUD endpoints for detection rules, log sources, and alerts. Provides read-heavy, paginated search endpoints for stored events.
   - **WebSockets:** Embeds a real-time WebSocket hub that broadcasts newly parsed events and alerts directly to the frontend.

---

## 2. Database Schema

Yes, **the system already has a dedicated database table for all events**. It is deployed automatically via `database/init/002_siem_schema.sql` on the PostgreSQL container.

### The `events` Table
This is the heart of the SIEM where all normalized logs are stored persistently.
```sql
CREATE TABLE events (
    id         BIGSERIAL    PRIMARY KEY,
    source_id  BIGINT       REFERENCES log_sources(id),
    timestamp  TIMESTAMPTZ  NOT NULL,
    level      TEXT         NOT NULL DEFAULT 'INFO',
    source     TEXT         NOT NULL,
    message    TEXT         NOT NULL,
    raw        TEXT,
    metadata   JSONB        NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```
- **`level`**: Normalized strictly to `INFO`, `WARN`, `ERROR`, or `CRITICAL`.
- **`metadata`**: A native JSONB column in PostgreSQL. This allows the E2 rule engine to run blazing-fast SQL queries on nested JSON properties (e.g., querying for specific HTTP status codes or IP addresses extracted from an NGINX log).

### Other Core Tables
- **`raw_logs`**: The temporary staging queue connecting the Collector and the Parser.
- **`log_sources`**: The registry of files to watch. (The system comes pre-seeded watching the backend app via `/app-logs/backend.log`).
- **`rules`**: Stores the JSON-based detection logic.
- **`alerts`**: Stores triggered security incidents.

---

## 3. Running Locally

The entire stack is containerized using multi-stage Alpine Docker images.

**Start the stack:**
```bash
docker compose up --build -d
```

**Check the background services:**
```bash
docker compose logs -f log-collector
docker compose logs -f log-parser
```

---

## 4. API Reference

The SIEM API runs on port `8080` (`http://localhost:8080/api/siem`).

### Events
*   `GET /events`
    *   **Query Params:** `page`, `limit`, `level`, `source_id`
    *   **Description:** Fetch a paginated list of parsed log events.
*   `GET /events/{id}`
    *   **Description:** Fetch a single event by ID.

### Rules
*   `GET /rules` - List all detection rules.
*   `POST /rules` - Create a new rule.
    *   *Payload expects a `condition` JSON object and an `action` JSON object.*
*   `PUT /rules/{id}` - Update a rule.
*   `DELETE /rules/{id}` - Delete a rule.
*   `POST /rules/reload` - Signals the background Rule Engine to flush its cache and reload rules from the DB.

### Log Sources
*   `GET /log-sources` - List all actively watched files.
*   `POST /log-sources` - Start watching a new file path.
*   `DELETE /log-sources/{id}` - Stop watching a file.

### Alerts
*   `GET /alerts` - List generated security alerts.
*   `PATCH /alerts/{id}` - Update alert status (`open`, `acknowledged`, `resolved`).

---

## 5. WebSockets

**Endpoint:** `ws://localhost:8080/ws`

Clients connected to this hub will receive live JSON payloads. The system polls the DB every 500ms for fresh data.

**Payload Format:**
```json
{
  "type": "new_event",  // or "new_alert"
  "data": { ... standard event/alert JSON ... }
}
```
