# Machbase SQL Reference

## Online Full Manuals (Markdown)

- In agent profile, use `agent.sqlref.list()`, `agent.sqlref.fetch(name, options)`, and `agent.sqlref.fetchAll(options)` to load current online markdown manuals.
- Use `agent.sqlref.index(options)` to fetch `index.md` directly.
- Use `maxBytes` to limit markdown payload size and `omitMarkdown: true` when only metadata is needed.

### SQL Reference Catalog

- `datatypes`: Machbase SQL datatype reference. https://docs.machbase.com/dbms/sql-reference/index.md
- `ddl`: Machbase SQL DDL reference. https://docs.machbase.com/dbms/sql-reference/ddl.md
- `dml`: Machbase SQL DML reference. https://docs.machbase.com/dbms/sql-reference/dml.md
- `math-functions`: Machbase SQL math function reference. https://docs.machbase.com/dbms/sql-reference/math-functions.md
- `functions`: Machbase SQL function reference. https://docs.machbase.com/dbms/sql-reference/functions.md

## SQL execution policy

- Default to `SELECT` queries for inspection and verification.
- Do not run DDL/DML unless the user explicitly requests data/schema changes.
- Always use bounded queries (`LIMIT`, or clear time range predicates for TAG data).
- For system catalogs, prefer explicit column lists over `SELECT *` where practical.
- For analysis/report requests, emit a bounded `jsh-sql` runnable fence first so the harness can execute evidence collection before you summarize findings.
- For analysis/report requests, the first non-empty output must start with a runnable `jsh-sql` fence.
- Do not present SQL as plain markdown examples when you intend the harness to run it.
- Do not place plain SQL examples before the first runnable fence.
- For time-series analysis, prefer a short verification query first, then follow with derived aggregation queries.
- When a report is requested, the final narrative must cite the concrete values returned by those verification or aggregation queries.
- SQL keywords and identifiers remain standard SQL/English forms.
- SQL comments and surrounding explanatory text for execution workflow should follow the user's prompt language.
- If prompt language is unclear or mixed, default those comments and explanations to English.
- When harness execution is available, do not ask users to run SQL manually or paste query results.

## Table Types

| Type | Description |
|------|-------------|
| `TAG` | Time-series tag table. Stores sensor/metric data. Primary key = tag name. |
| `LOG` | Append-only log table. Fast sequential ingestion. |
| `LOOKUP` | Key-value lookup table. Small dimension tables. |
| `VOLATILE` | In-memory volatile table. Not persisted. |

## Minimal templates

```sql
CREATE TAG TABLE tag_table (
  name VARCHAR(40) PRIMARY KEY,
  time DATETIME BASETIME,
  value DOUBLE SUMMARIZED
);

CREATE TABLE log_table (
  time DATETIME,
  level VARCHAR(10),
  message VARCHAR(4096)
);
```

## Common query patterns

```sql
SELECT time, value
FROM tag_table
WHERE name = 'sensor_001'
  AND time BETWEEN TO_DATE('2024-01-01') AND TO_DATE('2024-01-02')
ORDER BY time DESC
LIMIT 100;

SELECT DISTINCT name FROM tag_table;

SELECT DATE_TRUNC('hour', time) AS bucket, AVG(value) AS avg_val
FROM tag_table
WHERE name = 'sensor_001'
  AND time BETWEEN TO_DATE('2024-01-01') AND TO_DATE('2024-02-01')
GROUP BY bucket
ORDER BY bucket;
```

## Date/Time quick reference

```sql
-- Literals
TO_DATE('2024-01-15 12:00:00')         -- datetime literal
TO_DATE('2024-01-15', 'YYYY-MM-DD')    -- with format

-- Arithmetic and truncation
time + INTERVAL 1 HOUR
time - INTERVAL 30 MINUTE
DATE_TRUNC('second', time)
DATE_TRUNC('minute', time)
DATE_TRUNC('hour',   time)
DATE_TRUNC('day',    time)
DATE_TRUNC('month',  time)

-- Extraction and now
EXTRACT(HOUR FROM time)
EXTRACT(DAY  FROM time)
NOW()
```

## Aggregate quick reference

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

## String quick reference

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

## Useful patterns

```sql
SELECT NAME, TYPE, FLAG
FROM M$SYS_TABLES
WHERE DATABASE_ID = -1
ORDER BY NAME;

SELECT COUNT(*)
FROM tag_table
WHERE name = 'sensor_001'
  AND time BETWEEN TO_DATE('2024-01-01') AND NOW();
```

## Notes

- `DATETIME` values are stored in nanosecond precision UTC.
- TAG tables require a `PRIMARY KEY` column (tag name) and a `BASETIME` column (timestamp).
- Use `SUMMARIZED` on value columns to enable pre-computed statistics.
- `LIMIT` is always recommended to avoid large result sets.
