# Machbase SQL Reference

## Online Full Manuals (Markdown)

- In agent profile, use `agent.sqlref.list()`, `agent.sqlref.fetch(name, options)`, and `agent.sqlref.fetchAll(options)` to load current online markdown manuals.
- Use `agent.sqlref.index(options)` to fetch `index.md` directly.
- Use `maxBytes` to limit markdown payload size and `omitMarkdown: true` when only metadata is needed.

### SQL Reference Catalog

- `datatypes`: Machbase SQL datatype reference. https://docs.machbase.com/dbms/sql-reference/index.md
- `ddl`: Machbase SQL DDL reference. https://docs.machbase.com/dbms/sql-reference/ddl.md
- `dml`: Machbase SQL DML reference. https://docs.machbase.com/dbms/sql-reference/ddl.md
- `math-functions`: Machbase SQL math function reference. https://docs.machbase.com/dbms/sql-reference/ddl.md
- `functions`: Machbase SQL function reference. https://docs.machbase.com/dbms/sql-reference/functions.md

## Table Types

| Type | Description |
|------|-------------|
| `TAG` | Time-series tag table. Stores sensor/metric data. Primary key = tag name. |
| `LOG` | Append-only log table. Fast sequential ingestion. |
| `LOOKUP` | Key-value lookup table. Small dimension tables. |
| `VOLATILE` | In-memory volatile table. Not persisted. |

## Creating Tables

```sql
-- TAG table (most common for sensor/IoT data)
CREATE TAG TABLE tag_table (
    name     VARCHAR(40) PRIMARY KEY,
    time     DATETIME    BASETIME,
    value    DOUBLE      SUMMARIZED
);

-- LOG table
CREATE TABLE log_table (
    time    DATETIME,
    level   VARCHAR(10),
    message VARCHAR(4096)
);
```

## Tag Table Queries

```sql
-- Query recent data for a specific tag
SELECT time, value FROM tag_table
WHERE name = 'sensor_001'
  AND time BETWEEN TO_DATE('2024-01-01') AND TO_DATE('2024-01-02')
ORDER BY time DESC
LIMIT 100;

-- List all tags in a TAG table
SELECT DISTINCT name FROM tag_table;

-- Aggregate by time bucket (downsampling)
SELECT
    DATE_TRUNC('hour', time) AS bucket,
    AVG(value) AS avg_val,
    MIN(value) AS min_val,
    MAX(value) AS max_val
FROM tag_table
WHERE name = 'sensor_001'
  AND time BETWEEN TO_DATE('2024-01-01') AND TO_DATE('2024-02-01')
GROUP BY bucket
ORDER BY bucket;
```

## Date/Time Functions

```sql
-- Literals
TO_DATE('2024-01-15 12:00:00')         -- datetime literal
TO_DATE('2024-01-15', 'YYYY-MM-DD')    -- with format

-- Arithmetic
time + INTERVAL 1 HOUR
time - INTERVAL 30 MINUTE
time + INTERVAL 7 DAY

-- Truncation (for grouping)
DATE_TRUNC('second', time)
DATE_TRUNC('minute', time)
DATE_TRUNC('hour',   time)
DATE_TRUNC('day',    time)
DATE_TRUNC('month',  time)

-- Extraction
EXTRACT(HOUR FROM time)
EXTRACT(DAY  FROM time)

-- Current time
NOW()
```

## Aggregate Functions

```sql
COUNT(*)            -- row count
SUM(value)          -- sum
AVG(value)          -- average
MIN(value)          -- minimum
MAX(value)          -- maximum
STDDEV(value)       -- standard deviation
VARIANCE(value)     -- variance
FIRST(value, time)  -- value at earliest time (TAG tables)
LAST(value, time)   -- value at latest time  (TAG tables)
```

## String Functions

```sql
UPPER(str)        LOWER(str)
TRIM(str)         LTRIM(str)       RTRIM(str)
SUBSTR(str, pos, len)
LENGTH(str)
CONCAT(str1, str2, ...)
INSTR(str, substr)
REPLACE(str, from, to)
TO_CHAR(expr, fmt)           -- format number or datetime as string
TO_NUMBER(str)               -- parse string as number
```

## Useful Patterns

```sql
-- Show all tables (equivalent with `show tables`)
SELECT NAME, TYPE, FLAG 
    FROM M$SYS_TABLES 
    WHERE DATABASE_ID = -1
    ORDER BY NAME;

-- Count rows in a time range
SELECT COUNT(*) FROM tag_table
WHERE name = 'sensor_001'
  AND time BETWEEN TO_DATE('2024-01-01') AND NOW();

-- Find tags with recent data (last 1 hour)
SELECT DISTINCT name FROM tag_table
WHERE time >= NOW() - INTERVAL 1 HOUR;

-- Statistical summary
SELECT
    name,
    COUNT(*)     AS samples,
    AVG(value)   AS avg,
    MIN(value)   AS min,
    MAX(value)   AS max,
    STDDEV(value) AS stddev
FROM tag_table
WHERE time BETWEEN TO_DATE('2024-01-01') AND TO_DATE('2024-02-01')
GROUP BY name
ORDER BY name;
```

## Notes

- `DATETIME` values are stored in nanosecond precision UTC.
- TAG tables require a `PRIMARY KEY` column (tag name) and a `BASETIME` column (timestamp).
- Use `SUMMARIZED` on value columns to enable pre-computed statistics.
- `LIMIT` is always recommended to avoid large result sets.
