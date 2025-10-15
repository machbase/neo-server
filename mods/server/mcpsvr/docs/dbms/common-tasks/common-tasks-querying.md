# Querying Data

Master common query patterns and optimization techniques for Machbase. Learn how to write efficient queries for time-series data analysis.

## Query Basics

### Simple SELECT

```sql
-- Get all recent data
SELECT * FROM sensors DURATION 1 HOUR;

-- With specific columns
SELECT sensor_id, value, _arrival_time
FROM sensors DURATION 1 HOUR;

-- With conditions
SELECT * FROM sensors
WHERE sensor_id = 'sensor01'
DURATION 1 HOUR;
```

### Time-Based Queries

```sql
-- Last 10 minutes
SELECT * FROM logs DURATION 10 MINUTE;

-- Last hour
SELECT * FROM logs DURATION 1 HOUR;

-- Last day
SELECT * FROM logs DURATION 1 DAY;

-- 30 minutes starting 2 hours ago
SELECT * FROM logs DURATION 30 MINUTE BEFORE 2 HOUR;

-- Specific time range
SELECT * FROM logs
WHERE _arrival_time BETWEEN '2025-10-10 00:00:00' AND '2025-10-10 23:59:59';
```

## Common Patterns

### Pattern 1: Recent Data Monitoring

```sql
-- Last 5 minutes of errors
SELECT * FROM app_logs
WHERE level = 'ERROR'
DURATION 5 MINUTE;

-- Latest sensor readings
SELECT sensor_id, value, _arrival_time
FROM sensors
DURATION 10 MINUTE
ORDER BY _arrival_time DESC;
```

### Pattern 2: Aggregations

```sql
-- Count by hour
SELECT
    TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00') as hour,
    COUNT(*) as count
FROM logs
DURATION 24 HOUR
GROUP BY TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00');

-- Average by sensor
SELECT
    sensor_id,
    AVG(value) as avg_value,
    MIN(value) as min_value,
    MAX(value) as max_value
FROM sensors
DURATION 1 DAY
GROUP BY sensor_id;
```

### Pattern 3: Text Search

```sql
-- Search for keyword
SELECT * FROM logs
WHERE message SEARCH 'timeout'
DURATION 1 HOUR;

-- Multiple keywords (OR)
SELECT * FROM logs
WHERE message SEARCH 'error'
   OR message SEARCH 'failed'
DURATION 1 HOUR;

-- Case-insensitive search
SELECT * FROM logs
WHERE LOWER(message) LIKE '%error%'
DURATION 1 HOUR;
```

### Pattern 4: JOIN Operations

```sql
-- Enrich sensor data with device info
SELECT
    s.sensor_id,
    s.value,
    s._arrival_time,
    d.device_name,
    d.location
FROM sensors s
JOIN devices d ON s.sensor_id = d.device_id
DURATION 1 HOUR;

-- Multiple joins
SELECT
    s.*,
    d.device_name,
    f.facility_name,
    f.city
FROM sensors s
JOIN devices d ON s.sensor_id = d.device_id
JOIN facilities f ON d.facility = f.facility_code
DURATION 1 HOUR;
```

### Pattern 5: Rollup Queries (Tag Tables)

```sql
-- Query hourly rollup
SELECT
    sensor_id,
    time,
    min_temperature,
    max_temperature,
    avg_temperature,
    count
FROM sensors
WHERE rollup = hour
DURATION 7 DAY;

-- Minute-level rollup
SELECT * FROM sensors
WHERE rollup = min
DURATION 24 HOUR;

-- Second-level rollup
SELECT * FROM sensors
WHERE rollup = sec
DURATION 1 HOUR;
```

## Query Optimization

### 1. Always Use Time Filters

**Bad** (scans all data):
```sql
SELECT * FROM sensors WHERE sensor_id = 'sensor01';
```

**Good** (scans relevant partition):
```sql
SELECT * FROM sensors
WHERE sensor_id = 'sensor01'
DURATION 1 HOUR;
```

### 2. Use LIMIT for Large Results

```sql
-- Limit results
SELECT * FROM logs DURATION 1 DAY LIMIT 1000;

-- Top N results
SELECT * FROM sensors
ORDER BY value DESC
LIMIT 10;
```

### 3. Query Rollup, Not Raw Data

**Slow** (processes millions of rows):
```sql
SELECT sensor_id, AVG(value)
FROM sensors
DURATION 30 DAY
GROUP BY sensor_id;
```

**Fast** (queries pre-aggregated data):
```sql
SELECT sensor_id, AVG(avg_value)
FROM sensors
WHERE rollup = hour
DURATION 30 DAY
GROUP BY sensor_id;
```

### 4. Use Indexes

```sql
-- Create index on frequently queried column
CREATE INDEX idx_level ON logs(level);

-- Now this query is fast
SELECT * FROM logs
WHERE level = 'ERROR'
DURATION 1 HOUR;
```

### 5. Avoid SELECT *

```sql
-- Bad (reads all columns)
SELECT * FROM sensors;

-- Good (reads only needed columns)
SELECT sensor_id, value FROM sensors DURATION 1 HOUR;
```

## Advanced Queries

### Subqueries

```sql
-- Find sensors above average
SELECT sensor_id, value
FROM sensors
WHERE value > (
    SELECT AVG(value) FROM sensors DURATION 1 HOUR
)
DURATION 1 HOUR;
```

### Common Table Expressions (CTE)

```sql
-- Calculate hourly averages
WITH hourly_avg AS (
    SELECT
        sensor_id,
        TO_CHAR(_arrival_time, 'HH24') as hour,
        AVG(value) as avg_value
    FROM sensors
    DURATION 24 HOUR
    GROUP BY sensor_id, TO_CHAR(_arrival_time, 'HH24')
)
SELECT * FROM hourly_avg WHERE avg_value > 25.0;
```

### Window Functions

```sql
-- Running average
SELECT
    sensor_id,
    value,
    AVG(value) OVER (
        PARTITION BY sensor_id
        ORDER BY _arrival_time
        ROWS BETWEEN 9 PRECEDING AND CURRENT ROW
    ) as moving_avg
FROM sensors
DURATION 1 HOUR;
```

## Time Functions

### Date/Time Formatting

```sql
-- Format timestamp
SELECT
    TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:MI:SS') as formatted_time,
    sensor_id,
    value
FROM sensors DURATION 1 HOUR;

-- Extract parts
SELECT
    TO_CHAR(_arrival_time, 'YYYY') as year,
    TO_CHAR(_arrival_time, 'MM') as month,
    TO_CHAR(_arrival_time, 'DD') as day,
    TO_CHAR(_arrival_time, 'HH24') as hour
FROM logs DURATION 1 DAY;
```

### Time Calculations

```sql
-- Current time
SELECT SYSDATE;
SELECT NOW;

-- Time arithmetic
SELECT SYSDATE - INTERVAL '1' HOUR;
SELECT NOW + INTERVAL '30' MINUTE;

-- Date conversion
SELECT TO_DATE('2025-10-10', 'YYYY-MM-DD');
SELECT TO_TIMESTAMP('2025-10-10 14:30:00', 'YYYY-MM-DD HH24:MI:SS');
```

## Analytical Queries

### Statistical Analysis

```sql
-- Comprehensive statistics
SELECT
    sensor_id,
    COUNT(*) as count,
    AVG(value) as mean,
    STDDEV(value) as stddev,
    MIN(value) as min,
    MAX(value) as max,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY value) as median
FROM sensors
DURATION 1 DAY
GROUP BY sensor_id;
```

### Trend Analysis

```sql
-- Hourly trend
SELECT
    TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00') as hour,
    AVG(value) as avg_value,
    LAG(AVG(value)) OVER (ORDER BY TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00')) as prev_hour,
    AVG(value) - LAG(AVG(value)) OVER (ORDER BY TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00')) as change
FROM sensors
DURATION 24 HOUR
GROUP BY TO_CHAR(_arrival_time, 'YYYY-MM-DD HH24:00:00')
ORDER BY hour;
```

### Anomaly Detection

```sql
-- Find values outside 2 standard deviations
WITH stats AS (
    SELECT
        AVG(value) as mean,
        STDDEV(value) as stddev
    FROM sensors DURATION 1 DAY
)
SELECT s.*, st.mean, st.stddev
FROM sensors s, stats st
WHERE s.value < st.mean - 2 * st.stddev
   OR s.value > st.mean + 2 * st.stddev
DURATION 1 DAY;
```

## Query Performance Monitoring

### Check Active Queries

```sql
-- View running queries
SHOW STATEMENTS;
```

### Query Execution Plan

```sql
-- Explain query plan
EXPLAIN SELECT * FROM sensors DURATION 1 HOUR;
```

### Performance Tips

1. **Use time filters** - Always include DURATION or time WHERE clause
2. **Query rollup** - Use pre-aggregated data for Tag tables
3. **Create indexes** - Index frequently queried columns on Log/Lookup tables
4. **Limit results** - Use LIMIT to restrict result set size
5. **Select specific columns** - Avoid SELECT *
6. **Batch operations** - Process large datasets in chunks

## Common Query Patterns

### Dashboard Queries

```sql
-- Real-time status board
SELECT
    device_id,
    status,
    last_value,
    last_updated
FROM device_status
ORDER BY last_updated DESC
LIMIT 20;

-- Error summary
SELECT
    level,
    COUNT(*) as count,
    MAX(_arrival_time) as last_occurrence
FROM logs
DURATION 1 HOUR
GROUP BY level;
```

### Reporting Queries

```sql
-- Daily summary report
SELECT
    TO_CHAR(_arrival_time, 'YYYY-MM-DD') as date,
    COUNT(*) as total_records,
    COUNT(DISTINCT sensor_id) as unique_sensors,
    AVG(value) as avg_value
FROM sensors
DURATION 30 DAY
GROUP BY TO_CHAR(_arrival_time, 'YYYY-MM-DD')
ORDER BY date;
```

### Alert Queries

```sql
-- Critical errors in last 5 minutes
SELECT * FROM logs
WHERE level = 'ERROR'
  AND message SEARCH 'critical'
DURATION 5 MINUTE;

-- Sensors exceeding threshold
SELECT sensor_id, value, _arrival_time
FROM sensors
WHERE value > 30.0
DURATION 10 MINUTE;
```

## Best Practices

1. **Always filter by time** - Use DURATION or time-based WHERE clause
2. **Test queries on small time ranges first** - Verify before running on large datasets
3. **Use LIMIT** - Prevent accidentally returning millions of rows
4. **Query rollup for analytics** - Much faster than aggregating raw data
5. **Create indexes** - For frequently queried columns on Log/Lookup tables
6. **Monitor performance** - Use SHOW STATEMENTS to track slow queries
7. **Use appropriate table types** - Tag for sensors, Log for events, etc.

## Troubleshooting

**Query too slow**:
- Add time filter (DURATION)
- Use LIMIT clause
- Query rollup instead of raw data
- Create index on filter columns

**Out of memory**:
- Reduce time range
- Use LIMIT
- Select fewer columns
- Increase server memory (MAX_QPX_MEM)

**Connection timeout**:
- Increase query timeout
- Break into smaller queries
- Optimize query (add indexes, use rollup)

## Next Steps

- **User Management**: [User Management](../user-management/) - Control query permissions
- **Backup**: [Backup & Recovery](../backup-recovery/) - Protect your data
- **Core Concepts**: [Indexing](../../core-concepts/indexing/) - Deep dive into performance

---

Master these query patterns and unlock the full power of Machbase analytics!
