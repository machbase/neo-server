# Machbase Neo SQL Guide

## Quick Start

### Create table

Open the SQL editor by selecting "SQL" from the menu. The page shows the SQL editor on the left panel and results and logs on the right panel.

Copy the DDL statement below and paste it into the editor:

```sql
CREATE TAG TABLE IF NOT EXISTS example (
  name varchar(100) primary key,
  time datetime basetime,
  value double summarized
);
```

Execute the statement by pressing "Ctrl+Enter" or clicking the ▶︎ icon on the top-left of the panel. Don't forget the semi-colon at the end of the statement.

### Insert Table

Execute the statement below to insert a single record:

```sql
INSERT INTO example VALUES('my-car', now, 1.2345);
```

### Select Table

Execute the SELECT statement below. It will display the results in the right panel:

```sql
SELECT time, value FROM example WHERE name = 'my-car';
```

### Chart Draw

Insert more records by executing insert statement repeatedly.

```sql
INSERT INTO example VALUES('my-car', now, 1.2345*1.1);
INSERT INTO example VALUES('my-car', now, 1.2345*1.2);
INSERT INTO example VALUES('my-car', now, 1.2345*1.3);
```

Then review the stored 'my-car' records.

```sql
SELECT time, value FROM example WHERE name = 'my-car';
```

Click the *CHART* tab on the right side panel. It will display a line chart with the query results.

### Download CSV file

The full result of the query can be exported in a CSV file.

### Delete Table

Delete records with a *DELETE* statement.

```sql
DELETE FROM example WHERE name = 'my-car'
```

Or, remove the table if you want to create a fresh one.

```sql
DROP TABLE example;
```

## Non-SQL

### show tables

Simplified command that queries `M$SYS_TABLES` table.

```
show tables;
```

### desc _table_name_

Describe table's columns and related index.

```
desc example;
```

### show tags _table_name_

```
show tags example;
```

Query stored tags of the table, it works to TAG table only.

---

## SQL Examples

This section contains SQL examples that are actually executable and verified in Machbase Neo.

### 1. Table Creation - 10 Examples

#### 1.1 Basic TAG Table Creation
**Purpose**: Basic table for storing time-series data
**Keywords**: CREATE TAG TABLE, PRIMARY KEY, BASETIME, SUMMARIZED

```sql
-- Basic time-series table
CREATE TAG TABLE sensor_data (
    name VARCHAR(80) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
);
```

#### 1.2 TAG Table with Additional Columns
**Purpose**: Table with one SUMMARIZED column and additional data columns
**Keywords**: Single SUMMARIZED, Additional columns

```sql
-- Table with additional columns (only one SUMMARIZED allowed)
CREATE TAG TABLE enhanced_sensor (
    device_id VARCHAR(50) PRIMARY KEY,
    timestamp DATETIME BASETIME,
    temperature DOUBLE SUMMARIZED,  -- Only one SUMMARIZED allowed
    location VARCHAR(100),           -- Regular column
    status INTEGER                   -- Regular column
);
```

#### 1.3 Table with Full Rollup
**Purpose**: Automatic aggregation by second/minute/hour units
**Keywords**: WITH ROLLUP

```sql
-- Full Rollup for all units (second/minute/hour)
CREATE TAG TABLE iot_sensors (
    name VARCHAR(80) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
) WITH ROLLUP;
```

#### 1.4 Table with Rollup (Minute/Hour)
**Purpose**: Aggregation for minute/hour units only
**Keywords**: WITH ROLLUP(MIN)

```sql
-- Rollup for minute/hour units only
CREATE TAG TABLE hourly_stats (
    tag_id VARCHAR(50) PRIMARY KEY,
    event_time DATETIME BASETIME,
    temperature DOUBLE SUMMARIZED
) WITH ROLLUP(MIN);
```

#### 1.5 Table with Rollup (Hour Only)
**Purpose**: Aggregation for hour unit only
**Keywords**: WITH ROLLUP(HOUR)

```sql
-- Rollup for hour unit only
CREATE TAG TABLE daily_summary (
    sensor_id VARCHAR(50) PRIMARY KEY,
    timestamp DATETIME BASETIME,
    avg_value DOUBLE SUMMARIZED
) WITH ROLLUP(HOUR);
```

#### 1.6 Table with Statistics Feature
**Purpose**: Automatic collection of statistics information by tag
**Keywords**: TAG_STAT_ENABLE

```sql
-- Statistics feature enabled
CREATE TAG TABLE stat_enabled_table (
    name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
) TAG_STAT_ENABLE = 1;
```

#### 1.7 Outlier Removal (Minimum Value Validation)
**Purpose**: Automatic filtering of data below minimum value
**Keywords**: LOWER LIMIT

```sql
-- Pressure monitoring (minimum value validation only)
CREATE TAG TABLE pressure_monitor (
    tag_id VARCHAR(50) PRIMARY KEY,
    event_time DATETIME BASETIME,
    pressure_kpa INTEGER SUMMARIZED
);
```

#### 1.8 Outlier Removal (Min/Max Value Validation)
**Purpose**: Automatic filtering of data outside range
**Keywords**: LOWER LIMIT, UPPER LIMIT

```sql
-- Temperature monitoring (min/max value validation)
CREATE TAG TABLE temperature_sensor (
    sensor_name VARCHAR(50) PRIMARY KEY,
    measurement_time DATETIME BASETIME,
    temp_celsius DOUBLE SUMMARIZED
);
```

#### 1.9 Table with Data Retention Policy
**Purpose**: Managing old data by limiting partition count
**Keywords**: TAG_PARTITION_COUNT

```sql
-- Table with retention policy
CREATE TAG TABLE retention_table (
    name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
) WITH ROLLUP(MIN) 
TAG_PARTITION_COUNT = 4;
```

#### 1.10 Individual Advanced Feature Tables
**Purpose**: Applying advanced features individually
**Keywords**: Individual option application

```sql
-- Table with Rollup Extension feature
CREATE TAG TABLE rollup_extended_sensor (
    device_name VARCHAR(100) PRIMARY KEY,
    recorded_time DATETIME BASETIME,
    sensor_value DOUBLE SUMMARIZED
) WITH ROLLUP EXTENSION;

-- Table with statistics feature  
CREATE TAG TABLE stat_enabled_sensor (
    device_name VARCHAR(100) PRIMARY KEY,
    recorded_time DATETIME BASETIME,
    sensor_value DOUBLE SUMMARIZED
) TAG_STAT_ENABLE = 1;

-- Table with partition count limit
CREATE TAG TABLE partition_limited_sensor (
    device_name VARCHAR(100) PRIMARY KEY,
    recorded_time DATETIME BASETIME,
    sensor_value DOUBLE SUMMARIZED
) TAG_PARTITION_COUNT = 12;
```

---

### 2. Data Insertion - 10 Examples

#### 2.1 Basic Single INSERT
**Purpose**: Insert a single record
**Keywords**: INSERT INTO VALUES

```sql
-- Single record insertion
INSERT INTO sensor_data VALUES ('TEMP_A', '2024-03-10 10:05:15', 20.1);
```

#### 2.2 Current Time Single INSERT
**Purpose**: Insert data with current timestamp
**Keywords**: NOW

```sql
-- Insert data with current time
INSERT INTO sensor_data VALUES ('SENSOR_01', NOW, 25.5);
```

#### 2.3 INSERT with Additional Columns
**Purpose**: Insert data into all columns
**Keywords**: Full column insertion

```sql
-- Insert data into all columns
INSERT INTO enhanced_sensor VALUES ('DEVICE_01', NOW, 23.5, 'Building A', 1);
```

#### 2.4 Millisecond Precision INSERT
**Purpose**: Insert data with millisecond precision timestamp
**Keywords**: Millisecond timestamp

```sql
-- Millisecond precision time insertion (supports up to .123)
INSERT INTO sensor_data VALUES ('HIGH_FREQ', '2024-03-10 10:05:15.123', 45.67);
```

#### 2.5 Sequential Single INSERTs
**Purpose**: Insert multiple records individually (batch INSERT not supported)
**Keywords**: Sequential insertion

```sql
-- Individual INSERTs (batch INSERT syntax not supported)
INSERT INTO iot_sensors VALUES ('SENSOR_01', '2024-03-10 10:00:00', 10.0);
INSERT INTO iot_sensors VALUES ('SENSOR_01', '2024-03-10 10:00:01', 10.1);
INSERT INTO iot_sensors VALUES ('SENSOR_01', '2024-03-10 10:00:02', 10.2);
```

#### 2.6 Multiple Tag Sequential INSERT
**Purpose**: Individual insertion of data from multiple sensors
**Keywords**: Multi-tag sequential insertion

```sql
-- Individual insertion for multiple tags
INSERT INTO sensor_data VALUES ('CPU_TEMP', '2024-03-10 15:30:00', 68.5);
INSERT INTO sensor_data VALUES ('CPU_USAGE', '2024-03-10 15:30:00', 85.2);
INSERT INTO sensor_data VALUES ('MEMORY_USAGE', '2024-03-10 15:30:00', 76.4);
```

#### 2.7 Normal Range Data INSERT
**Purpose**: Test outlier feature - normal range
**Keywords**: Normal value insertion

```sql
-- Normal range data (success)
INSERT INTO temperature_sensor (sensor_name, measurement_time, temp_celsius) 
VALUES ('ROOM_A', NOW, 22.5);
```

#### 2.8 Partial Column INSERT
**Purpose**: Insert only required columns
**Keywords**: Partial INSERT

```sql
-- Insert only required columns (excluding NULL-allowed columns)
INSERT INTO enhanced_sensor (device_id, timestamp, temperature) 
VALUES ('DEVICE_03', NOW, 25.8);
```

#### 2.9 Integer Data INSERT
**Purpose**: Insert into integer SUMMARIZED column
**Keywords**: INTEGER type

```sql
-- Integer data insertion
INSERT INTO pressure_monitor (tag_id, event_time, pressure_kpa) 
VALUES ('VALVE_01', NOW, 1250);
```

---

### 3. Data Selection - 10 Examples

#### 3.1 Basic Full Data Query
**Purpose**: Check latest data
**Keywords**: SELECT *, ORDER BY, LIMIT

```sql
-- Query all data in latest order
SELECT * FROM sensor_data 
ORDER BY time DESC 
LIMIT 100;
```

#### 3.2 Specific Tag Data Query
**Purpose**: Check data from specific sensor only
**Keywords**: WHERE name

```sql
-- Query specific tag data only
SELECT * FROM sensor_data 
WHERE name = 'TEMP_A' 
ORDER BY time DESC 
LIMIT 10;
```

#### 3.3 Time Range Query
**Purpose**: Analyze data for specific period
**Keywords**: BETWEEN

```sql
-- Time range query
SELECT name, time, value FROM sensor_data 
WHERE time BETWEEN '2024-03-10 09:00:00' AND '2024-03-10 18:00:00'
ORDER BY time;
```

#### 3.4 Conditional Value Filtering
**Purpose**: Search for data exceeding threshold
**Keywords**: WHERE condition

```sql
-- Data exceeding threshold
SELECT name, time, value 
FROM sensor_data 
WHERE value > 25.0 
ORDER BY time DESC;
```

#### 3.5 Multiple Tag Query
**Purpose**: Check multiple sensor data simultaneously
**Keywords**: IN

```sql
-- Query multiple tags simultaneously
SELECT name, time, value FROM sensor_data 
WHERE name IN ('TEMP_A', 'TEMP_B', 'HUMIDITY_A')
ORDER BY name, time DESC;
```

#### 3.6 Pattern Matching Query
**Purpose**: Search tag names with specific pattern
**Keywords**: LIKE

```sql
-- Pattern matching query
SELECT * FROM sensor_data 
WHERE name LIKE 'TEMP_%' 
AND time >= '2024-03-10'
ORDER BY name, time;
```

#### 3.7 Aggregate Function Query
**Purpose**: Calculate statistics by group
**Keywords**: GROUP BY, AVG, COUNT

```sql
-- Calculate average by tag
SELECT name, 
       COUNT(*) as record_count,
       AVG(value) as avg_value,
       MIN(value) as min_value,
       MAX(value) as max_value
FROM sensor_data 
WHERE time >= '2024-03-01'
GROUP BY name
ORDER BY name;
```

#### 3.8 Latest Time by Tag Query
**Purpose**: Check latest data time for each tag
**Keywords**: GROUP BY, MAX

```sql
-- Latest data time for each tag
SELECT name, MAX(time) as latest_time 
FROM sensor_data 
GROUP BY name 
ORDER BY name;
```

#### 3.9 Fixed Time-Based Filtering
**Purpose**: Query data after specific timestamp
**Keywords**: Fixed time condition

```sql
-- Data after specific timestamp (fixed time only)
SELECT * FROM sensor_data 
WHERE time >= '2025-08-12 00:00:00'
ORDER BY time DESC;
```

#### 3.10 Complex Condition Query
**Purpose**: Search with multiple combined conditions
**Keywords**: AND, OR conditions

```sql
-- Complex condition query
SELECT name, time, value FROM sensor_data 
WHERE (name LIKE 'TEMP_%' AND value > 20.0)
   OR (name LIKE 'PRESSURE_%' AND value > 1000.0)
ORDER BY time DESC
LIMIT 50;
```

---

### 4. Basic Aggregations - 10 Examples

#### 4.1 Basic Statistics by Tag
**Purpose**: Basic statistical information for each tag
**Keywords**: GROUP BY basic aggregation

```sql
-- Basic statistics by tag
SELECT name,
       COUNT(*) as count_value,
       AVG(value) as avg_value,
       MIN(value) as min_value,
       MAX(value) as max_value,
       MAX(value) - MIN(value) as value_range
FROM iot_sensors
GROUP BY name
ORDER BY name;
```

#### 4.2 Overall Data Summary
**Purpose**: System-wide data summary
**Keywords**: Overall statistics

```sql
-- Overall system data summary
SELECT COUNT(*) as total_records,
       COUNT(DISTINCT name) as total_tags,
       AVG(value) as system_avg,
       MIN(value) as system_min,
       MAX(value) as system_max
FROM iot_sensors;
```

#### 4.3 Value Range Distribution
**Purpose**: Check data distribution by value range
**Keywords**: CASE WHEN classification

```sql
-- Data distribution by value range
SELECT 
    CASE 
        WHEN value < 10 THEN 'LOW (< 10)'
        WHEN value < 20 THEN 'MEDIUM (10-20)'
        WHEN value < 30 THEN 'HIGH (20-30)'
        ELSE 'VERY HIGH (≥ 30)'
    END as value_range,
    COUNT(*) as count,
    AVG(value) as avg_in_range
FROM iot_sensors
GROUP BY 
    CASE 
        WHEN value < 10 THEN 'LOW (< 10)'
        WHEN value < 20 THEN 'MEDIUM (10-20)'
        WHEN value < 30 THEN 'HIGH (20-30)'
        ELSE 'VERY HIGH (≥ 30)'
    END
ORDER BY avg_in_range;
```

#### 4.4 Latest Data by Tag
**Purpose**: Most recent data for each tag
**Keywords**: MAX time query

```sql
-- Latest data time for each tag
SELECT name,
       MAX(time) as latest_time,
       COUNT(*) as total_records
FROM iot_sensors
GROUP BY name
ORDER BY latest_time DESC;
```

#### 4.5 High Value Tags
**Purpose**: Identify tags with high average values
**Keywords**: HAVING condition

```sql
-- Tags with average value exceeding threshold
SELECT name,
       COUNT(*) as record_count,
       AVG(value) as avg_value,
       MAX(value) as max_value
FROM iot_sensors
GROUP BY name
HAVING AVG(value) > 15.0
ORDER BY avg_value DESC;
```

#### 4.6 Data Collection Activity
**Purpose**: Analyze data collection activity by tag
**Keywords**: COUNT-based classification

```sql
-- Tag classification by data collection volume
SELECT name,
       COUNT(*) as record_count,
       CASE 
           WHEN COUNT(*) < 2 THEN 'LOW_ACTIVITY'
           WHEN COUNT(*) < 10 THEN 'MEDIUM_ACTIVITY'
           ELSE 'HIGH_ACTIVITY'
       END as activity_level
FROM iot_sensors
GROUP BY name
ORDER BY record_count DESC;
```

#### 4.7 Value Volatility Analysis
**Purpose**: Analyze value volatility by tag
**Keywords**: Volatility calculation

```sql
-- Value volatility by tag (range-based)
SELECT name,
       COUNT(*) as sample_size,
       MIN(value) as min_val,
       MAX(value) as max_val,
       MAX(value) - MIN(value) as value_range,
       CASE 
           WHEN MAX(value) - MIN(value) > 10 THEN 'HIGH_VOLATILITY'
           WHEN MAX(value) - MIN(value) > 5 THEN 'MEDIUM_VOLATILITY'
           ELSE 'LOW_VOLATILITY'
       END as volatility_level
FROM iot_sensors
GROUP BY name
ORDER BY value_range DESC;
```

#### 4.8 Threshold Violation Analysis
**Purpose**: Analyze data exceeding set thresholds
**Keywords**: Conditional COUNT

```sql
-- Threshold violation data analysis
SELECT name,
       COUNT(*) as total_count,
       SUM(CASE WHEN value > 25.0 THEN 1 ELSE 0 END) as over_threshold,
       SUM(CASE WHEN value < 5.0 THEN 1 ELSE 0 END) as under_threshold,
       AVG(value) as avg_value
FROM iot_sensors
GROUP BY name
ORDER BY name;
```

#### 4.9 Data Quality Analysis
**Purpose**: Data quality analysis by tag
**Keywords**: NULL value analysis

```sql
-- Data quality analysis
SELECT name,
       COUNT(*) as total_records,
       COUNT(value) as valid_values,
       COUNT(*) - COUNT(value) as null_values,
       CASE 
           WHEN COUNT(value) = COUNT(*) THEN 'COMPLETE'
           WHEN COUNT(value) > COUNT(*) * 0.9 THEN 'GOOD'
           ELSE 'INCOMPLETE'
       END as data_quality
FROM iot_sensors
GROUP BY name
ORDER BY data_quality, name;
```

---

### 5. Table Management - 10 Examples

#### 5.1 System Tables Query
**Purpose**: Check list of created tables
**Keywords**: M$SYS_TABLES

```sql
-- Query all table list
SELECT name FROM M$SYS_TABLES ORDER BY name;
```

#### 5.2 Specific Pattern Table Search
**Purpose**: Find tables with specific name pattern
**Keywords**: LIKE pattern

```sql
-- Search sensor-related tables
SELECT name FROM M$SYS_TABLES WHERE name LIKE '%SENSOR%' ORDER BY name;
```

#### 5.3 Conditional Table Creation
**Purpose**: Prevent duplicate creation
**Keywords**: IF NOT EXISTS

```sql
-- Create only if not exists
CREATE TAG TABLE IF NOT EXISTS backup_sensor (
    name VARCHAR(80) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
);
```

#### 5.4 Metadata Table Query
**Purpose**: Check metadata of TAG tables
**Keywords**: _META table

```sql
-- Check metadata table contents
SELECT * FROM _ENHANCED_SENSOR_META;
```

#### 5.5 Table Components Check
**Purpose**: Check internal tables of TAG table
**Keywords**: System table pattern

```sql
-- Check all components of specific table
SELECT name FROM M$SYS_TABLES 
WHERE name LIKE '_IOT_SENSORS_%' 
ORDER BY name;
```

#### 5.6 Rollup Tables Check
**Purpose**: Check automatically generated Rollup tables
**Keywords**: ROLLUP tables

```sql
-- Check Rollup table list
SELECT name FROM M$SYS_TABLES 
WHERE name LIKE '%_ROLLUP_%' 
ORDER BY name;
```

#### 5.7 Single Table Drop
**Purpose**: Remove unnecessary table
**Keywords**: DROP TABLE

```sql
-- Drop single table
DROP TABLE backup_sensor;
```

#### 5.8 Drop Including Related Tables
**Purpose**: Drop all including Rollup tables
**Keywords**: CASCADE

```sql
-- Drop including Rollup tables
DROP TABLE test_table CASCADE;
```

#### 5.9 Check Existence Before Drop
**Purpose**: Safe table deletion
**Keywords**: Check existence before drop

```sql
-- Check table existence first
SELECT COUNT(*) as table_exists 
FROM M$SYS_TABLES 
WHERE name = 'TEMPORARY_DATA';

-- Drop only if exists (manual verification required)
-- DROP TABLE temporary_data;
```

#### 5.10 Table Existence Check
**Purpose**: Verify table existence
**Keywords**: Existence check

```sql
-- Check specific table existence
SELECT COUNT(*) as table_exists 
FROM M$SYS_TABLES 
WHERE name = 'IOT_SENSORS';
```

---

### 6. Data Analysis - 10 Examples

#### 6.1 Data Distribution by Tag
**Purpose**: Check data distribution for each tag
**Keywords**: Analysis by tag

```sql
-- Data distribution analysis by tag
SELECT name, 
       COUNT(*) as record_count,
       MIN(time) as first_record,
       MAX(time) as last_record
FROM sensor_data 
GROUP BY name 
ORDER BY record_count DESC;
```

#### 6.2 Overall System Summary
**Purpose**: System-wide data status
**Keywords**: Overall summary

```sql
-- Overall system data summary
SELECT 
    COUNT(*) as total_records,
    COUNT(DISTINCT name) as total_sensors,
    MIN(time) as earliest_data,
    MAX(time) as latest_data,
    AVG(value) as system_avg_value
FROM sensor_data;
```

#### 6.3 Value Range Analysis
**Purpose**: Value range analysis for each tag
**Keywords**: Value range statistics

```sql
-- Value range analysis by tag
SELECT name,
       COUNT(*) as count,
       MIN(value) as min_val,
       MAX(value) as max_val,
       AVG(value) as avg_val,
       MAX(value) - MIN(value) as value_range
FROM sensor_data 
GROUP BY name 
ORDER BY value_range DESC;
```

#### 6.4 Threshold Violation Analysis
**Purpose**: Analyze data exceeding set thresholds
**Keywords**: Threshold analysis

```sql
-- Threshold violation data analysis
SELECT name,
       COUNT(CASE WHEN value > 30.0 THEN 1 END) as over_30,
       COUNT(CASE WHEN value < 10.0 THEN 1 END) as under_10,
       COUNT(*) as total,
       COUNT(CASE WHEN value > 30.0 THEN 1 END) * 100 / COUNT(*) as over_percentage
FROM sensor_data 
GROUP BY name
ORDER BY over_percentage DESC;
```

#### 6.5 Data Quality Check
**Purpose**: Check NULL values or validity
**Keywords**: Data quality

```sql
-- Data quality check
SELECT name,
       COUNT(*) as total_records,
       COUNT(value) as valid_values,
       COUNT(*) - COUNT(value) as null_values
FROM sensor_data 
GROUP BY name
ORDER BY null_values DESC;
```

#### 6.6 Time-Based Basic Analysis
**Purpose**: Data analysis for fixed time range
**Keywords**: Time range analysis

```sql
-- Data analysis for specific period
SELECT name, 
       COUNT(*) as record_count,
       AVG(value) as avg_value
FROM sensor_data 
WHERE time >= '2025-08-12 00:00:00'
AND time < '2025-08-13 00:00:00'
GROUP BY name
ORDER BY record_count DESC;
```

#### 6.7 Value Distribution Range Analysis
**Purpose**: Check distribution by value ranges
**Keywords**: Range-based analysis

```sql
-- Distribution analysis by value ranges
SELECT name,
       SUM(CASE WHEN value < 10 THEN 1 ELSE 0 END) as low_range,
       SUM(CASE WHEN value >= 10 AND value < 20 THEN 1 ELSE 0 END) as medium_range,
       SUM(CASE WHEN value >= 20 THEN 1 ELSE 0 END) as high_range,
       COUNT(*) as total
FROM sensor_data 
GROUP BY name
ORDER BY name;
```

#### 6.8 Extreme Value Analysis
**Purpose**: Check extreme value data
**Keywords**: Extreme value analysis

```sql
-- Top 10 highest values
SELECT name, time, value
FROM sensor_data 
ORDER BY value DESC
LIMIT 10;
```

#### 6.9 Latest Status by Tag
**Purpose**: Check current status of each tag
**Keywords**: Latest status

```sql
-- Latest data status by tag
SELECT name,
       COUNT(*) as total_records,
       MAX(time) as last_update,
       AVG(value) as current_avg,
       MAX(value) as current_max
FROM sensor_data 
GROUP BY name
ORDER BY last_update DESC;
```

#### 6.10 Performance Metrics Summary
**Purpose**: Overall sensor system performance metrics
**Keywords**: Performance metrics

```sql
-- Sensor system performance metrics
SELECT 
    COUNT(DISTINCT name) as active_sensors,
    COUNT(*) as total_data_points,
    AVG(value) as system_average,
    MIN(value) as system_minimum,
    MAX(value) as system_maximum,
    MAX(value) - MIN(value) as total_range
FROM sensor_data 
WHERE time >= '2025-08-01';
```

---

### 7. Backup & Restore - 10 Examples

#### 7.1 Pre-Backup Data Check
**Purpose**: Verify backup target data
**Keywords**: Pre-backup verification

```sql
-- Check backup target table size
SELECT COUNT(*) as total_records,
       MIN(time) as earliest,
       MAX(time) as latest
FROM sensor_data;
```

#### 7.2 Record Count by Table
**Purpose**: Check data volume of tables to backup
**Keywords**: Backup target selection

```sql
-- Backup target table list and sizes
SELECT name FROM M$SYS_TABLES 
WHERE name NOT LIKE '_%' 
AND name IN ('SENSOR_DATA', 'IOT_SENSORS', 'ENHANCED_SENSOR')
ORDER BY name;
```

#### 7.3 Specific Period Data Check
**Purpose**: Check data range for partial backup
**Keywords**: Time range check

```sql
-- Check specific period data
SELECT name,
       COUNT(*) as record_count,
       MIN(time) as start_time,
       MAX(time) as end_time
FROM sensor_data 
WHERE time >= '2025-08-01' AND time < '2025-08-13'
GROUP BY name
ORDER BY name;
```

#### 7.4 Important Metadata Backup Check
**Purpose**: Check metadata table status
**Keywords**: Metadata check

```sql
-- Check metadata tables
SELECT name FROM M$SYS_TABLES 
WHERE name LIKE '%_META' 
ORDER BY name;
```

#### 7.5 System Tables List
**Purpose**: Check system components
**Keywords**: System status

```sql
-- Overall system tables status
SELECT name FROM M$SYS_TABLES 
ORDER BY name;
```

#### 7.6 Pre-Backup Data Integrity Check
**Purpose**: Check data integrity before backup
**Keywords**: Integrity check

```sql
-- Data integrity check
SELECT name,
       COUNT(*) as total_records,
       COUNT(value) as valid_values,
       COUNT(*) - COUNT(value) as null_values
FROM sensor_data 
GROUP BY name
ORDER BY name;
```

#### 7.7 Recent Activity Tables Check
**Purpose**: Identify active tables
**Keywords**: Active tables

```sql
-- Check tables with recent activity
SELECT name,
       COUNT(*) as recent_records,
       MAX(time) as last_activity
FROM sensor_data 
WHERE time >= '2025-08-01'
GROUP BY name
ORDER BY last_activity DESC;
```

#### 7.8 Post-Backup Verification Preparation
**Purpose**: Baseline data for post-backup comparison
**Keywords**: Verification baseline

```sql
-- Baseline data for backup verification
SELECT 
    COUNT(*) as total_count,
    SUM(CASE WHEN name = 'SENSOR_01' THEN 1 ELSE 0 END) as sensor01_count,
    SUM(CASE WHEN name = 'TEMP_A' THEN 1 ELSE 0 END) as temp_a_count
FROM sensor_data;
```

#### 7.9 Table Dependency Check
**Purpose**: Check related tables
**Keywords**: Dependency analysis

```sql
-- Check related table structure
SELECT name FROM M$SYS_TABLES 
WHERE name LIKE 'SENSOR_DATA%' OR name LIKE '_SENSOR_DATA_%'
ORDER BY name;
```

#### 7.10 Post-Backup Verification
**Purpose**: Check backup success
**Keywords**: Backup verification

```sql
-- Check current status after backup completion
SELECT 
    COUNT(DISTINCT name) as unique_tags,
    COUNT(*) as total_records,
    MIN(time) as earliest_time,
    MAX(time) as latest_time
FROM sensor_data;
```

---

### 8. Monitoring & Diagnostics - 10 Examples

#### 8.1 Real-time Data Check
**Purpose**: Check recent data collection status
**Keywords**: Real-time monitoring

```sql
-- Recent data collection status
SELECT name, COUNT(*) as recent_count,
       MAX(time) as last_update
FROM sensor_data 
WHERE time >= '2025-08-12 00:00:00'
GROUP BY name
ORDER BY recent_count DESC;
```

#### 8.2 Activity Status by Tag
**Purpose**: Check recent activity of each sensor
**Keywords**: Activity status

```sql
-- Recent activity time by tag
SELECT name,
       MAX(time) as last_activity,
       COUNT(*) as total_records
FROM sensor_data 
GROUP BY name
ORDER BY last_activity DESC;
```

#### 8.3 Data Collection Status
**Purpose**: Overall system data collection status
**Keywords**: Collection status

```sql
-- Overall data collection status
SELECT 
    COUNT(DISTINCT name) as active_sensors,
    COUNT(*) as total_records_today,
    MIN(time) as earliest_today,
    MAX(time) as latest_today
FROM sensor_data 
WHERE time >= '2025-08-12 00:00:00';
```

#### 8.4 Anomaly Value Monitoring
**Purpose**: Real-time detection of abnormal values
**Keywords**: Anomaly detection

```sql
-- Anomaly detection
SELECT name, time, value,
       CASE 
           WHEN value > 100 THEN 'HIGH_ALARM'
           WHEN value < -10 THEN 'LOW_ALARM'
           WHEN value > 50 THEN 'HIGH_WARNING'
           ELSE 'NORMAL'
       END as alarm_level
FROM sensor_data 
WHERE (value > 50 OR value < -10)
ORDER BY time DESC
LIMIT 20;
```

#### 8.5 Sensor Responsiveness Check
**Purpose**: Check data collection frequency by sensor
**Keywords**: Responsiveness check

```sql
-- Data collection frequency by sensor
SELECT name,
       COUNT(*) as record_count,
       MIN(time) as first_record,
       MAX(time) as last_record
FROM sensor_data 
WHERE time >= '2025-08-01'
GROUP BY name
ORDER BY record_count DESC;
```

#### 8.6 System Performance Summary
**Purpose**: Check overall system performance
**Keywords**: Performance summary

```sql
-- System performance summary
SELECT 
    COUNT(DISTINCT name) as total_sensors,
    COUNT(*) as total_records,
    AVG(value) as avg_value,
    MIN(value) as min_value,
    MAX(value) as max_value
FROM sensor_data 
WHERE time >= '2025-08-01';
```

#### 8.7 Data Integrity Check
**Purpose**: Check duplicate or missing data
**Keywords**: Integrity check

```sql
-- Data quality check
SELECT name,
       COUNT(*) as total_records,
       COUNT(value) as valid_values,
       COUNT(*) - COUNT(value) as null_count,
       MIN(value) as min_val,
       MAX(value) as max_val
FROM sensor_data 
GROUP BY name
ORDER BY null_count DESC;
```

#### 8.8 Sensor Status Classification
**Purpose**: Classification by sensor status
**Keywords**: Status classification

```sql
-- Sensor status classification
SELECT name,
       COUNT(*) as record_count,
       MAX(time) as last_seen,
       CASE 
           WHEN COUNT(*) > 10 THEN 'ACTIVE'
           WHEN COUNT(*) > 1 THEN 'MODERATE'
           ELSE 'INACTIVE'
       END as activity_status
FROM sensor_data 
WHERE time >= '2025-08-01'
GROUP BY name
ORDER BY record_count DESC;
```

#### 8.9 Alarm Event Summary
**Purpose**: Alarm occurrence statistics
**Keywords**: Alarm statistics

```sql
-- Alarm event statistics
SELECT name,
       COUNT(*) as total_events,
       SUM(CASE WHEN value > 50 THEN 1 ELSE 0 END) as warning_events,
       SUM(CASE WHEN value > 100 THEN 1 ELSE 0 END) as critical_events,
       MAX(value) as max_value_seen
FROM sensor_data 
WHERE time >= '2025-08-01'
GROUP BY name
ORDER BY critical_events DESC, warning_events DESC;
```

#### 8.10 Sensor Health Check Summary
**Purpose**: Overall sensor system health status
**Keywords**: Health check

```sql
-- Sensor system health check summary
SELECT 
    name,
    COUNT(*) as records_count,
    MAX(time) as last_seen,
    AVG(value) as avg_value,
    CASE 
        WHEN COUNT(*) >= 5 THEN 'HEALTHY'
        WHEN COUNT(*) >= 2 THEN 'WARNING'
        ELSE 'CRITICAL'
    END as health_status
FROM sensor_data 
WHERE time >= '2025-08-01'
GROUP BY name
ORDER BY 
    CASE health_status 
        WHEN 'CRITICAL' THEN 1 
        WHEN 'WARNING' THEN 2 
        ELSE 3 
    END, name;
```

---

## Usage Example Scenarios

### Complete IoT Sensor Data Processing Guide
1. **Initial Setup**: Create sensor table with Rollup and statistics features (1.3, 1.6)
2. **Data Collection**: Batch insertion of real-time sensor data (2.3, 2.6)
3. **Real-time Monitoring**: Trend analysis with minute/hour aggregated data (4.2, 4.3)
4. **Anomaly Detection**: Automatic anomaly detection based on thresholds (6.4, 8.4)
5. **Performance Management**: Monitor data collection status and system performance (8.1, 8.10)
6. **Regular Backup**: Daily automatic backup and verification (7.1, 7.9)

### Equipment Monitoring System
1. **Equipment-specific Tables**: Automatic filtering of abnormal data with outlier removal feature (1.7, 1.8)
2. **Threshold Management**: Dynamic threshold setting through metadata (5.5, 5.6)
3. **Preventive Maintenance**: Early detection of equipment anomalies through data analysis (6.6, 6.9)
4. **Report Generation**: Efficient daily/monthly reports with Rollup data (4.10, 6.7)

### Data Archiving Strategy
1. **Retention Policy**: Partition-based data management (1.9)
2. **Backup Automation**: Scheduled backup and verification process (7.1-7.10)
3. **Data Quality**: Regular data integrity checks (6.5, 8.7)

---

## SQL Guide

The following section provides an overview of the core concepts and features of TAG tables.  
For comprehensive details and additional features, refer to the [DBMS References](https://docs.machbase.com/dbms/).