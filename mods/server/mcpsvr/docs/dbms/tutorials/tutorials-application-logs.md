# Tutorial 2: Application Logs

Learn how to store and search application logs using Machbase Log tables. This tutorial demonstrates efficient log management with full-text search capabilities.

## Scenario

You're managing a web application that generates:
- HTTP access logs
- Application error logs
- User activity events
- Millions of log entries per day

**Goal**: Store logs efficiently, search for specific errors, and analyze patterns over time.

## What You'll Learn

- Creating Log tables for event data
- Storing high-volume logs
- Full-text search with SEARCH keyword
- Time-based queries with DURATION
- Log retention strategies

## Step 1: Create Log Tables

```sql
-- Application error logs
CREATE TABLE app_error_logs (
    level VARCHAR(10),
    component VARCHAR(50),
    message VARCHAR(2000),
    user_id INTEGER,
    session_id VARCHAR(50)
);

-- HTTP access logs
CREATE TABLE access_logs (
    method VARCHAR(10),
    uri VARCHAR(500),
    status_code INTEGER,
    response_time INTEGER,
    ip_addr IPV4,
    user_agent VARCHAR(500)
);
```

**Note**: Log tables automatically add `_arrival_time` column with nanosecond precision!

## Step 2: Insert Log Data

```sql
-- Error logs
INSERT INTO app_error_logs VALUES (
    'ERROR', 'DatabaseConnection', 'Connection timeout after 30s', 12345, 'sess_abc123'
);
INSERT INTO app_error_logs VALUES (
    'WARN', 'Cache', 'Cache miss ratio exceeded 50%', NULL, NULL
);
INSERT INTO app_error_logs VALUES (
    'ERROR', 'Authentication', 'Invalid credentials for user', 67890, 'sess_def456'
);

-- Access logs
INSERT INTO access_logs VALUES (
    'GET', '/api/users', 200, 45, '192.168.1.100', 'Mozilla/5.0...'
);
INSERT INTO access_logs VALUES (
    'POST', '/api/login', 401, 120, '192.168.1.101', 'curl/7.64.1'
);
INSERT INTO access_logs VALUES (
    'GET', '/api/products', 500, 3000, '192.168.1.102', 'Mozilla/5.0...'
);
```

## Step 3: Full-Text Search

The `SEARCH` keyword enables fast text searching:

```sql
-- Find logs containing "timeout"
SELECT _arrival_time, component, message
FROM app_error_logs
WHERE message SEARCH 'timeout';

-- Find "connection" errors
SELECT * FROM app_error_logs
WHERE level = 'ERROR'
  AND message SEARCH 'connection';

-- Find logs with "cache" OR "memory"
SELECT * FROM app_error_logs
WHERE message SEARCH 'cache'
   OR message SEARCH 'memory';

-- Find logs with both "user" AND "invalid"
SELECT * FROM app_error_logs
WHERE message SEARCH 'user invalid';
```

## Step 4: Time-Based Analysis

```sql
-- Errors in last 10 minutes
SELECT * FROM app_error_logs
WHERE level = 'ERROR'
DURATION 10 MINUTE;

-- HTTP 500 errors in last hour
SELECT uri, COUNT(*) as error_count
FROM access_logs
WHERE status_code >= 500
DURATION 1 HOUR
GROUP BY uri;

-- Slow requests (>1000ms) in last day
SELECT uri, AVG(response_time) as avg_time
FROM access_logs
WHERE response_time > 1000
DURATION 1 DAY
GROUP BY uri
ORDER BY avg_time DESC;
```

## Step 5: Pattern Analysis

```sql
-- Most common errors
SELECT component, message, COUNT(*) as occurrences
FROM app_error_logs
WHERE level = 'ERROR'
DURATION 24 HOUR
GROUP BY component, message
ORDER BY occurrences DESC
LIMIT 10;

-- Traffic by hour
SELECT
    TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00') as hour,
    COUNT(*) as request_count
FROM access_logs
DURATION 1 DAY
GROUP BY TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00')
ORDER BY hour;

-- Error rate by component
SELECT
    component,
    COUNT(*) as total_logs,
    SUM(CASE WHEN level = 'ERROR' THEN 1 ELSE 0 END) as error_count
FROM app_error_logs
DURATION 1 HOUR
GROUP BY component;
```

## Step 6: Data Retention

```sql
-- Keep only 30 days of logs
DELETE FROM app_error_logs EXCEPT 30 DAYS;
DELETE FROM access_logs EXCEPT 30 DAYS;

-- Delete oldest million rows (for size control)
DELETE FROM access_logs OLDEST 1000000 ROWS;

-- Keep only last 100000 error logs
DELETE FROM app_error_logs EXCEPT 100000 ROWS;
```

## Try It Yourself

### Exercise 1: Create Security Logs

Create a table for security events:

<details>
<summary>Solution</summary>

```sql
CREATE TABLE security_logs (
    event_type VARCHAR(50),
    user_id INTEGER,
    ip_addr IPV4,
    success CHAR(1),  -- 'Y' or 'N'
    details VARCHAR(500)
);

INSERT INTO security_logs VALUES (
    'LOGIN_ATTEMPT', 12345, '192.168.1.100', 'Y', 'Successful login'
);
INSERT INTO security_logs VALUES (
    'LOGIN_ATTEMPT', 67890, '10.0.0.50', 'N', 'Invalid password'
);
```
</details>

### Exercise 2: Find Failed Logins

Write a query to find failed login attempts from same IP in last hour:

<details>
<summary>Solution</summary>

```sql
SELECT ip_addr, COUNT(*) as failed_attempts
FROM security_logs
WHERE event_type = 'LOGIN_ATTEMPT'
  AND success = 'N'
DURATION 1 HOUR
GROUP BY ip_addr
HAVING COUNT(*) > 3
ORDER BY failed_attempts DESC;
```
</details>

## Real-World Application

### Automated Log Collection

```python
# Python example using machbase-python
import machbase

conn = machbase.connect('127.0.0.1', 5656, 'SYS', 'MANAGER')
cur = conn.cursor()

# Bulk insert logs
logs = [
    ('ERROR', 'DB', 'Connection failed', 123, 'sess1'),
    ('WARN', 'Cache', 'High miss rate', None, None),
    # ... thousands more
]

cur.executemany(
    "INSERT INTO app_error_logs VALUES (?, ?, ?, ?, ?)",
    logs
)
conn.commit()
```

### Log Monitoring Script

```bash
#!/bin/bash
# monitor_errors.sh - Run every 5 minutes

ERRORS=$(machsql -s localhost -u SYS -p MANAGER -i -f - <<EOF
SELECT COUNT(*) FROM app_error_logs
WHERE level = 'ERROR'
DURATION 5 MINUTE;
EOF
)

if [ "$ERRORS" -gt 10 ]; then
    echo "Alert: $ERRORS errors in last 5 minutes" | mail -s "Error Alert" admin@company.com
fi
```

### Dashboard Query

```sql
-- Real-time monitoring dashboard
SELECT
    level,
    component,
    COUNT(*) as count,
    MAX(_arrival_time) as last_occurrence
FROM app_error_logs
DURATION 15 MINUTE
GROUP BY level, component
ORDER BY count DESC;
```

## Performance Tips

1. **Use SEARCH for text**: Faster than LIKE '%pattern%'
2. **Always limit time range**: Use DURATION or WHERE on _arrival_time
3. **Aggregate when possible**: Use GROUP BY instead of returning raw logs
4. **Regular cleanup**: Run retention deletes daily