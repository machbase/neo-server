# Table Types: Complete Guide

Master the art of choosing the right table type for your data. This comprehensive guide compares all four Machbase table types with decision frameworks, performance characteristics, and real-world examples.

## The Four Table Types

Machbase provides four specialized table types, each optimized for different workloads:

1. **Tag Table** - Sensor/device time-series data
2. **Log Table** - Event streams and logs
3. **Volatile Table** - In-memory real-time data
4. **Lookup Table** - Reference and master data

## Quick Decision Guide

### Start Here

Answer these questions to find your table type:

```
┌─────────────────────────────────────────────────┐
│ What kind of data do you have?                  │
└─────────────────────────────────────────────────┘
                      │
        ┌─────────────┴─────────────┐
        │                           │
    Persistent?                 Temporary?
        │                           │
        ▼                           ▼
    ┌───────┐                 ┌──────────┐
    │ YES   │                 │ Volatile │
    └───┬───┘                 │  Table   │
        │                     └──────────┘
        ▼
    Sensor data
    (ID, time, value)?
        │
    ┌───┴────┐
    │        │
   YES      NO
    │        │
    ▼        ▼
  Tag     Log/Event
  Table    data?
            │
        ┌───┴────┐
        │        │
       YES      NO
        │        │
        ▼        ▼
      Log     Lookup
      Table    Table
```

### Decision Table

| Your Data | Recommended Table | Why |
|-----------|------------------|-----|
| Temperature sensors from 1000 devices | **Tag Table** | Multiple sensors, time-series values |
| Application error logs | **Log Table** | Event stream, flexible schema |
| Live user sessions | **Volatile Table** | Needs UPDATE, temporary |
| Device metadata/registry | **Lookup Table** | Reference data, rare updates |
| Stock market ticks | **Tag Table** | Symbol as tag, price as value |
| HTTP access logs | **Log Table** | Event-based, many columns |
| Shopping cart contents | **Volatile Table** | Frequent updates, session-based |
| Product catalog | **Lookup Table** | Master data, infrequent changes |

## Tag Table Deep Dive

### When to Use

Perfect for:
- IoT sensor data (temp, humidity, pressure)
- Industrial equipment telemetry
- Smart meters
- GPS tracking
- Any data with (sensor_id, timestamp, value) pattern

### Structure

```sql
CREATE TAGDATA TABLE sensors (
    sensor_id VARCHAR(20) PRIMARY KEY,    -- Tag name (sensor identifier)
    time DATETIME BASETIME,               -- Timestamp
    value DOUBLE SUMMARIZED,              -- Measured value(s)
    other_value DOUBLE SUMMARIZED
);
```

### Key Features

**Automatic Rollup Statistics**:
```sql
-- Raw data
INSERT INTO sensors VALUES ('sensor01', NOW, 25.3);

-- Automatic statistics (no manual work!)
SELECT * FROM sensors WHERE rollup = hour;
-- Returns: min_value, max_value, avg_value, sum_value, count, sumsq_value
```

**Metadata Layer**:
```sql
-- Separate table for sensor metadata
SELECT * FROM sensors._META;

-- Add custom metadata columns
ALTER TABLE sensors._META ADD COLUMN location VARCHAR(100);
UPDATE sensors._META SET location = 'Building A' WHERE name = 'sensor01';
```

**Performance**:
- Millions of inserts per second
- Ultra-fast queries by sensor_id + time
- Automatic 3-level partitioned indexing

### Best Practices

**DO**:
- Use for multi-sensor data (1000s of sensors in one table)
- Mark analytical columns as SUMMARIZED
- Query rollup tables for statistics
- Use metadata table for sensor info

**DON'T**:
- Create separate tables for each sensor
- Try to UPDATE data values (use metadata for updates)
- Use for non-sensor data

### Example Use Cases

```sql
-- Manufacturing: Equipment sensors
CREATE TAGDATA TABLE equipment_telemetry (
    equipment_id VARCHAR(50) PRIMARY KEY,
    time DATETIME BASETIME,
    temperature DOUBLE SUMMARIZED,
    vibration DOUBLE SUMMARIZED,
    rpm DOUBLE SUMMARIZED,
    power_consumption DOUBLE SUMMARIZED
);

-- Smart City: Environmental monitoring
CREATE TAGDATA TABLE air_quality (
    station_id VARCHAR(30) PRIMARY KEY,
    time DATETIME BASETIME,
    pm25 DOUBLE SUMMARIZED,
    pm10 DOUBLE SUMMARIZED,
    co2 DOUBLE SUMMARIZED,
    temperature DOUBLE SUMMARIZED
);
```

## Log Table Deep Dive

### When to Use

Perfect for:
- Application logs
- Event streams
- Access logs
- Transaction logs
- Any time-stamped events with variable schema

### Structure

```sql
CREATE TABLE app_logs (
    level VARCHAR(10),
    component VARCHAR(50),
    message VARCHAR(2000),
    user_id INTEGER,
    ip_addr IPV4
    -- _arrival_time automatically added!
);
```

### Key Features

**Automatic Timestamps**:
```sql
-- You insert
INSERT INTO app_logs VALUES ('ERROR', 'DB', 'Connection timeout', 123, '192.168.1.1');

-- Machbase stores with nanosecond timestamp
-- _arrival_time: 2025-10-10 14:23:45.123456789
```

**Full-Text Search**:
```sql
-- Fast text search
SELECT * FROM app_logs
WHERE message SEARCH 'timeout'
  AND level = 'ERROR';
```

**Flexible Schema**:
- Any number of columns
- Any data types
- No fixed pattern required

**Performance**:
- Millions of inserts per second
- Newest data returned first (automatic ordering)
- Optional LSM indexing for fast lookups

### Best Practices

**DO**:
- Use for variable event data
- Leverage SEARCH for text queries
- Use DURATION for time-based queries
- Implement retention policies

**DON'T**:
- Use for sensor data (use Tag table instead)
- Store reference data (use Lookup table)
- Expect UPDATE/DELETE by key

### Example Use Cases

```sql
-- Application monitoring
CREATE TABLE application_events (
    app_name VARCHAR(50),
    event_type VARCHAR(50),
    severity VARCHAR(20),
    message VARCHAR(2000),
    user_id INTEGER,
    session_id VARCHAR(100),
    stack_trace VARCHAR(4000)
);

-- Web server access logs
CREATE TABLE http_access (
    method VARCHAR(10),
    uri VARCHAR(1000),
    status_code INTEGER,
    response_time INTEGER,
    client_ip IPV4,
    user_agent VARCHAR(500),
    referer VARCHAR(500)
);

-- Financial transactions
CREATE TABLE transactions (
    transaction_id VARCHAR(50),
    account_id INTEGER,
    transaction_type VARCHAR(30),
    amount DOUBLE,
    currency VARCHAR(3),
    status VARCHAR(20),
    description VARCHAR(500)
);
```

## Volatile Table Deep Dive

### When to Use

Perfect for:
- Real-time dashboards
- Session management
- Live status boards
- Caching layer
- Any data requiring UPDATE/DELETE

### Structure

```sql
CREATE VOLATILE TABLE live_status (
    device_id INTEGER PRIMARY KEY,    -- PRIMARY KEY required for updates
    status VARCHAR(20),
    last_value DOUBLE,
    last_updated DATETIME
);
```

### Key Features

**UPDATE and DELETE by Key**:
```sql
-- Update existing record
UPDATE live_status
SET status = 'RUNNING', last_value = 25.3, last_updated = NOW
WHERE device_id = 101;

-- Delete specific record
DELETE FROM live_status WHERE device_id = 101;
```

**In-Memory Storage**:
- All data in RAM
- Extremely fast reads/writes
- 10,000s of operations per second

**WARNING: Non-Persistent**:
- Data lost on server restart
- Archive important data before shutdown

### Best Practices

**DO**:
- Use PRIMARY KEY for fast lookups
- Keep data volume small (limited by RAM)
- Archive to Log/Tag tables periodically
- Use for current state only

**DON'T**:
- Store data that must persist
- Use for high-volume streaming data
- Expect data to survive restarts

### Example Use Cases

```sql
-- Real-time equipment status
CREATE VOLATILE TABLE equipment_status (
    equipment_id INTEGER PRIMARY KEY,
    online CHAR(1),
    current_temp DOUBLE,
    current_pressure DOUBLE,
    last_heartbeat DATETIME
);

-- Active user sessions
CREATE VOLATILE TABLE user_sessions (
    session_token VARCHAR(100) PRIMARY KEY,
    user_id INTEGER,
    ip_address IPV4,
    login_time DATETIME,
    last_activity DATETIME,
    expires_at DATETIME
);

-- Live monitoring cache
CREATE VOLATILE TABLE monitoring_cache (
    metric_key VARCHAR(100) PRIMARY KEY,
    metric_value VARCHAR(500),
    updated_at DATETIME
);
```

## Lookup Table Deep Dive

### When to Use

Perfect for:
- Device registries
- Configuration tables
- Category/dimension tables
- Master data
- Reference data that changes rarely

### Structure

```sql
CREATE LOOKUP TABLE devices (
    device_id INTEGER,
    device_name VARCHAR(100),
    location VARCHAR(200),
    device_type VARCHAR(50),
    owner VARCHAR(100)
);
```

### Key Features

**Full CRUD Support**:
```sql
-- Insert
INSERT INTO devices VALUES (101, 'Sensor A', 'Building 1', 'Temperature', 'Facilities');

-- Update
UPDATE devices SET location = 'Building 2' WHERE device_id = 101;

-- Delete
DELETE FROM devices WHERE device_id = 101;

-- Select
SELECT * FROM devices WHERE device_type = 'Temperature';
```

**JOIN with Time-Series**:
```sql
-- Enrich sensor data with device info
SELECT s.*, d.device_name, d.location
FROM sensors s
JOIN devices d ON s.sensor_id = d.device_id
DURATION 1 HOUR;
```

**Performance**:
- Fast reads
- Slower writes (100s per second)
- Persistent disk storage

### Best Practices

**DO**:
- Use for reference/master data
- JOIN with Tag/Log tables
- Index frequently queried columns
- Keep data volume reasonable (<1M rows ideal)

**DON'T**:
- Use for high-frequency inserts
- Store time-series data
- Expect millions of writes per second

### Example Use Cases

```sql
-- Device registry
CREATE LOOKUP TABLE device_registry (
    device_id VARCHAR(50),
    device_name VARCHAR(100),
    device_type VARCHAR(50),
    location VARCHAR(200),
    installation_date DATETIME,
    status VARCHAR(20)
);

-- Configuration management
CREATE LOOKUP TABLE system_config (
    config_key VARCHAR(100),
    config_value VARCHAR(500),
    config_category VARCHAR(50),
    description VARCHAR(500)
);

-- User management
CREATE LOOKUP TABLE users (
    user_id INTEGER,
    username VARCHAR(100),
    email VARCHAR(200),
    role VARCHAR(50),
    created_at DATETIME
);
```

## Performance Comparison

### Write Performance

| Table Type | Inserts/sec | UPDATE Support | DELETE Support |
|-----------|-------------|----------------|----------------|
| Tag | Millions | Metadata only | Time-based |
| Log | Millions | No | Time-based |
| Volatile | 10,000s | By PRIMARY KEY | By PRIMARY KEY |
| Lookup | 100s | Yes | Yes |

### Read Performance

| Table Type | Read Speed | Best For | Index Type |
|-----------|-----------|----------|------------|
| Tag | Very Fast | sensor_id + time | 3-level partitioned |
| Log | Fast | Time range | LSM (optional) |
| Volatile | Very Fast | PRIMARY KEY | Red-black tree |
| Lookup | Fast | Any column | LSM (optional) |

### Storage

| Table Type | Storage | Compression | Persistence |
|-----------|---------|-------------|-------------|
| Tag | Disk | 10-100x | Yes |
| Log | Disk | 10-100x | Yes |
| Volatile | Memory | None | No |
| Lookup | Disk | Moderate | Yes |

## Combining Table Types

### Pattern: IoT Platform

```sql
-- Tag: Sensor readings
CREATE TAGDATA TABLE sensor_data (...);

-- Lookup: Device registry
CREATE LOOKUP TABLE devices (...);

-- Volatile: Live status
CREATE VOLATILE TABLE device_status (...);

-- Log: Events and alerts
CREATE TABLE device_events (...);
```

### Pattern: Web Application

```sql
-- Log: Access logs
CREATE TABLE http_access (...);

-- Log: Application logs
CREATE TABLE app_logs (...);

-- Volatile: Active sessions
CREATE VOLATILE TABLE sessions (...);

-- Lookup: User accounts
CREATE LOOKUP TABLE users (...);
```

### Pattern: Manufacturing

```sql
-- Tag: Equipment sensors
CREATE TAGDATA TABLE equipment_telemetry (...);

-- Log: Production events
CREATE TABLE production_log (...);

-- Volatile: Line status
CREATE VOLATILE TABLE line_status (...);

-- Lookup: Equipment catalog
CREATE LOOKUP TABLE equipment_catalog (...);
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Wrong Table for Use Case

**Bad**: Using Log table for sensor data
```sql
-- Don't do this!
CREATE TABLE sensors (sensor_id VARCHAR(20), value DOUBLE);
```

**Good**: Use Tag table
```sql
CREATE TAGDATA TABLE sensors (
    sensor_id VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
);
```

### Anti-Pattern 2: One Table Per Sensor

**Bad**: Creating 1000 tables for 1000 sensors
```sql
CREATE TAGDATA TABLE sensor001 (...);
CREATE TAGDATA TABLE sensor002 (...);
-- ... 998 more tables
```

**Good**: One table for all sensors
```sql
CREATE TAGDATA TABLE all_sensors (
    sensor_id VARCHAR(20) PRIMARY KEY,
    ...
);
```

### Anti-Pattern 3: Storing History in Volatile

**Bad**: Using Volatile for persistent data
```sql
-- Data will be lost on restart!
CREATE VOLATILE TABLE important_transactions (...);
```

**Good**: Use Log or Tag table
```sql
CREATE TABLE important_transactions (...);
```

### Anti-Pattern 4: High-Frequency Writes to Lookup

**Bad**: Millions of inserts to Lookup table
```sql
-- Too slow!
CREATE LOOKUP TABLE sensor_readings (...);
```

**Good**: Use Tag or Log table
```sql
CREATE TAGDATA TABLE sensor_readings (...);
```

## Migration Guide

### From Other Databases

**From PostgreSQL/MySQL**:
- Regular tables → Log tables
- Time-series tables → Tag tables
- Temp tables → Volatile tables
- Dimension tables → Lookup tables

**From InfluxDB**:
- Measurements → Tag tables
- Tags → Tag primary key + metadata
- Fields → SUMMARIZED columns

**From MongoDB**:
- Time-series collections → Tag/Log tables
- Reference collections → Lookup tables
- Capped collections → Log tables with retention

## Summary Matrix

| Feature | Tag | Log | Volatile | Lookup |
|---------|-----|-----|----------|--------|
| **Primary Use** | Sensors | Events | Cache | Reference |
| **Schema** | Fixed pattern | Flexible | Flexible | Flexible |
| **Writes/sec** | Millions | Millions | 10,000s | 100s |
| **UPDATE** | Metadata | No | Yes | Yes |
| **DELETE** | Time-based | Time-based | By key | By key |
| **Storage** | Disk | Disk | Memory | Disk |
| **Persistence** | Yes | Yes | No | Yes |
| **Rollup** | Auto | No | No | No |
| **Best Query** | ID + time | Time | Key | Any |
| **Compression** | Very high | High | None | Moderate |

## Next Steps

- **Deep Dive**: [Indexing and Performance](../indexing/) - Optimize queries
- **Detailed Reference**: [Table Types](../../table-types/) - Complete documentation
- **Hands-On**: [Tutorials](../../tutorials/) - Practice with real examples

## Key Takeaways

1. **Tag tables** for sensor/device data with automatic rollup
2. **Log tables** for flexible event streams and logs
3. **Volatile tables** for in-memory, update-able data
4. **Lookup tables** for reference and master data
5. **Combine types** for complete solutions
6. **Choose wisely** - table type determines performance

---

Master table selection and build efficient Machbase applications!
