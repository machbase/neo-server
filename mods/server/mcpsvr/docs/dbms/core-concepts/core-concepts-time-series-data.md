# Understanding Time-Series Data

Learn what makes time-series data unique and why Machbase is specifically designed to handle it efficiently. This guide explains the fundamental concepts behind time-series databases.

## What is Time-Series Data?

Time-series data is a sequence of data points indexed by time. Each record has a timestamp and one or more values:

```
(timestamp, value1, value2, ...)
```

### Common Examples

**IoT and Sensors**:
- Temperature readings every second
- GPS coordinates from vehicles
- Smart meter electricity usage
- Manufacturing equipment telemetry

**Application Monitoring**:
- Server CPU/memory metrics
- Application logs
- HTTP request logs
- Database query performance

**Business Data**:
- Stock market ticks
- Sales transactions
- Website clickstreams
- Mobile app events

## Characteristics of Time-Series Workloads

### 1. Write-Heavy

Time-series systems are dominated by writes:
- **Millions of writes per second** (sensors, logs, events)
- **Few updates or deletes** (historical data rarely changes)
- **Append-only pattern** (always adding new data)

Traditional databases struggle because:
- Row-level locking slows writes
- Complex UPDATE logic not needed
- Transaction overhead wasted

**Machbase solution**: Append-only architecture with no row locking.

### 2. Time-Based Queries

Most queries involve time ranges:

```sql
-- Last hour's data
SELECT * FROM logs DURATION 1 HOUR;

-- Yesterday's statistics
SELECT AVG(temperature) FROM sensors
WHERE time BETWEEN '2025-10-09 00:00:00' AND '2025-10-10 00:00:00';
```

Traditional databases struggle because:
- Generic indexes not optimized for time
- Full table scans for time ranges
- No time-aware partitioning

**Machbase solution**: Time-based partitioning and indexing built-in.

### 3. Recent Data Focus

Users care most about recent data:
- Last 24 hours for monitoring
- Last week for trending
- Older data for compliance/archival

Traditional databases struggle because:
- All data treated equally
- No automatic aging policies
- Manual archival processes

**Machbase solution**: DURATION keyword, automatic partitioning, efficient time-based deletion.

### 4. High Compression Potential

Time-series data compresses extremely well:
- Sequential timestamps
- Repeated patterns
- Similar values

Traditional databases struggle because:
- Row-oriented storage
- Generic compression
- 2-5x compression typical

**Machbase solution**: Columnar storage with specialized compression (10-100x ratios).

### 5. Bulk Aggregations

Common analytical queries:
- MIN, MAX, AVG over time windows
- Grouping by time intervals
- Statistical summaries

Traditional databases struggle because:
- Aggregations computed on-demand
- No pre-computed statistics
- Slow for large datasets

**Machbase solution**: Automatic rollup tables with pre-computed statistics.

## Why Traditional Databases Fail

### Problem 1: Row-Oriented Storage

Traditional databases store entire rows together:

```
Row 1: [timestamp1, sensor_id, temp, humidity]
Row 2: [timestamp2, sensor_id, temp, humidity]
Row 3: [timestamp3, sensor_id, temp, humidity]
```

**Issue**: When you query "AVG(temperature)", database reads ALL columns, not just temperature.

**Machbase approach**: Columnar storage reads only needed columns.

### Problem 2: B-Tree Indexes

Traditional databases use B-Tree indexes:
- Good for random access
- Expensive for sequential writes
- Not optimized for time-ordered data

**Machbase approach**: LSM indexes and time-based partitioning.

### Problem 3: ACID Overhead

Traditional databases provide full ACID guarantees:
- Row-level locking
- Transaction logs
- Rollback support

**Not needed for time-series**:
- Historical data never changes
- No concurrent updates to same row
- Wasted overhead

**Machbase approach**: Simplified architecture for append-only data.

### Problem 4: No Time Awareness

Traditional databases don't understand time:
- No automatic time-based partitioning
- No built-in retention policies
- No time-optimized queries

**Machbase approach**: Time is a first-class concept.

## Machbase Design Principles

### 1. Append-Only Architecture

Data is only added, never modified:

```sql
-- Allowed: Adding new data
INSERT INTO sensors VALUES ('sensor01', NOW, 25.3);

-- Not allowed on Tag/Log tables: Modifying old data
UPDATE sensors SET temperature = 26.0 WHERE id = 123;  -- ✗
```

**Benefits**:
- No row locking
- Ultra-fast writes (millions/sec)
- Data integrity (can't alter history)

### 2. Time-Based Partitioning

Data is automatically partitioned by time:

```
Partition 1: 2025-10-01 to 2025-10-07
Partition 2: 2025-10-08 to 2025-10-14
Partition 3: 2025-10-15 to 2025-10-21
```

**Benefits**:
- Queries only scan relevant partitions
- Easy data retention (drop old partitions)
- Optimal compression per partition

### 3. Columnar Compression

Each column stored separately:

```
Timestamps:    [100, 101, 102, 103, 104, ...]
Sensor IDs:    [s01, s01, s01, s02, s02, ...]
Temperatures:  [22.5, 22.7, 22.6, 21.3, 21.5, ...]
```

**Benefits**:
- High compression (similar values)
- Read only needed columns
- Faster analytical queries

### 4. Write-Optimized Indexes

LSM (Log-Structured Merge) indexes:
- Optimized for sequential writes
- Batch writes to memory
- Periodic merging to disk

**Benefits**:
- Millions of writes per second
- No write amplification
- Consistent performance

### 5. Automatic Statistics (Rollup)

Tag tables generate statistics automatically:

```sql
-- Raw data: millions of rows
INSERT INTO sensors VALUES ('sensor01', NOW, 25.3);

-- Automatic rollup: per-second, per-minute, per-hour
SELECT * FROM sensors WHERE rollup = hour;
-- Returns: MIN, MAX, AVG, SUM, COUNT, SUMSQ
```

**Benefits**:
- Instant analytics
- No manual aggregation
- Reduced query time

## Time-Series vs Traditional Databases

| Feature | Traditional DB | Machbase |
|---------|---------------|----------|
| **Primary Use** | Transactions | Analytics |
| **Write Pattern** | Random | Sequential |
| **UPDATE Support** | Full | Limited |
| **Indexing** | B-Tree | LSM + Partitioned |
| **Storage** | Row-oriented | Columnar |
| **Compression** | 2-5x | 10-100x |
| **Write Speed** | 1,000s/sec | Millions/sec |
| **Time Queries** | Generic | Optimized |
| **Data Retention** | Manual | Automatic |

## Time-Series Data Patterns

### Pattern 1: High-Frequency Sensors

```
Sensor ID: sensor01
Frequency: 10 readings/second
Data Volume: 864,000 readings/day
```

**Best for**: Tag table with SUMMARIZED columns

```sql
CREATE TAGDATA TABLE sensors (
    sensor_id VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
);
```

### Pattern 2: Event Streams

```
Type: Application logs
Frequency: Variable (bursty)
Data Volume: Millions/day
Schema: Flexible (many columns)
```

**Best for**: Log table

```sql
CREATE TABLE app_logs (
    level VARCHAR(10),
    message VARCHAR(2000),
    user_id INTEGER
);
```

### Pattern 3: Real-Time State

```
Type: Live dashboard data
Frequency: Constant updates
Data Volume: Small (100s of rows)
Persistence: Not required
```

**Best for**: Volatile table

```sql
CREATE VOLATILE TABLE live_status (
    device_id INTEGER PRIMARY KEY,
    status VARCHAR(20),
    last_updated DATETIME
);
```

### Pattern 4: Dimension Data

```
Type: Device metadata
Frequency: Rare updates
Data Volume: Small (1000s of rows)
Persistence: Required
```

**Best for**: Lookup table

```sql
CREATE LOOKUP TABLE devices (
    device_id INTEGER,
    name VARCHAR(100),
    location VARCHAR(200)
);
```

## Common Time-Series Challenges

### Challenge 1: Data Volume

**Problem**: Sensors generate millions of readings/day

**Machbase solution**:
- High-speed ingestion (APPEND protocol)
- Automatic compression (10-100x)
- Time-based retention

```sql
-- Keep only 30 days
DELETE FROM sensors EXCEPT 30 DAYS;
```

### Challenge 2: Query Performance

**Problem**: Analyzing millions of rows is slow

**Machbase solution**:
- Automatic rollup statistics
- Time-based partitioning
- Columnar storage

```sql
-- Fast: Query pre-aggregated data
SELECT * FROM sensors WHERE rollup = hour;
```

### Challenge 3: Late-Arriving Data

**Problem**: Data arrives out of order

**Machbase solution**:
- LSM indexes handle out-of-order writes
- Background merge processes
- Consistent query results

### Challenge 4: Multiple Time Zones

**Problem**: Global deployments with different time zones

**Machbase solution**:
- Store in UTC, display in local time
- Timezone conversion functions
- Client-side timezone setting

```bash
machsql -z +0900  # Korea timezone
```

## Best Practices

### 1. Use Appropriate Table Types

Match data characteristics to table type:
- Regular sensor readings → Tag table
- Variable event streams → Log table
- Real-time updates → Volatile table
- Reference data → Lookup table

### 2. Implement Data Retention

Don't keep data forever:

```sql
-- Daily cleanup job
DELETE FROM logs EXCEPT 90 DAYS;
```

### 3. Use DURATION for Time Queries

Optimized syntax for time ranges:

```sql
-- Good
SELECT * FROM logs DURATION 1 HOUR;

-- Less optimal
SELECT * FROM logs WHERE _arrival_time >= NOW - INTERVAL '1' HOUR;
```

### 4. Batch Writes When Possible

Use APPEND protocol for bulk inserts:
- Higher throughput
- Better compression
- Reduced overhead

### 5. Query Rollup, Not Raw Data

For analytics, use pre-aggregated data:

```sql
-- Fast: Rollup
SELECT AVG(avg_temperature) FROM sensors WHERE rollup = hour;

-- Slow: Raw data
SELECT AVG(temperature) FROM sensors;
```

## Next Steps

Now that you understand time-series data:

1. **Read**: [Table Types Overview](../table-types-overview/) - Choose the right table
2. **Learn**: [Indexing and Performance](../indexing/) - Optimize queries
3. **Practice**: [Tutorials](../../tutorials/) - Hands-on exercises

## Key Takeaways

1. Time-series data is **write-heavy** and **time-focused**
2. Traditional databases **not optimized** for time-series
3. Machbase uses **append-only** architecture
4. **Columnar storage** enables high compression
5. **Time-based partitioning** optimizes queries
6. **Automatic rollup** provides instant analytics
7. Choose the **right table type** for your data pattern

---

Understanding time-series data fundamentals helps you design better Machbase solutions!
