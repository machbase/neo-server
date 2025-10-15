# Basic Concepts

Understanding a few core concepts will help you use Machbase effectively. This guide covers the essentials you need to know.

## What Makes Machbase Different?

Machbase is optimized for **time-series data** - data that continuously flows in with timestamps:

- IoT sensor readings
- Application logs
- Manufacturing equipment data
- Network traffic
- Financial ticks

Unlike traditional databases, Machbase is built specifically for:
- **Write-heavy** workloads (millions of inserts per second)
- **Time-based** queries (recent data, time ranges)
- **Append-only** data (rarely updated or deleted)

## The Four Table Types

Machbase provides four different table types. **Choosing the right one is important!**

### Quick Decision Guide

**Which table should I use?**

```
Do you have sensor data (ID, timestamp, value)?
    YES → Use TAG TABLE

Is it log data or mixed data types?
    YES → Use LOG TABLE

Do you need to UPDATE or DELETE specific records by key?
    YES → Use VOLATILE TABLE (in-memory)

Is it reference/master data that changes rarely?
    YES → Use LOOKUP TABLE
```

### 1. Tag Table - For Sensor Data

**Use when**: Storing sensor/device time-series data

**Best for**:
- IoT sensor readings (temperature, pressure, vibration)
- Smart meter data
- Environmental monitoring
- Equipment telemetry

**Structure**:
```
(sensor_name, timestamp, value, [optional columns])
```

**Key features**:
- Millions of records per second
- Automatic statistics generation (rollup)
- Ultra-fast queries by sensor ID + time range
- Deduplication support

**Example**:
```sql
CREATE TAGDATA TABLE sensors (
    sensor_name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    temperature DOUBLE SUMMARIZED,
    humidity DOUBLE SUMMARIZED
);
```

### 2. Log Table - For General Time-Series Data

**Use when**: Storing log files, events, or any time-stamped data

**Best for**:
- Application logs
- Event streams
- Access logs
- Transaction logs
- PLC data with multiple columns

**Structure**: Any schema you want!

**Key features**:
- Millions of records per second
- Automatic `_arrival_time` column (nanosecond precision)
- Newest data returned first
- Full-text search with SEARCH keyword
- Flexible schema

**Example**:
```sql
CREATE TABLE app_logs (
    level VARCHAR(10),
    user_id INTEGER,
    message VARCHAR(1000),
    ip_addr IPV4
);
```

### 3. Volatile Table - For In-Memory Data

**Use when**: You need fast INSERT/UPDATE/DELETE in memory

**Best for**:
- Real-time dashboards
- Session data
- Temporary calculations
- Key-value caching
- Live monitoring displays

**Structure**: Any schema, supports PRIMARY KEY

**Key features**:
- Tens of thousands of ops per second
- Supports UPDATE and DELETE by primary key
- All data in memory (very fast)
- **Data lost on shutdown!**

**Example**:
```sql
CREATE VOLATILE TABLE live_status (
    device_id INTEGER PRIMARY KEY,
    status VARCHAR(20),
    last_updated DATETIME
);
```

### 4. Lookup Table - For Reference Data

**Use when**: Storing reference/master data that changes rarely

**Best for**:
- Device registry
- Configuration tables
- Category/dimension tables
- Master data

**Structure**: Any schema

**Key features**:
- Fast SELECT performance
- Persistent storage
- Slower INSERT/UPDATE (disk-based)
- Standard database operations

**Example**:
```sql
CREATE LOOKUP TABLE devices (
    device_id INTEGER,
    name VARCHAR(50),
    location VARCHAR(100),
    type VARCHAR(20)
);
```

## Comparison Table

| Feature | Tag Table | Log Table | Volatile Table | Lookup Table |
|---------|-----------|-----------|----------------|--------------|
| **Purpose** | Sensor data | Log/event data | In-memory cache | Master data |
| **Insert Speed** | Millions/sec | Millions/sec | 10,000s/sec | 100s/sec |
| **UPDATE Support** | No* | No | Yes | Yes |
| **DELETE Support** | Time-based | Time-based | By key | By key |
| **Storage** | Disk | Disk | Memory | Disk |
| **Schema** | Fixed pattern | Flexible | Flexible | Flexible |
| **Best Query** | By ID + time | Time-based | By key | Any |
| **Data Persistence** | Yes | Yes | **No** | Yes |

*Tag table metadata columns can be updated

## Automatic Timestamp: _arrival_time

Every Log table record automatically gets a timestamp:

```sql
-- You insert this
INSERT INTO app_logs VALUES ('ERROR', 'Connection failed');

-- Machbase stores this
-- _arrival_time: 2025-10-10 14:23:45 123:456:789
-- level: ERROR
-- message: Connection failed
```

Access it with:
```sql
SELECT _arrival_time, * FROM app_logs;
```

The timestamp has **nanosecond precision** - perfect for high-frequency data!

## Data Order: Newest First

Unlike traditional databases, Machbase returns **newest data first**:

```sql
SELECT * FROM app_logs;
-- Returns most recent logs at the top
-- No need for ORDER BY _arrival_time DESC
```

This is optimized for time-series analysis where recent data is most valuable.

## Time-Based Queries: DURATION

The `DURATION` keyword makes time queries simple:

```sql
-- Last 10 minutes
SELECT * FROM app_logs DURATION 10 MINUTE;

-- Instead of:
-- SELECT * FROM app_logs
-- WHERE _arrival_time >= NOW() - INTERVAL '10' MINUTE;
```

More examples:
```sql
-- Last hour
DURATION 1 HOUR

-- Last day
DURATION 1 DAY

-- 30 minutes starting from 2 hours ago
DURATION 30 MINUTE BEFORE 2 HOUR
```

## Write-Once Architecture

Machbase is designed for **append-only** data:

- No UPDATE on Tag/Log tables
- No random DELETE
- Only time-based deletion

**Why?** This enables:
- Ultra-fast writes (no row locking)
- Data integrity (logs can't be tampered)
- Simplified architecture

**When you need UPDATE/DELETE:**
- Use Volatile table for in-memory data
- Use Lookup table for persistent reference data

## Time-Based Deletion

Clean up old data efficiently:

```sql
-- Delete oldest 1000 rows
DELETE FROM app_logs OLDEST 1000 ROWS;

-- Keep only last 10000 rows
DELETE FROM app_logs EXCEPT 10000 ROWS;

-- Keep only last 7 days
DELETE FROM app_logs EXCEPT 7 DAYS;

-- Delete data before specific date
DELETE FROM app_logs
BEFORE TO_DATE('2025-01-01', 'YYYY-MM-DD');
```

## Indexes

Machbase automatically creates indexes optimally:

- **Tag table**: 3-level partitioned index (automatic)
- **Log table**: LSM index (optional, created with CREATE INDEX)
- **Volatile table**: Red-black tree index (for PRIMARY KEY)
- **Lookup table**: LSM index (optional)

Most users don't need to manage indexes manually!

## Rollup Tables (Tag Tables Only)

Tag tables automatically generate statistics:

```sql
-- Create tag table with SUMMARIZED columns
CREATE TAGDATA TABLE sensors (
    sensor_name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    temperature DOUBLE SUMMARIZED
);

-- Query hourly statistics automatically
SELECT * FROM sensors WHERE rollup = hour;
-- Returns: MIN, MAX, AVG, SUM, COUNT, SUMSQ
```

Three automatic rollup levels:
- Per second
- Per minute
- Per hour

## Compression

Machbase automatically compresses data:

- **Logical compression**: Column-based compression (up to 100x)
- **Physical compression**: Block-level compression (patented)

You don't need to configure anything - it just works!

## Key Terminology

| Term | Meaning |
|------|---------|
| **Tag** | A sensor or data source identifier |
| **BASETIME** | The timestamp column in Tag tables |
| **SUMMARIZED** | Marks columns for automatic rollup statistics |
| **_arrival_time** | Auto-generated timestamp (nanosecond precision) |
| **DURATION** | Keyword for time-range queries |
| **Rollup** | Automatically generated statistical summaries |
| **LSM Index** | Log-Structured Merge index (for fast writes) |

## Best Practices

### 1. Choose the Right Table Type

- **High-volume sensors with many IDs** → Tag table
- **Application logs, events** → Log table
- **Real-time updates needed** → Volatile table
- **Configuration, reference data** → Lookup table

### 2. Use DURATION for Time Queries

```sql
-- Good (optimized)
SELECT * FROM logs DURATION 1 HOUR;

-- Less optimal
SELECT * FROM logs
WHERE _arrival_time >= NOW() - INTERVAL '1' HOUR;
```

### 3. Implement Data Retention

Set up automated cleanup:

```sql
-- Keep only 30 days of data
DELETE FROM app_logs EXCEPT 30 DAYS;
```

Consider setting up a cron job for this.

### 4. Use Tag Tables for Multi-Sensor Data

If you have 1000 sensors, don't create 1000 tables!

```sql
-- Good: One Tag table for all sensors
CREATE TAGDATA TABLE all_sensors (
    sensor_id VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
);

-- Query specific sensor
SELECT * FROM all_sensors
WHERE sensor_id = 'sensor123'
AND time BETWEEN ... AND ...;
```

## Common Patterns

### Pattern 1: IoT Sensor Collection

```sql
-- Tag table for sensor data
CREATE TAGDATA TABLE sensors (...);

-- Lookup table for sensor metadata
CREATE LOOKUP TABLE sensor_info (
    sensor_id VARCHAR(20),
    location VARCHAR(100),
    type VARCHAR(50)
);
```

### Pattern 2: Application Monitoring

```sql
-- Log table for application logs
CREATE TABLE app_logs (...);

-- Log table for access logs
CREATE TABLE access_logs (...);

-- Volatile table for live user sessions
CREATE VOLATILE TABLE active_sessions (...);
```

### Pattern 3: Manufacturing

```sql
-- Tag table for equipment sensors
CREATE TAGDATA TABLE equipment_sensors (...);

-- Log table for production events
CREATE TABLE production_events (...);

-- Lookup table for equipment registry
CREATE LOOKUP TABLE equipment_list (...);
```

## What's Next?

Now that you understand the core concepts:

1. [**Tutorials**](../../tutorials/) - Practice with real-world scenarios
2. [**Common Tasks**](../../common-tasks/) - Learn everyday operations
3. [**Table Types Deep Dive**](../../table-types/) - Detailed documentation

## Quick Reference

```sql
-- TAG TABLE (sensor data)
CREATE TAGDATA TABLE t (
    name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
);

-- LOG TABLE (flexible time-series)
CREATE TABLE t (
    col1 TYPE,
    col2 TYPE
);
-- Auto-adds _arrival_time

-- VOLATILE TABLE (in-memory)
CREATE VOLATILE TABLE t (
    id INTEGER PRIMARY KEY,
    value TYPE
);

-- LOOKUP TABLE (reference data)
CREATE LOOKUP TABLE t (
    id INTEGER,
    name VARCHAR(100)
);
```
