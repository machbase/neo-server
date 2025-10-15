# Machbase Neo SQL Duplicate Data Removal

## Introduction

In distributed systems involving sensor data collection and transmission, transient network interruptions or specific device configurations can lead to the retransmission of identical data points. While intended to ensure data delivery and prevent loss, this practice results in duplicate records being ingested into the database. Managing data integrity by identifying and eliminating these duplicates at the application layer presents significant challenges, particularly concerning performance degradation under high data volumes.

Machbase provides a built-in, database-level mechanism to address this issue. The Duplicate Transmission Removal feature allows for the automatic detection and rejection of duplicate data entries based on a configurable temporal window, thereby maintaining data consistency without requiring complex application-level logic. This feature operates specifically on Machbase TAG tables.

## Core Concept

The Duplicate Transmission Removal feature functions by defining a specific lookback duration during TAG table creation. This duration establishes a temporal window, relative to the system time of data insertion. When a new data row is being inserted into the TAG table, Machbase performs a check:

1.  **Identification:** It compares the `PRIMARY KEY` column value (typically the Tag ID or `name`) and the `BASETIME` column value (the timestamp) of the incoming row against existing rows in the table.
2.  **Temporal Check:** It searches for any existing row that has the *exact same* `PRIMARY KEY` and `BASETIME` values as the incoming row.
3.  **Window Condition:** If such an identical row exists, and its `BASETIME` falls within the configured lookback duration (measured backward from the **system time** of the current insertion attempt), the incoming row is considered a duplicate.
4.  **Action:** Duplicate incoming rows identified through this process are automatically discarded and are not persisted in the TAG table.

Essentially, this mechanism implements a "first-write-wins" semantic within the defined temporal window. The very first instance of a data point, identified by its unique combination of Tag ID and timestamp, is successfully stored. Any subsequent attempts to insert a row with the *exact same* Tag ID and timestamp, occurring while the original record's timestamp is still within the lookback window relative to the current system time, will be silently ignored. Importantly, this check is based *only* on the `PRIMARY KEY` and `BASETIME` columns; differences in other columns (like the sensor value) do not prevent a row from being identified as a duplicate if the key and time match.

## Configuration

The Duplicate Transmission Removal feature is configured at the time of TAG table creation using a specific table property:

*   **`TAG_DUPLICATE_CHECK_DURATION`**: This property specifies the duration, in minutes, for the lookback window used for duplicate detection.

**Syntax:**

```sql
CREATE TAG TABLE table_name (
    name_column datatype PRIMARY KEY,
    time_column DATETIME BASETIME,
    value_column datatype [SUMMARIZED]
    [, additional_columns...]
)
TAG_DUPLICATE_CHECK_DURATION = duration_in_minutes;
```

*   `duration_in_days`: An integer specifying the lookback period in minutes.
    *   Minimum value: `1` (minute)
    *   Maximum value: `43200` (minutes)
    *   Default value: `0` (disabled)

**Verification:**

The configured duration for a specific TAG table can be verified by querying the system catalog views.

1.  **Retrieve Table ID:**
    ```sql
    SELECT id
    FROM m$sys_tables
    WHERE name = 'YOUR_TABLE_NAME'; -- Note: Table name must be in uppercase
    ```

2.  **Query Property Value:**
    ```sql
    SELECT value
    FROM m$sys_table_property
    WHERE id = {table_id_from_step_1}
      AND name = 'TAG_DUPLICATE_CHECK_DURATION';
    ```

**Changing configuration**
TAG_DUPLICATE_CHECK_DURATION settings can be modified as shown below.
```sql
ALTER TABLE {table_name} set TAG_DUPLICATE_CHECK_DURATION={duration in minutes};
```

## Behavior and Constraints

Understanding the following constraints and behavioral aspects is crucial for effectively utilizing this feature:

*   **Granularity and Scope:** The duration is configured exclusively in minutes units, with a maximum temporal scope of 43200 minutes(30 days).
*   **Interaction with Data Deletion:** The deduplication check relies on the presence of the original data point within the lookback window. If the *original* data record (the "first write") is explicitly deleted from the TAG table *before* an identical duplicate arrives, the newly arriving record will **not** be identified as a duplicate. It will be treated as a new "first write" because its potential duplicate counterpart no longer exists for comparison within the database's current state.
*   **Semantic Behavior:** The mechanism strictly adheres to keeping the *first* encountered record for a given (Primary Key, Basetime) combination and discarding subsequent identical entries within the defined window. It is not suitable for scenarios requiring "last-write-wins" semantics.
*   **Consistency Model:** In high-volume, real-time ingestion scenarios, there might be minimal latency between data insertion and the point at which the deduplication check fully reflects the most current state across all internal structures. This is consistent with typical eventually consistent behaviors in distributed data systems.
*   **Target-Based Deduplication:** This feature performs deduplication within the Machbase database (target-based). It does not prevent duplicate data from being transmitted by the source system.
*   **Resource Implications:** Compared to source-based deduplication (where the application filters duplicates before transmission), target-based deduplication inherently consumes additional database resources (CPU, I/O) to perform the necessary checks during the ingestion process.

## Examples

This section provides practical examples of creating a TAG table with duplicate removal enabled and observing its behavior.

**1. Schema Definition:**

```sql
-- Drop the table if it exists from previous runs
DROP TABLE IF EXISTS dup_tag;

-- Create a TAG table named 'dup_tag'
-- Configure it to check for duplicates within a 1440 minutes (1-day) window
CREATE TAG TABLE dup_tag (
    name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED -- Value column (summarized is optional for dedupe itself)
)
TAG_DUPLICATE_CHECK_DURATION=1440; -- Enable deduplication with a 1440 minutes (1-day) lookback
```

**2. Data Insertion:**

The following INSERT statements demonstrate how duplicates are handled. Assume these are executed sequentially and the system time progresses such that the 1-day window is relevant for timestamps on `2024-01-02` relative to each other, `2024-01-04` relative to each other, etc.

```sql
-- Insert initial records
INSERT INTO dup_tag VALUES('tag1', '2024-01-01 09:00:00 000:000:001', 0); -- Kept (First instance)
INSERT INTO dup_tag VALUES('tag1', '2024-01-02 09:00:00 000:000:001', 0); -- Kept (Different day/time from first)
INSERT INTO dup_tag VALUES('tag1', '2024-01-02 09:00:00 000:000:002', 0); -- Kept (First instance for this specific time)

-- Attempt to insert a duplicate (same name, same time as previous row)
-- This row has a different 'value' (1 vs 0), but will still be treated as a duplicate
-- because the (name, time) pair matches an existing record within the duration.
INSERT INTO dup_tag VALUES('tag1', '2024-01-02 09:00:00 000:000:002', 1); -- Discarded (Duplicate based on name & time)

-- Insert record for a subsequent day
INSERT INTO dup_tag VALUES('tag1', '2024-01-03 09:00:00 000:000:003', 0); -- Kept (New timestamp)

-- Insert records for a different tag ('tag2'), demonstrating multiple duplicates at the same timestamp
INSERT INTO dup_tag VALUES('tag2', '2024-01-04 09:00:00 000:000:001', 0); -- Kept (First instance for tag2 at this time)
INSERT INTO dup_tag VALUES('tag2', '2024-01-04 09:00:00 000:000:001', 1); -- Discarded (Duplicate based on name & time)
INSERT INTO dup_tag VALUES('tag2', '2024-01-04 09:00:00 000:000:001', 2); -- Discarded (Duplicate based on name & time)

-- Insert records for 'tag2' at different timestamps
INSERT INTO dup_tag VALUES('tag2', '2024-01-04 09:00:00 000:000:002', 1); -- Kept (First instance for this specific time)
INSERT INTO dup_tag VALUES('tag2', '2024-01-04 09:00:00 000:000:003', 2); -- Kept (First instance for this specific time)

```

**3. Data Verification:**

Querying the table will show only the records that were successfully inserted (i.e., the "first-write" for each unique `name` and `time` combination within the effective window).

```sql
-- Query data for 'tag1'
SELECT * FROM dup_tag WHERE name = 'tag1';

/* Expected Output for tag1:
ROWNUM | NAME | TIME                              | VALUE
------ | ---- | --------------------------------- | -----
1      | tag1 | 2024-01-01 09:00:00 000:000:001 | 0.0
2      | tag1 | 2024-01-02 09:00:00 000:000:001 | 0.0
3      | tag1 | 2024-01-02 09:00:00 000:000:002 | 0.0  -- Note: The row with value 1 was discarded
4      | tag1 | 2024-01-03 09:00:00 000:000:003 | 0.0
*/

-- Query data for 'tag2'
SELECT * FROM dup_tag WHERE name = 'tag2';

/* Expected Output for tag2:
ROWNUM | NAME | TIME                              | VALUE
------ | ---- | --------------------------------- | -----
1      | tag2 | 2024-01-04 09:00:00 000:000:001 | 0.0  -- Note: Rows with values 1 and 2 at this time were discarded
2      | tag2 | 2024-01-04 09:00:00 000:000:002 | 1.0
3      | tag2 | 2024-01-04 09:00:00 000:000:003 | 2.0
*/
```