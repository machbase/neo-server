# First Steps with machsql

Learn to use machsql, the interactive SQL command-line interface for Machbase. This guide covers essential commands and workflows you'll use every day.

## What is machsql?

`machsql` is Machbase's interactive SQL client - think of it as your command center for:
- Running SQL queries
- Managing tables and users
- Viewing system information
- Importing/exporting data

## Connecting to Machbase

### Basic Connection

```bash
machsql
```

You'll be prompted for:
- Server address (default: 127.0.0.1)
- User ID (default: SYS)
- Password (default: MANAGER)

### Connection with Parameters

Skip the prompts by providing connection details:

```bash
machsql -s localhost -u SYS -p MANAGER
```

Common options:

```bash
machsql -s 192.168.1.100     # Connect to remote server
machsql -u myuser -p mypass  # Specify credentials
machsql -P 7878              # Use different port
machsql -f script.sql        # Execute SQL script file
```

## Essential Commands

### SHOW Commands

Display system information:

```sql
-- List all tables
SHOW TABLES;

-- View table structure
SHOW TABLE sensor_data;

-- List indexes
SHOW INDEXES;

-- List users
SHOW USERS;

-- View license info
SHOW LICENSE;

-- Check disk usage
SHOW STORAGE;

-- View tablespaces
SHOW TABLESPACES;

-- See active queries
SHOW STATEMENTS;
```

### Creating Tables

```sql
-- Simple log table
CREATE TABLE app_logs (
    level VARCHAR(10),
    message VARCHAR(1000)
);

-- Sensor data table
CREATE TABLE temperatures (
    sensor_id VARCHAR(20),
    value DOUBLE
);

-- Table with multiple columns
CREATE TABLE device_data (
    device_id INTEGER,
    location VARCHAR(50),
    temperature DOUBLE,
    humidity DOUBLE,
    pressure DOUBLE
);
```

### Inserting Data

```sql
-- Single insert
INSERT INTO temperatures VALUES ('sensor01', 25.3);

-- Multiple inserts
INSERT INTO app_logs VALUES ('INFO', 'Application started');
INSERT INTO app_logs VALUES ('WARN', 'High memory usage detected');
INSERT INTO app_logs VALUES ('ERROR', 'Connection timeout');
```

### Querying Data

```sql
-- Get all records
SELECT * FROM temperatures;

-- With timestamp
SELECT _arrival_time, * FROM temperatures;

-- With condition
SELECT * FROM app_logs WHERE level = 'ERROR';

-- Recent data (last 10 minutes)
SELECT * FROM temperatures DURATION 10 MINUTE;

-- Data from specific time range
SELECT * FROM temperatures
DURATION 30 MINUTE BEFORE 1 HOUR;

-- Aggregations
SELECT sensor_id, AVG(value), MAX(value), MIN(value)
FROM temperatures
GROUP BY sensor_id;

-- Count records
SELECT COUNT(*) FROM app_logs;
```

### Deleting Data

```sql
-- Delete oldest 100 rows
DELETE FROM app_logs OLDEST 100 ROWS;

-- Keep only last 1000 rows
DELETE FROM app_logs EXCEPT 1000 ROWS;

-- Delete data older than 7 days
DELETE FROM app_logs EXCEPT 7 DAYS;

-- Delete before specific date
DELETE FROM app_logs
BEFORE TO_DATE('2025-01-01', 'YYYY-MM-DD');
```

### Managing Tables

```sql
-- Drop table
DROP TABLE temperatures;

-- Truncate table (delete all data)
TRUNCATE TABLE app_logs;

-- Create index
CREATE INDEX idx_sensor ON temperatures(sensor_id);
```

## Understanding _arrival_time

Every record in Machbase automatically gets a timestamp:

```sql
SELECT _arrival_time, * FROM temperatures;
```

Output:
```
_arrival_time                   SENSOR_ID    VALUE
--------------------------------------------------------
2025-10-10 14:23:45 123:456:789 sensor01     25.3
2025-10-10 14:23:40 987:654:321 sensor01     24.8
```

The timestamp includes nanosecond precision!

## Working with Time Ranges

### DURATION Keyword

The `DURATION` keyword makes time-based queries simple:

```sql
-- Last 5 minutes
SELECT * FROM temperatures DURATION 5 MINUTE;

-- Last hour
SELECT * FROM temperatures DURATION 1 HOUR;

-- Last day
SELECT * FROM temperatures DURATION 1 DAY;

-- 30 minutes starting from 2 hours ago
SELECT * FROM temperatures
DURATION 30 MINUTE BEFORE 2 HOUR;
```

## Text Search

Search for text within columns:

```sql
-- Find logs containing "error"
SELECT * FROM app_logs
WHERE message SEARCH 'error';

-- Find logs with "timeout" OR "connection"
SELECT * FROM app_logs
WHERE message SEARCH 'timeout'
   OR message SEARCH 'connection';

-- Find logs with both "high" AND "memory"
SELECT * FROM app_logs
WHERE message SEARCH 'high memory';
```

## Running SQL Scripts

Execute a file containing SQL commands:

```bash
machsql -f setup.sql
```

Or from within machsql:

```sql
@/path/to/script.sql
```

## Exporting Query Results

Save results to a file:

```bash
# Export to CSV
machsql -s localhost -u SYS -p MANAGER \
  -f query.sql -o output.csv -r csv

# Export to JSON
machsql -s localhost -u SYS -p MANAGER \
  -f query.sql -o output.json -r json
```

## Tips and Tricks

### 1. Command History

Use arrow keys to navigate previous commands:
- **↑** - Previous command
- **↓** - Next command

### 2. Auto-completion

Press `Tab` to auto-complete:
- Table names
- Column names
- SQL keywords

### 3. Multi-line Queries

machsql supports multi-line SQL:

```sql
SELECT
    sensor_id,
    AVG(value) as avg_temp,
    MAX(value) as max_temp
FROM
    temperatures
WHERE
    _arrival_time > NOW() - INTERVAL '1' HOUR
GROUP BY
    sensor_id;
```

### 4. Quiet Output

For scripts, suppress the banner:

```bash
machsql -i  # or --silent
```

### 5. Set Timezone

```bash
machsql -z +0900  # Korea timezone
machsql -z -0500  # US Eastern
```

## Common Workflows

### Daily Monitoring

```sql
-- Check recent errors
SELECT * FROM app_logs
WHERE level = 'ERROR'
DURATION 1 DAY;

-- Monitor sensor health
SELECT sensor_id, COUNT(*), AVG(value)
FROM temperatures
DURATION 1 HOUR
GROUP BY sensor_id;

-- Check database size
SHOW STORAGE;
```

### Data Cleanup

```sql
-- Keep only 30 days of data
DELETE FROM app_logs EXCEPT 30 DAYS;

-- Remove old sensor data
DELETE FROM temperatures EXCEPT 7 DAYS;
```

### Performance Check

```sql
-- View active queries
SHOW STATEMENTS;

-- Check index status
SHOW INDEXES;

-- View index building progress
SHOW INDEXGAP;
```

## User Management

```sql
-- Create user
CREATE USER datauser IDENTIFIED BY 'password123';

-- Grant permissions
GRANT SELECT ON temperatures TO datauser;
GRANT INSERT ON temperatures TO datauser;

-- Change password
ALTER USER datauser IDENTIFIED BY 'newpassword';

-- Drop user
DROP USER datauser;
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+C` | Cancel current query |
| `Ctrl+D` | Exit machsql |
| `Ctrl+L` | Clear screen |
| `↑` / `↓` | Navigate command history |
| `Tab` | Auto-complete |

## Getting Help

Within machsql:

```sql
-- Display help
help

-- Get command syntax
help CREATE TABLE
help SELECT
help DELETE
```

## Troubleshooting

### Connection Failed

```bash
# Check server is running
machadmin -e

# Check if port is open
netstat -an | grep 5656
```

### Query Timeout

```sql
-- For long-running queries, increase timeout
SET QUERY_TIMEOUT = 300;  -- 5 minutes
```

### Out of Memory

```sql
-- Limit result set
SELECT * FROM large_table LIMIT 1000;

-- Use aggregation instead of raw data
SELECT COUNT(*), AVG(value) FROM large_table;
```

## Next Steps

Now that you know machsql basics:

1. [**Basic Concepts**](../concepts/) - Understand table types and architecture
2. [**Tutorials**](../../tutorials/) - Follow hands-on examples
3. [**SQL Reference**](../../sql-reference/) - Complete SQL syntax guide

## Quick Reference Card

```sql
-- TABLE OPERATIONS
SHOW TABLES;
SHOW TABLE tablename;
CREATE TABLE t (col TYPE);
DROP TABLE t;

-- DATA OPERATIONS
INSERT INTO t VALUES (...);
SELECT * FROM t;
SELECT * FROM t DURATION 10 MINUTE;
DELETE FROM t EXCEPT 7 DAYS;

-- SYSTEM INFO
SHOW LICENSE;
SHOW STORAGE;
SHOW USERS;

-- TIME QUERIES
DURATION 5 MINUTE
DURATION 1 HOUR
DURATION 1 DAY
DURATION 30 MINUTE BEFORE 2 HOUR

-- TEXT SEARCH
WHERE column SEARCH 'text'
```

---

**Practice makes perfect!** Try these commands with your own data to get comfortable with machsql.
