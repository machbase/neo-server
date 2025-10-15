# Tutorial 3: Real-time Analytics

Learn how to build a real-time monitoring dashboard using Machbase Volatile tables. This tutorial shows you how to handle in-memory data that requires frequent updates.

## Scenario

You're building a live factory monitoring system that displays:
- Current status of 50 production lines
- Real-time equipment health metrics
- Active alarms and warnings
- Dashboard updated every second

**Challenge**: You need fast INSERT, UPDATE, and DELETE operations for constantly changing data.

**Solution**: Use Volatile tables - in-memory tables optimized for high-speed updates.

## What You'll Learn

- Creating Volatile tables for in-memory data
- Using UPDATE and DELETE operations
- Fast key-based lookups
- Combining Volatile tables with Log/Tag tables
- Building real-time status boards

## Prerequisites

- Machbase installed and running
- machsql client connected
- Completed Tutorial 1 and 2
- 20 minutes of time

## Step 1: Create Volatile Table

Volatile tables live entirely in memory for maximum speed:

```sql
-- Production line status
CREATE VOLATILE TABLE production_status (
    line_id INTEGER PRIMARY KEY,
    line_name VARCHAR(50),
    status VARCHAR(20),
    current_product VARCHAR(100),
    items_produced INTEGER,
    target_rate INTEGER,
    actual_rate INTEGER,
    last_updated DATETIME
);
```

**Key features**:
- `PRIMARY KEY` enables fast UPDATE/DELETE by line_id
- All data stored in memory (very fast)
- Data is NOT persistent (lost on restart)

## Step 2: Insert Initial Status

```sql
-- Initialize production lines
INSERT INTO production_status VALUES (
    1, 'Assembly Line A', 'RUNNING', 'Widget-X100', 1250, 100, 98, NOW
);
INSERT INTO production_status VALUES (
    2, 'Assembly Line B', 'RUNNING', 'Widget-X200', 890, 80, 82, NOW
);
INSERT INTO production_status VALUES (
    3, 'Packaging Line 1', 'IDLE', NULL, 0, 120, 0, NOW
);
INSERT INTO production_status VALUES (
    4, 'Assembly Line C', 'WARNING', 'Widget-X100', 450, 100, 75, NOW
);
INSERT INTO production_status VALUES (
    5, 'Quality Check', 'RUNNING', 'Widget-X200', 320, 50, 48, NOW
);
```

## Step 3: Update Status in Real-Time

Unlike Log/Tag tables, Volatile tables support UPDATE:

```sql
-- Production line completed a batch
UPDATE production_status
SET items_produced = items_produced + 50,
    actual_rate = 101,
    last_updated = NOW
WHERE line_id = 1;

-- Line went into maintenance
UPDATE production_status
SET status = 'MAINTENANCE',
    current_product = NULL,
    actual_rate = 0,
    last_updated = NOW
WHERE line_id = 3;

-- Line resolved warning
UPDATE production_status
SET status = 'RUNNING',
    actual_rate = 98,
    last_updated = NOW
WHERE line_id = 4;
```

## Step 4: Query Current Status

Get real-time dashboard data:

```sql
-- All production lines
SELECT * FROM production_status
ORDER BY line_id;

-- Only running lines
SELECT line_id, line_name, actual_rate, target_rate
FROM production_status
WHERE status = 'RUNNING';

-- Lines with issues
SELECT line_id, line_name, status
FROM production_status
WHERE status IN ('WARNING', 'ERROR', 'MAINTENANCE');

-- Performance metrics
SELECT
    line_name,
    actual_rate,
    target_rate,
    ROUND((actual_rate * 100.0 / target_rate), 2) as efficiency_pct
FROM production_status
WHERE status = 'RUNNING';
```

## Step 5: Create Alert Table

Combine with Log table for historical tracking:

```sql
-- Log all status changes
CREATE TABLE production_events (
    line_id INTEGER,
    event_type VARCHAR(50),
    old_status VARCHAR(20),
    new_status VARCHAR(20),
    message VARCHAR(500)
);

-- Insert event log
INSERT INTO production_events VALUES (
    4, 'STATUS_CHANGE', 'WARNING', 'RUNNING', 'Line recovered from warning state'
);
INSERT INTO production_events VALUES (
    3, 'STATUS_CHANGE', 'RUNNING', 'MAINTENANCE', 'Scheduled maintenance started'
);
```

## Step 6: Track Active Alarms

Create another Volatile table for active alarms:

```sql
CREATE VOLATILE TABLE active_alarms (
    alarm_id INTEGER PRIMARY KEY,
    line_id INTEGER,
    alarm_type VARCHAR(50),
    severity VARCHAR(20),
    message VARCHAR(500),
    triggered_at DATETIME
);

-- Add alarms
INSERT INTO active_alarms VALUES (
    1001, 4, 'LOW_THROUGHPUT', 'WARNING', 'Rate below 80% of target', NOW
);
INSERT INTO active_alarms VALUES (
    1002, 2, 'TEMPERATURE_HIGH', 'WARNING', 'Temperature 5C above normal', NOW
);

-- Clear alarm when resolved
DELETE FROM active_alarms WHERE alarm_id = 1001;

-- Query active alarms
SELECT a.*, p.line_name
FROM active_alarms a
JOIN production_status p ON a.line_id = p.line_id
ORDER BY severity, triggered_at;
```

## Step 7: Implement Live Metrics

Create session tracking for operator dashboard:

```sql
CREATE VOLATILE TABLE operator_sessions (
    session_id VARCHAR(50) PRIMARY KEY,
    operator_name VARCHAR(100),
    login_time DATETIME,
    assigned_lines VARCHAR(200),
    last_activity DATETIME
);

-- Operator logs in
INSERT INTO operator_sessions VALUES (
    'sess_12345', 'John Smith', NOW, '1,2,3', NOW
);

-- Update activity
UPDATE operator_sessions
SET last_activity = NOW
WHERE session_id = 'sess_12345';

-- Remove inactive sessions (timeout after 30 minutes)
DELETE FROM operator_sessions
WHERE last_activity < NOW - INTERVAL '30' MINUTE;
```

## Step 8: Create Dashboard View

Combine multiple tables for comprehensive view:

```sql
-- Current production summary
SELECT
    COUNT(*) as total_lines,
    SUM(CASE WHEN status = 'RUNNING' THEN 1 ELSE 0 END) as running,
    SUM(CASE WHEN status = 'IDLE' THEN 1 ELSE 0 END) as idle,
    SUM(CASE WHEN status = 'WARNING' THEN 1 ELSE 0 END) as warnings,
    SUM(CASE WHEN status = 'ERROR' THEN 1 ELSE 0 END) as errors,
    SUM(items_produced) as total_produced
FROM production_status;

-- Lines needing attention
SELECT
    p.line_id,
    p.line_name,
    p.status,
    COUNT(a.alarm_id) as active_alarms
FROM production_status p
LEFT JOIN active_alarms a ON p.line_id = a.line_id
WHERE p.status != 'RUNNING'
   OR a.alarm_id IS NOT NULL
GROUP BY p.line_id, p.line_name, p.status;
```

## Try It Yourself

### Exercise 1: Simulate Production Cycle

Write a series of updates to simulate a production cycle:

<details>
<summary>Solution</summary>

```sql
-- Start production
UPDATE production_status
SET status = 'RUNNING',
    current_product = 'Widget-X300',
    items_produced = 0,
    actual_rate = 95
WHERE line_id = 3;

-- Production progressing
UPDATE production_status
SET items_produced = items_produced + 100,
    last_updated = NOW
WHERE line_id = 3;

-- Complete batch
UPDATE production_status
SET status = 'IDLE',
    current_product = NULL,
    actual_rate = 0,
    last_updated = NOW
WHERE line_id = 3;
```
</details>

### Exercise 2: Build Alert System

Create a query to find lines that need immediate attention:

<details>
<summary>Solution</summary>

```sql
-- Critical issues: ERROR status or multiple alarms
SELECT
    p.line_id,
    p.line_name,
    p.status,
    COUNT(a.alarm_id) as alarm_count,
    MAX(a.severity) as worst_severity
FROM production_status p
LEFT JOIN active_alarms a ON p.line_id = a.line_id
WHERE p.status = 'ERROR'
   OR EXISTS (
       SELECT 1 FROM active_alarms
       WHERE line_id = p.line_id
       AND severity = 'CRITICAL'
   )
GROUP BY p.line_id, p.line_name, p.status;
```
</details>

### Exercise 3: Performance Tracking

Track which lines are underperforming:

<details>
<summary>Solution</summary>

```sql
SELECT
    line_id,
    line_name,
    status,
    target_rate,
    actual_rate,
    target_rate - actual_rate as shortfall,
    ROUND((actual_rate * 100.0 / target_rate), 1) as efficiency
FROM production_status
WHERE status = 'RUNNING'
  AND actual_rate < target_rate * 0.9  -- Below 90% efficiency
ORDER BY efficiency ASC;
```
</details>

## Real-World Application

### Pattern: Hybrid Storage

Combine Volatile (current state) with Log (history):

```sql
-- Volatile: Current equipment status
CREATE VOLATILE TABLE equipment_status (
    equipment_id INTEGER PRIMARY KEY,
    temperature DOUBLE,
    vibration DOUBLE,
    status VARCHAR(20),
    last_reading DATETIME
);

-- Log: Historical readings
CREATE TABLE equipment_history (
    equipment_id INTEGER,
    temperature DOUBLE,
    vibration DOUBLE,
    status VARCHAR(20)
);

-- Update current status (fast)
UPDATE equipment_status
SET temperature = 75.5,
    vibration = 0.2,
    last_reading = NOW
WHERE equipment_id = 101;

-- Archive to history every minute
INSERT INTO equipment_history
SELECT equipment_id, temperature, vibration, status
FROM equipment_status;
```

### Pattern: Key-Value Cache

Use Volatile table as fast lookup cache:

```sql
CREATE VOLATILE TABLE config_cache (
    config_key VARCHAR(100) PRIMARY KEY,
    config_value VARCHAR(500),
    updated_at DATETIME
);

-- Store configuration
INSERT INTO config_cache VALUES ('max_temp_threshold', '80.0', NOW);
INSERT INTO config_cache VALUES ('alert_email', 'ops@company.com', NOW);

-- Fast lookup
SELECT config_value
FROM config_cache
WHERE config_key = 'max_temp_threshold';

-- Update configuration
UPDATE config_cache
SET config_value = '85.0',
    updated_at = NOW
WHERE config_key = 'max_temp_threshold';
```

### Pattern: Session Management

Track active user sessions:

```sql
CREATE VOLATILE TABLE web_sessions (
    session_token VARCHAR(100) PRIMARY KEY,
    user_id INTEGER,
    ip_address IPV4,
    created_at DATETIME,
    expires_at DATETIME,
    last_activity DATETIME
);

-- Create session
INSERT INTO web_sessions VALUES (
    'tok_abc123xyz', 12345, '192.168.1.100', NOW, NOW + INTERVAL '2' HOUR, NOW
);

-- Validate session
SELECT user_id
FROM web_sessions
WHERE session_token = 'tok_abc123xyz'
  AND expires_at > NOW;

-- Update activity
UPDATE web_sessions
SET last_activity = NOW
WHERE session_token = 'tok_abc123xyz';

-- Cleanup expired sessions
DELETE FROM web_sessions
WHERE expires_at < NOW;
```

## Important Considerations

### Data Persistence

**WARNING**: Volatile table data is lost when Machbase shuts down!

```sql
-- Before shutdown, archive important data
INSERT INTO production_events
SELECT line_id, 'SHUTDOWN', status, 'OFFLINE', 'Server shutdown'
FROM production_status
WHERE status = 'RUNNING';
```

### Memory Management

Monitor memory usage:

```sql
-- Check volatile table sizes
SHOW TABLE production_status;
SHOW TABLE active_alarms;

-- Implement cleanup policies
DELETE FROM operator_sessions
WHERE last_activity < NOW - INTERVAL '1' HOUR;
```

### When NOT to Use Volatile Tables

Don't use Volatile tables for:
- Data that must persist
- High-volume continuous data (use Tag/Log tables)
- Large datasets (limited by available memory)

## Performance Tips

1. **Use PRIMARY KEY**: Enables O(1) lookups
2. **Keep data small**: Only store current state, not history
3. **Regular cleanup**: Delete old/expired records
4. **Archive to Log tables**: Move old data to persistent storage
