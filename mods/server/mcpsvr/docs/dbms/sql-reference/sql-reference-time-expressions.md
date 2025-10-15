# Relative Time Expressions

## Overview

Relative time expressions let you describe offsets from a known timestamp directly inside SQL statements. They are designed for operators who need to filter recent telemetry, schedule future jobs, or align time series windows without calling helper functions.

> **Note**: This feature is supported from Machbase version 8.0.50 or later.

## Quick Start

1. Find records from the last hour:
   ```sql
   SELECT * FROM sensor_log WHERE event_time > now - 1h;
   ```
2. Look ahead two days and six hours:
   ```sql
   SELECT * FROM maintenance_plan WHERE planned_at < now + 2d6h;
   ```
3. Combine segments for sub-second precision:
   ```sql
   SELECT to_char(now + 3s125ms10us4ns, 'YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn');
   ```

## Syntax Summary

- A literal is one or more `<number><unit>` segments written back-to-back with no spaces.
- Units are lowercase. Separate magnitudes by concatenation: `2h30m`.
- Prefix with `+` or `-`, or use arithmetic (`now - 90m`, `sample_time + 15s`).
- When a numeric literal lacks a unit, Machbase treats the value as nanoseconds.
- Relative literals evaluate to an `INTERVAL`. Adding or subtracting them to/from a `DATETIME` produces another `DATETIME`.

## Supported Units

| Suffix | Meaning          | Example | Equivalent duration |
|--------|------------------|---------|---------------------|
| `ns`   | nanoseconds      | `500ns` | 500 nanoseconds     |
| `us`   | microseconds     | `20us`  | 0.00002 seconds     |
| `ms`   | milliseconds     | `15ms`  | 0.015 seconds       |
| `s`    | seconds          | `45s`   | 45 seconds          |
| `m`    | minutes          | `30m`   | 30 minutes          |
| `h`    | hours            | `12h`   | 12 hours            |
| `d`    | days             | `7d`    | 7 days              |
| `w`    | weeks            | `2w`    | 14 days             |

> **Note**: Months and years are not supported because their length is not constant. Using unsupported suffixes (for example, `1y`, `1mo`) raises `ERR_QP_TIME_FORMAT`.

## Building Compound Literals

- Write the largest unit first to improve readability: `5d4h30m`.
- Omit segments that are zero; `4h15m` is preferred over `4h15m0s`.
- Segment order can vary, but consistent ordering reduces mistakes. `1h30m` and `30m1h` evaluate to the same interval.
- For long intervals, consider grouping with underscores for readability when possible inside SQL string literals: `'1d12h_30m'` is not allowed as a literal, but you can add a comment (`/* +1d12h30m */`) or store the literal in a SQL variable for documentation.

## Usage Patterns

### Filtering Windows of Time

```sql
-- Records in the most recent 24 hours
SELECT *
  FROM rtrollup
 WHERE time BETWEEN now - 1d AND now;

-- Alerts raised within the last 10 minutes
SELECT alert_id, level, occurred_at
  FROM alert_log
 WHERE occurred_at >= sysdate - 10m;
```

### Scheduling Future Operations

```sql
-- Tasks to execute in the next business day plus two hours
SELECT job_id, scheduled_at
  FROM job_queue
 WHERE scheduled_at <= now + 1d2h;

-- Insert maintenance schedule 30 minutes from now
INSERT INTO device_schedule (device_id, maintenance_due)
VALUES ('device-001', now + 30m);
```

### Time-based Filtering and Joins

```sql
-- Select data within a specific time window
SELECT device_id, ts, value
  FROM metrics_stream
 WHERE ts BETWEEN now - 15m AND now
   AND device_id = 'sensor-01';

-- Join two sources using relative offsets
SELECT a.ts, a.value AS raw_value, b.value AS calibrated
  FROM raw_metrics a
  JOIN calibration b
    ON b.ts BETWEEN a.ts - 500ms AND a.ts + 500ms;
```

### String Literals and Casting

```sql
SELECT to_char('2024-05-01' + 3d, 'YYYY-MM-DD');           -- 2024-05-04
SELECT to_char('2024-05-01 08:00:00' - 4h15m,
               'YYYY-MM-DD HH24:MI:SS');                   -- 2024-05-01 03:45:00
SELECT to_char('2024-05-01'::datetime + 2h30m45s250ms,
               'YYYY-MM-DD HH24:MI:SS mmm');               -- 2024-05-01 02:30:45 250
```

### Mixing with Plain Numbers

```sql
-- Adds one exact second because numeric literal defaults to nanoseconds
SELECT event_time + 1000000000 AS event_time_plus_1s
  FROM events;

-- Subtracts 250 nanoseconds
SELECT event_time - 250 AS event_time_minus_250ns
  FROM events;
```

## Behaviour and Limitations

- Precision is capped at nanoseconds. Values beyond 64-bit range overflow.
- Arithmetic follows standard precedence: parentheses first, multiplication/division, then addition/subtraction. Use parentheses when chaining multiple operations.
- Comparisons involving intervals use the resulting `DATETIME` values. Intervals alone cannot appear in `ORDER BY` clauses.
- The feature is available in standard edition builds. Check release notes for availability in older versions.

## Error Handling

| Scenario                               | Error Code                | Resolution                                   |
|----------------------------------------|---------------------------|----------------------------------------------|
| Unsupported suffix (`1y`, `5mo`)       | `ERR_QP_TIME_FORMAT`      | Replace with supported units (`30d`, etc.).  |
| Missing unit (`now + 10`)              | Interpreted as nanoseconds | Add explicit suffix if minutes or seconds intended. |
| Overflowing literal (`1000000d`)       | `ERR_OVERFLOW_INTERVAL`   | Reduce magnitude or break the logic into loops. |
| Non-numeric characters (`1h3xm`)       | `ERR_QP_TIME_FORMAT`      | Fix the typo (`1h3m`).                       |

## Best Practices

- Standardize on lowercase suffixes across your team scripts.
- Store frequently used offsets in configuration tables for reuse and auditing.
- Comment complex expressions to aid future maintenance (`-- subtract 1 business week`).
- Validate application inputs when constructing literals dynamically to avoid injection of unsupported suffixes.

## Troubleshooting Checklist

- **Unexpected range**: Print both `now` and the computed boundary to verify the offset.
- **Wrong unit**: Remember plain numbers are nanoseconds; append `s`, `m`, or `h` for human-readable units.
- **Function interaction**: When combining with window functions or aggregate filters, evaluate the literal in a subquery to ensure it resolves once per statement.

## Frequently Asked Questions

- **Can I chain relative literals with `ADD_TIME`?** Yes. `ADD_TIME(now, '0/0/0 0:15:0') + 30s` combines function-based and literal offsets.
- **Can I store literals in variables?** Relative time literals are evaluated at query execution time and cannot be stored in variables. However, you can use them directly in repeated queries.
- **How do I subtract business days?** Relative literals operate on absolute durations. Implement business-day rules in application logic or reference calendar tables.

## Reference Cheat Sheet

```
Pattern          Meaning
--------------  --------------------------------------------
now - 5m        Timestamp exactly five minutes ago
sysdate + 1d    Tomorrow (24 hours after system timestamp)
col_ts + 90s    Column value shifted by 90 seconds
'2024-01-01' + 2w  Adds 14 days to the literal date
value + 250     Adds 250 nanoseconds to `value`
```

Relative time expressions offer precise, readable temporal arithmetic without helper functions. Use them wherever expressions are supported—`WHERE` clauses, computed columns, projections, or procedural code—to keep your Machbase analytics concise and maintainable.
