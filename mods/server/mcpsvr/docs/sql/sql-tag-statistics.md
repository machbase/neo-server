# Machbase Neo SQL Tag Statistics

## Introduction

Querying large volumes of time-series data stored in Machbase TAG tables frequently involves retrieving statistical summaries associated with specific Tag Identifiers (Tag IDs). While direct queries on the TAG table using `NAME` (Tag ID) and `time` predicates are fundamental, computing aggregate statistics such as minimum/maximum values, counts, or time boundaries across extensive datasets for individual tags can incur significant computational overhead and latency.

To address this challenge and provide expedited access to essential per-tag statistics, Machbase offers an automated statistical aggregation feature centered around a dedicated system view. This mechanism pre-calculates and maintains key statistical metrics for each Tag ID, enabling substantially faster retrieval compared to direct computation on the raw TAG table data, akin to the performance benefits observed with Rollup tables but focused on per-tag summaries rather than time-based intervals.

## The `v$tag_table_name_stat` View

Upon the creation of a TAG table, Machbase automatically generates a corresponding system view named `v$tag_table_name_stat` (where `tag_table_name` is the name of the parent TAG table). The primary function of this view is to store and provide readily accessible statistical summaries aggregated on a per-Tag ID basis.

This view is populated and maintained internally by the Machbase engine, alleviating the need for users to manually configure or manage complex aggregation processes for these common statistical indicators.

## Enabling Statistics Collection

The population of the `v$tag_table_name_stat` view is contingent upon specific configurations during the TAG table definition:

1.  **`TAG_STAT_ENABLE` Property:** This table property controls the overall activation of the per-tag statistics collection mechanism. It defaults to `1` (enabled). If explicitly set to `0` during table creation (`CREATE TAG TABLE ... TAG_STAT_ENABLE=0`), the `v$tag_table_name_stat` view will not be populated, and no per-tag statistics will be maintained.
2.  **`SUMMARIZED` Keyword:** To collect value-based statistics (minimum value, maximum value, and their corresponding timestamps), the designated value column within the TAG table schema (conventionally the third column, representing the sensor reading or metric) **must** be defined with the `SUMMARIZED` keyword. If the `SUMMARIZED` keyword is omitted from the value column definition, only non-value-based statistics (row count, time boundaries, recent time) will be collected and stored in the view, while value-related fields will remain NULL.

**Syntax Example (Enabling Full Statistics):**

```sql
CREATE TAG TABLE device_metrics (
    name VARCHAR(80) PRIMARY KEY, -- Tag ID column
    time DATETIME BASETIME,        -- Timestamp column
    value DOUBLE SUMMARIZED        -- Value column with SUMMARIZED
)
TAG_STAT_ENABLE=1; -- Property (default, can be omitted)
```

## Available Statistics

The `v$tag_table_name_stat` view exposes the following pre-calculated statistical columns for each Tag ID present in the parent TAG table:

| Column Name       | Data Type | Description                                                                 | Requires `SUMMARIZED` |
| :---------------- | :-------- | :-------------------------------------------------------------------------- | :-------------------- |
| `NAME`            | VARCHAR   | The unique Tag Identifier (Tag ID).                                         | No                    |
| `ROW_COUNT`       | ULONG     | The total number of data points (rows) recorded for this Tag ID.            | No                    |
| `MIN_TIME`        | DATETIME  | The earliest timestamp recorded among all data points for this Tag ID.      | No                    |
| `MAX_TIME`        | DATETIME  | The latest timestamp recorded among all data points for this Tag ID.        | No                    |
| `MIN_VALUE`       | *matches* | The minimum value recorded in the `SUMMARIZED` column for this Tag ID.      | **Yes**               |
| `MIN_VALUE_TIME`  | DATETIME  | The timestamp corresponding to the first occurrence of `MIN_VALUE`.         | **Yes**               |
| `MAX_VALUE`       | *matches* | The maximum value recorded in the `SUMMARIZED` column for this Tag ID.      | **Yes**               |
| `MAX_VALUE_TIME`  | DATETIME  | The timestamp corresponding to the first occurrence of `MAX_VALUE`.         | **Yes**               |
| `RECENT_ROW_TIME` | DATETIME  | The timestamp of the most recently inserted data point for this Tag ID.     | No                    |

*Note: The data type for `MIN_VALUE` and `MAX_VALUE` mirrors the data type of the `value` column declared with `SUMMARIZED` in the parent TAG table.*

## Querying Statistics

The primary benefit of the `v$tag_table_name_stat` view is the ability to retrieve these key statistics rapidly without scanning the potentially voluminous base TAG table data.

**Basic Query Patterns:**

```sql
-- Retrieve min/max time boundaries for a specific tag
SELECT min_time, max_time
FROM v$your_tag_table_stat -- Replace 'your_tag_table' with the actual table name
WHERE name = 'specific_tag_id';

-- Retrieve min/max time boundaries for multiple tags
SELECT name, min_time, max_time
FROM v$your_tag_table_stat
WHERE name IN ('tag_id_1', 'tag_id_2', 'tag_id_3');

-- Retrieve row count and min/max values for a specific tag (requires SUMMARIZED)
SELECT row_count, min_value, max_value
FROM v$your_tag_table_stat
WHERE name = 'specific_tag_id';

-- Retrieve all statistics for all tags
SELECT *
FROM v$your_tag_table_stat;

-- Retrieve the actual data record corresponding to the most recent entry for a tag
SELECT *
FROM your_tag_table
WHERE name = 'specific_tag_id'
  AND time = (SELECT recent_row_time
              FROM v$your_tag_table_stat
              WHERE name = 'specific_tag_id');

-- Retrieve the actual data record corresponding to the minimum value occurrence for a tag
SELECT *
FROM your_tag_table
WHERE name = 'specific_tag_id'
  AND time = (SELECT min_value_time
              FROM v$your_tag_table_stat
              WHERE name = 'specific_tag_id');
```

## Limitations

The per-tag statistics feature, while highly beneficial for performance, possesses certain inherent limitations:

*   **Fixed Statistical Scope:** The view provides a predefined set of eight statistical indicators. If different or more complex statistical functions are required (e.g., standard deviation, percentiles), direct computation on the base TAG table or utilization of other features like Rollup tables may be necessary.
*   **Dependency on Source Data Integrity:** The accuracy of the statistics stored in the `v$tag_table_name_stat` view is directly dependent on the quality of the data ingested into the source TAG table. Erroneous data points or noise will be reflected in the calculated aggregates (MIN_VALUE, MAX_VALUE, etc.). Input data validation and cleansing are recommended.
*   **Configuration Requirements:** As previously detailed, the statistics collection mechanism relies on the `TAG_STAT_ENABLE=1` property and the presence of the `SUMMARIZED` keyword on the target value column for full functionality. Incorrect configuration will result in incomplete or entirely absent statistical data within the view.

## Examples

This section provides practical examples demonstrating the setup and usage of the per-tag statistics feature.

### Prerequisites: Schema Creation and Data Loading

These examples assume a TAG table named `tag` is created and populated, mirroring the setup described in the presentation materials.

**1. Schema Definition (Ensuring Statistics Enabled):**

```sql
-- Drop existing table if necessary
-- DROP TABLE tag;

-- Create the TAG table, enabling statistics and marking 'value' for summary
CREATE TAG TABLE tag (
    name VARCHAR(80) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED -- Ensure SUMMARIZED is present for value stats
)
tag_partition_count=1, -- Example property
TAG_STAT_ENABLE=1;    -- Explicitly set (though default=1)
```

**2. Data Loading (Example using `machbase-neo` import):**

Assume a CSV file `homes.csv` exists with data in the format `tag_name,unix_timestamp,value`.

```bash
# Example import command (adjust path and options as needed)
machbase-neo>> import --input C:\path\to\homes.csv --timeformat s --table tag --method append TAG;
```

### Verifying Statistics and Basic Queries

**1. Inspecting the Statistics View Structure:**

```sql
-- Display the columns and types of the auto-generated statistics view
DESC v$tag_stat;
```

**2. Checking Row Counts per Tag:**

```sql
-- Efficiently get row counts using the statistics view
SELECT name, row_count
FROM v$tag_stat
ORDER BY name;

-- Compare with direct count on the base table (will be slower for large tables)
SELECT name, COUNT(*) AS direct_count
FROM tag
GROUP BY name
ORDER BY name;
```

**3. Retrieving Time Boundaries:**

```sql
-- Get the earliest and latest timestamps for specific tags
SELECT name, min_time, max_time
FROM v$tag_stat
WHERE name IN ('use', 'gen', 'temperature');
```

**4. Querying Data at Specific Statistical Points:**

```sql
-- Get the full data record for the most recently added 'use' tag data
SELECT *
FROM tag
WHERE name = 'use'
  AND time = (SELECT recent_row_time FROM v$tag_stat WHERE name = 'use');

-- Get the full data record when the minimum 'temperature' was recorded
SELECT *
FROM tag
WHERE name = 'temperature'
  AND time = (SELECT min_value_time FROM v$tag_stat WHERE name = 'temperature');

-- Get the recorded value at the maximum time for the 'gen' tag
SELECT value
FROM tag
WHERE name = 'gen'
  AND time = (SELECT max_time FROM v$tag_stat WHERE name = 'gen');
```

### Illustrating Configuration Impact (Limitations)

These examples demonstrate how configuration choices affect statistics collection.

**Scenario Setup:** Create three tables with different configurations but load them with the *same* sample data.

```sql
-- Table 1: Full Statistics Enabled (Standard Case)
DROP TABLE IF EXISTS stat1;
CREATE TAG TABLE stat1 (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED) TAG_STAT_ENABLE=1;

-- Table 2: SUMMARIZED Keyword Omitted
DROP TABLE IF EXISTS stat2;
CREATE TAG TABLE stat2 (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE) TAG_STAT_ENABLE=1; -- No SUMMARIZED

-- Table 3: Statistics Disabled via Property
DROP TABLE IF EXISTS stat3;
CREATE TAG TABLE stat3 (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED) TAG_STAT_ENABLE=0; -- Stats explicitly disabled

-- Insert Identical Sample Data into all three tables
INSERT INTO stat1 VALUES('tag-0', TO_DATE('2022-08-11'), 10);
INSERT INTO stat1 VALUES('tag-0', TO_DATE('2022-08-13'), 20);
INSERT INTO stat1 VALUES('tag-0', TO_DATE('2022-08-14'), 5);
INSERT INTO stat1 VALUES('tag-1', TO_DATE('2023-08-12'), 200);
INSERT INTO stat1 VALUES('tag-1', TO_DATE('2023-08-13'), 50);
INSERT INTO stat1 VALUES('tag-1', TO_DATE('2023-08-15'), 120);

INSERT INTO stat2 VALUES('tag-0', TO_DATE('2022-08-11'), 10);
INSERT INTO stat2 VALUES('tag-0', TO_DATE('2022-08-13'), 20);
INSERT INTO stat2 VALUES('tag-0', TO_DATE('2022-08-14'), 5);
INSERT INTO stat2 VALUES('tag-1', TO_DATE('2023-08-12'), 200);
INSERT INTO stat2 VALUES('tag-1', TO_DATE('2023-08-13'), 50);
INSERT INTO stat2 VALUES('tag-1', TO_DATE('2023-08-15'), 120);

INSERT INTO stat3 VALUES('tag-0', TO_DATE('2022-08-11'), 10);
INSERT INTO stat3 VALUES('tag-0', TO_DATE('2022-08-13'), 20);
INSERT INTO stat3 VALUES('tag-0', TO_DATE('2022-08-14'), 5);
INSERT INTO stat3 VALUES('tag-1', TO_DATE('2023-08-12'), 200);
INSERT INTO stat3 VALUES('tag-1', TO_DATE('2023-08-13'), 50);
INSERT INTO stat3 VALUES('tag-1', TO_DATE('2023-08-15'), 120);

```

**Observing the Results:**

```sql
-- Query Statistics for Table 1 (Full Stats)
SELECT * FROM v$stat1_stat;
-- Expected: All columns populated, including MIN/MAX_VALUE and their times.

-- Query Statistics for Table 2 (No SUMMARIZED)
SELECT * FROM v$stat2_stat;
-- Expected: MIN_VALUE, MIN_VALUE_TIME, MAX_VALUE, MAX_VALUE_TIME columns will be NULL.
--          ROW_COUNT, MIN_TIME, MAX_TIME, RECENT_ROW_TIME will be populated.

-- Query Statistics for Table 3 (TAG_STAT_ENABLE=0)
SELECT * FROM v$stat3_stat;
-- Expected: No rows returned, or an error if the view itself wasn't created.
--          Statistics collection is entirely disabled for this table.
```

These examples clearly show the necessity of correct table definition (specifically the `SUMMARIZED` keyword) and the `TAG_STAT_ENABLED` property to leverage the full capabilities of the per-tag statistics feature.