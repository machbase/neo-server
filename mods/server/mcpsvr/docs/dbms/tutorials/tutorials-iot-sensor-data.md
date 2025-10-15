# Tutorial 1: IoT Sensor Data

Learn how to collect and analyze IoT sensor data using Machbase Tag tables. This tutorial simulates a real-world scenario with temperature and humidity sensors.

## Scenario

You're building a warehouse monitoring system with:
- 10 temperature/humidity sensors across different zones
- Readings every 10 seconds from each sensor
- Need to track trends and identify anomalies
- Must keep 30 days of historical data

**Goal**: Store sensor data efficiently and query it for real-time monitoring and historical analysis.

## What You'll Learn

- Creating Tag tables for sensor data
- Inserting high-frequency sensor readings
- Querying by sensor ID and time range
- Using automatic rollup statistics
- Implementing data retention

## Prerequisites

- Machbase installed and running
- machsql client connected
- 15 minutes of time

## Step 1: Create Tag Table

Tag tables are perfect for sensor data with (ID, timestamp, value) structure.

```sql
CREATE TAGDATA TABLE warehouse_sensors (
    sensor_id VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    temperature DOUBLE SUMMARIZED,
    humidity DOUBLE SUMMARIZED
);
```

**Understanding the schema**:
- `sensor_id`: Unique identifier for each sensor
- `time`: Timestamp when reading was taken
- `temperature`, `humidity`: The values we're measuring
- `SUMMARIZED`: Tells Machbase to generate automatic statistics

**Verify the table**:
```sql
SHOW TABLE warehouse_sensors;
```

## Step 2: Insert Sensor Data

Insert sample readings from different sensors:

```sql
-- Zone 1 sensors
INSERT INTO warehouse_sensors VALUES ('zone1-temp01', NOW, 22.5, 55.2);
INSERT INTO warehouse_sensors VALUES ('zone1-temp01', NOW, 22.7, 55.0);
INSERT INTO warehouse_sensors VALUES ('zone1-temp01', NOW, 22.6, 55.1);

-- Zone 2 sensors
INSERT INTO warehouse_sensors VALUES ('zone2-temp01', NOW, 21.3, 60.5);
INSERT INTO warehouse_sensors VALUES ('zone2-temp01', NOW, 21.5, 60.3);
INSERT INTO warehouse_sensors VALUES ('zone2-temp01', NOW, 21.4, 60.7);

-- Zone 3 sensors
INSERT INTO warehouse_sensors VALUES ('zone3-temp01', NOW, 23.1, 52.8);
INSERT INTO warehouse_sensors VALUES ('zone3-temp01', NOW, 23.3, 52.5);
INSERT INTO warehouse_sensors VALUES ('zone3-temp01', NOW, 23.2, 52.6);
```

**In production**, you'd insert data using:
- CLI/ODBC API from your application
- Machbase APPEND protocol for bulk inserts
- CSV import tools

## Step 3: Query Recent Data

Get the latest readings from all sensors:

```sql
-- Last 10 minutes of data
SELECT * FROM warehouse_sensors
DURATION 10 MINUTE;
```

Get latest reading from a specific sensor:

```sql
SELECT * FROM warehouse_sensors
WHERE sensor_id = 'zone1-temp01'
DURATION 1 HOUR;
```

## Step 4: Use Rollup Statistics

Tag tables automatically generate statistics. Query hourly averages:

```sql
-- Get hourly statistics for a sensor
SELECT
    sensor_id,
    time,
    min_temperature,
    max_temperature,
    avg_temperature,
    avg_humidity
FROM warehouse_sensors
WHERE sensor_id = 'zone1-temp01'
  AND rollup = hour;
```

**Rollup levels available**:
- `rollup = sec` - Per-second statistics
- `rollup = min` - Per-minute statistics
- `rollup = hour` - Per-hour statistics

## Step 5: Analyze Trends

Find sensors with high temperatures:

```sql
-- Sensors above 23°C in last hour
SELECT DISTINCT sensor_id, max_temperature
FROM warehouse_sensors
WHERE rollup = min
  AND max_temperature > 23.0
DURATION 1 HOUR;
```

Calculate averages across all sensors:

```sql
-- Average temperature by zone (last 24 hours)
SELECT
    sensor_id,
    AVG(avg_temperature) as daily_avg_temp,
    AVG(avg_humidity) as daily_avg_humidity
FROM warehouse_sensors
WHERE rollup = hour
DURATION 24 HOUR
GROUP BY sensor_id;
```

## Step 6: Handle Tag Metadata

Tag tables have a special metadata layer for sensor information:

```sql
-- Insert metadata for sensors
INSERT INTO warehouse_sensors._META
VALUES ('zone1-temp01');

INSERT INTO warehouse_sensors._META
VALUES ('zone2-temp01');

INSERT INTO warehouse_sensors._META
VALUES ('zone3-temp01');

-- Query metadata
SELECT * FROM warehouse_sensors._META;
```

Add custom metadata columns:

```sql
-- Add location metadata
ALTER TABLE warehouse_sensors._META
ADD COLUMN location VARCHAR(50);

-- Update metadata
UPDATE warehouse_sensors._META
SET location = 'North Warehouse'
WHERE name = 'zone1-temp01';

-- Query with metadata
SELECT * FROM warehouse_sensors._META
WHERE location = 'North Warehouse';
```

## Step 7: Implement Data Retention

Keep only 30 days of historical data:

```sql
-- Delete data older than 30 days
DELETE FROM warehouse_sensors
BEFORE TO_DATE(TO_CHAR(SYSDATE - 30, 'YYYY-MM-DD'), 'YYYY-MM-DD');

-- Or keep last 30 days using EXCEPT
DELETE FROM warehouse_sensors EXCEPT 30 DAYS;
```

**Best practice**: Set up a daily cron job to run this cleanup automatically.

## Step 8: Monitor for Anomalies

Create a query to detect temperature spikes:

```sql
-- Find sensors with temperature change > 5°C in 1 hour
SELECT
    sensor_id,
    max_temperature - min_temperature as temp_range,
    avg_temperature
FROM warehouse_sensors
WHERE rollup = hour
  AND (max_temperature - min_temperature) > 5.0
DURATION 24 HOUR;
```

## Try It Yourself

### Exercise 1: Add More Sensors

Insert data for additional sensors:

```sql
-- Add zone4 and zone5 sensors
-- Try different temperature and humidity ranges
INSERT INTO warehouse_sensors VALUES ('zone4-temp01', NOW, 20.1, 65.0);
-- Add more readings...
```

### Exercise 2: Create Alert Query

Write a query to find:
- Sensors with humidity > 70%
- In the last 30 minutes

<details>
<summary>Solution</summary>

```sql
SELECT sensor_id, humidity, time
FROM warehouse_sensors
WHERE humidity > 70.0
DURATION 30 MINUTE;
```

Or using rollup:

```sql
SELECT sensor_id, max_humidity
FROM warehouse_sensors
WHERE rollup = min
  AND max_humidity > 70.0
DURATION 30 MINUTE;
```
</details>

### Exercise 3: Historical Analysis

Find the hottest and coldest sensors over the last 7 days:

<details>
<summary>Solution</summary>

```sql
-- Hottest sensor
SELECT sensor_id, MAX(max_temperature) as highest_temp
FROM warehouse_sensors
WHERE rollup = hour
DURATION 7 DAY
GROUP BY sensor_id
ORDER BY highest_temp DESC
LIMIT 1;

-- Coldest sensor
SELECT sensor_id, MIN(min_temperature) as lowest_temp
FROM warehouse_sensors
WHERE rollup = hour
DURATION 7 DAY
GROUP BY sensor_id
ORDER BY lowest_temp ASC
LIMIT 1;
```
</details>

## Real-World Application

Expand this to production by:

### 1. Use Bulk Insert API

Instead of individual INSERTs, use APPEND protocol:

```c
// C/CLI example
SQLAppendOpen(stmt, "warehouse_sensors");
SQLAppendDataV(stmt, "zone1-temp01", time_val, 22.5, 55.2);
SQLAppendDataV(stmt, "zone1-temp01", time_val, 22.7, 55.0);
// ... more records
SQLAppendClose(stmt);
```

### 2. Create Monitoring Dashboard

```sql
-- Real-time dashboard query
SELECT
    sensor_id,
    temperature,
    humidity,
    time
FROM warehouse_sensors
DURATION 5 MINUTE
ORDER BY time DESC;
```

### 3. Set Up Automated Alerts

```bash
#!/bin/bash
# check_sensors.sh - Run every 5 minutes via cron

machsql -s localhost -u SYS -p MANAGER -f - <<EOF
SELECT sensor_id, temperature
FROM warehouse_sensors
WHERE temperature > 30.0
DURATION 5 MINUTE;
EOF
```

### 4. Integrate with Applications

Use SDKs to connect:
- **Python**: Connect via machbase-python
- **Java**: Use JDBC driver
- **C/C++**: CLI/ODBC API
- **REST API**: HTTP-based integration

## Performance Tips

1. **Batch inserts**: Use APPEND protocol for high-volume data
2. **Use rollup**: Query rollup tables instead of raw data for statistics
3. **Limit time ranges**: Always use time conditions in WHERE clause
4. **Proper indexing**: Tag tables auto-index - you don't need to manage it

## Common Patterns

### Pattern: Multi-Value Sensors

```sql
-- Sensor with multiple measurements
CREATE TAGDATA TABLE multi_sensors (
    sensor_id VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    temperature DOUBLE SUMMARIZED,
    pressure DOUBLE SUMMARIZED,
    vibration DOUBLE SUMMARIZED,
    rpm DOUBLE SUMMARIZED
);
```

### Pattern: Hierarchical Sensor IDs

```sql
-- Use structured naming
-- Format: building-floor-room-type-number
INSERT INTO warehouse_sensors
VALUES ('building1-floor2-roomA-temp-01', NOW, 22.5, 55.0);

-- Query by pattern
SELECT * FROM warehouse_sensors
WHERE sensor_id LIKE 'building1-floor2%'
DURATION 1 HOUR;
```
