-- =============================================================================
-- 003_siem_seed.sql — Seed realistic SIEM events, rules, and alerts
-- =============================================================================

INSERT INTO log_sources (name, file_path, format) VALUES
  ('Chatbot API',    '/app-logs/chatbot-api.log',  'json'),
  ('NCC Web Server', '/app-logs/ncc-web-srv.log',  'nginx'),
  ('PostgreSQL DB',  '/app-logs/db-postgres.log',  'syslog'),
  ('Redis Cache',    '/app-logs/redis-cache.log',  'syslog'),
  ('Nginx Proxy',    '/app-logs/nginx-proxy.log',  'nginx')
ON CONFLICT DO NOTHING;

INSERT INTO rules (name, description, condition, severity, enabled) VALUES
  ('Brute Force SSH',     'Multiple failed SSH login attempts',
   '{"type":"threshold","field":"message","pattern":"Failed password","threshold":5,"window_seconds":60}',
   'CRITICAL', true),
  ('Auth Failure Spike',  'Unusual auth failure rate',
   '{"type":"threshold","field":"message","pattern":"authentication failure","threshold":10,"window_seconds":300}',
   'HIGH', true),
  ('Prompt Injection',    'Possible LLM prompt injection in chat input',
   '{"type":"pattern","field":"message","pattern":"(?i)(ignore previous|system prompt|jailbreak)"}',
   'HIGH', true),
  ('Session Token Reuse', 'JWT or session token replayed',
   '{"type":"pattern","field":"message","pattern":"(?i)(token reuse|replay attack|session hijack)"}',
   'HIGH', true),
  ('Rootcheck Anomaly',   'Host-based anomaly detected via rootcheck',
   '{"type":"pattern","field":"message","pattern":"(?i)(rootcheck|integrity check failed)"}',
   'WARN', true),
  ('Forbidden Directory', 'Attempt to access forbidden path',
   '{"type":"pattern","field":"message","pattern":"(?i)(403|forbidden directory)"}',
   'WARN', true),
  ('CVE Vulnerability',   'Known CVE detected on host packages',
   '{"type":"pattern","field":"message","pattern":"CVE-[0-9]{4}-[0-9]+"}',
   'HIGH', true),
  ('DB Query Anomaly',    'Unusual DB query volume',
   '{"type":"threshold","field":"message","pattern":"SELECT","threshold":1000,"window_seconds":60}',
   'HIGH', true),
  ('GuardDuty Anomaly',   'AWS GuardDuty anomalous network finding',
   '{"type":"pattern","field":"message","pattern":"(?i)(guardduty|unusual outbound|suspicious)"}',
   'HIGH', true),
  ('SSH Version Scan',    'Possible SSH version-gathering attack',
   '{"type":"pattern","field":"message","pattern":"(?i)(version gathering|ssh scan|sshd.*attack)"}',
   'HIGH', true)
ON CONFLICT (name) DO NOTHING;

DO $$
DECLARE
  src_ids  bigint[];
  rule_ids bigint[];
  i        int;
  src_idx  int;
  lvl      text;
  msg      text;
  ts       timestamptz;
  eid      bigint;
  rid      bigint;
  agent_id   text;
  agent_name text;
  technique  text;
  tactic     text;
  sev        text;
  lvl_num    int;

  messages text[] := ARRAY[
    'Failed password for root from 192.168.1.45 port 22 ssh2',
    'authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=10.0.0.22',
    'Accepted password for deploy from 10.0.0.5 port 51234 ssh2',
    'session opened for user ubuntu by (uid=0)',
    'Prompt injection attempt detected in user input: ignore previous instructions',
    'Host-based anomaly detection event (rootcheck) - integrity check failed',
    'Apache: attempt to access forbidden directory index /etc/passwd',
    'sshd: possible attack on the ssh server (version gathering) from 203.0.113.88',
    'CVE-2019-1010204 affects binutils package installed on this host',
    'Unusual DB query volume - 2340 SELECT queries in 60s - possible data exfiltration',
    'AWS GuardDuty: unusual outbound EC2 communication on port 5060 to 198.51.100.44',
    'GitHub organisation update: member repository creation permission changed',
    'Chatbot session token reuse detected - possible replay attack from 10.0.0.99',
    'OpenSCAP: record events that modify the system network environment (not passed)',
    'Failed password for invalid user admin from 172.16.0.44 port 54321 ssh2',
    'login failed for user api_service from 10.10.0.55',
    'Accepted publickey for ci-runner from 10.0.0.10 port 60001 ssh2',
    'Connection closed by 192.168.2.100 port 44444 [preauth]',
    'Invalid user postgres from 203.0.113.10 port 33219',
    'Received disconnect from 198.51.100.10 port 12345: 11: disconnected by user'
  ];

  agent_ids   text[] := ARRAY['014','001','008','002','005'];
  agent_names text[] := ARRAY['chatbot-api','ncc-web-srv','db-postgres','redis-cache','nginx-proxy'];
  levels      text[] := ARRAY['INFO','INFO','WARN','WARN','ERROR','CRITICAL'];

BEGIN
  SELECT ARRAY_AGG(id ORDER BY id) INTO src_ids  FROM log_sources LIMIT 5;
  SELECT ARRAY_AGG(id ORDER BY id) INTO rule_ids FROM rules LIMIT 10;

  FOR i IN 1..500 LOOP
    src_idx    := (i % 5) + 1;
    ts         := NOW() - (random() * INTERVAL '7 days');
    msg        := messages[(i % array_length(messages,1)) + 1];
    lvl        := levels[(floor(random()*6)+1)::int];
    agent_id   := agent_ids[(src_idx - 1) % 5 + 1];
    agent_name := agent_names[(src_idx - 1) % 5 + 1];

    INSERT INTO events (source_id, timestamp, level, source, message, metadata)
    VALUES (
      src_ids[src_idx], ts, lvl, agent_name, msg,
      jsonb_build_object(
        'agent_id',   agent_id,
        'agent_name', agent_name,
        'host',       agent_name || '.prod',
        'ip',         '10.0.' || (i % 10)::text || '.' || (i % 255 + 1)::text
      )
    )
    RETURNING id INTO eid;

    IF random() < 0.4 AND lvl IN ('ERROR','CRITICAL','WARN') THEN
      rid := rule_ids[(i % array_length(rule_ids,1)) + 1];

      CASE
        WHEN lvl = 'CRITICAL' THEN sev := 'CRITICAL'; lvl_num := 14;
        WHEN lvl = 'ERROR'    THEN sev := 'HIGH';     lvl_num := 10;
        ELSE                       sev := 'WARN';     lvl_num := 7;
      END CASE;

      IF msg ILIKE '%password%' OR msg ILIKE '%auth%' THEN
        technique := 'Brute Force'; tactic := 'Credential Access';
      ELSIF msg ILIKE '%injection%' THEN
        technique := 'T1190'; tactic := 'Initial Access';
      ELSIF msg ILIKE '%exfil%' OR msg ILIKE '%query volume%' THEN
        technique := 'T1114'; tactic := 'Collection';
      ELSIF msg ILIKE '%guardduty%' OR msg ILIKE '%outbound%' THEN
        technique := 'Endpoint DoS'; tactic := 'Impact';
      ELSE
        technique := NULL; tactic := NULL;
      END IF;

      INSERT INTO alerts (rule_id, event_id, severity, status, message, metadata)
      VALUES (
        rid, eid, sev,
        CASE WHEN random() < 0.7 THEN 'open'
             WHEN random() < 0.5 THEN 'acknowledged'
             ELSE 'resolved' END,
        msg,
        jsonb_build_object(
          'agent_id',   agent_id,
          'agent_name', agent_name,
          'level',      lvl_num,
          'technique',  technique,
          'tactic',     tactic
        )
      );
    END IF;
  END LOOP;
END $$;
