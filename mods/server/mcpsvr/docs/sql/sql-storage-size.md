# Machbase Neo SQL Automatic Storage Management

## Introduction

Time-series databases, particularly those handling high-frequency data from numerous sources, face the challenge of continuous data accumulation. Ingesting potentially millions of data points per second necessitates substantial storage capacity. Over time, managing this storage often involves manual monitoring of disk utilization followed by periodic execution of `DELETE` operations to reclaim space, introducing operational complexity and potential for error. Furthermore, many applications require data to be retained only for a specific operational period, after which older data becomes obsolete.

To address these challenges, Machbase implements an Automatic Storage Size Management mechanism through its **Retention Policy** feature. This feature provides a declarative approach to automatically purge data that has aged beyond a defined retention period, thereby maintaining predictable storage utilization and simplifying long-term data lifecycle management.

## Core Concepts: Retention Policy

A Retention Policy in Machbase defines a rule for the automatic, time-based deletion of data from specified tables. It operates based on two primary parameters:

*   **Duration:** This specifies the maximum age of data to be retained within a table. Data older than this duration, measured relative to the current system time during the policy check, becomes eligible for deletion. The duration can be defined in units of `MONTH` or `DAY`.
*   **Interval:** This determines the frequency at which Machbase checks the associated table(s) for data eligible for deletion based on the defined `DURATION`. The interval defines how often the retention enforcement process is executed and can be set in units of `DAY` or `HOUR`.

When a Retention Policy is applied to a table, a background process periodically (as defined by `INTERVAL`) scans the table. It identifies and automatically deletes all data rows whose timestamp (specifically, the value in the `BASETIME` column) is older than the current system time minus the specified `DURATION`.

The lifecycle of managing data retention using this feature involves:
1.  Creating a named Retention Policy object specifying the `DURATION` and `INTERVAL`.
2.  Applying the created Retention Policy to one or more target tables.
3.  Machbase automatically executing the deletion process according to the policy's schedule.
4.  Optionally detaching the policy from a table if automatic deletion is no longer required for that table.
5.  Optionally dropping the Retention Policy object itself once it is no longer applied to any tables.

## Creating a Retention Policy

A Retention Policy is defined as a distinct database object using the `CREATE RETENTION` statement.

**Syntax:**

```sql
CREATE RETENTION policy_name
    DURATION duration_value { MONTH | DAY }
    INTERVAL interval_value { DAY | HOUR };
```

*   `policy_name`: A unique identifier chosen by the user for this specific retention policy.
*   `duration_value`: An integer representing the length of the data retention period.
*   `MONTH | DAY`: The time unit for the `duration_value`.
*   `interval_value`: An integer representing the frequency of the deletion check.
*   `DAY | HOUR`: The time unit for the `interval_value`.

**Examples:**

```sql
-- Policy to retain data for 1 day, checking every 1 hour
CREATE RETENTION policy_1d_1h
    DURATION 1 DAY
    INTERVAL 1 HOUR;

-- Policy to retain data for 1 month (approximated), checking every 3 days
CREATE RETENTION policy_1m_3d
    DURATION 1 MONTH
    INTERVAL 3 DAY;
```

## Applying a Retention Policy to a Table

Once a Retention Policy is created, it must be explicitly associated with a target table using the `ALTER TABLE ... ADD RETENTION` statement. A table can only have one Retention Policy applied at any given time.

**Syntax:**

```sql
ALTER TABLE table_name ADD RETENTION policy_name;
```

*   `table_name`: The name of the table to which the policy should be applied.
*   `policy_name`: The name of a previously created Retention Policy object.

**Example:**

```sql
-- Assume a TAG table 'sensor_data' and policy 'policy_1d_1h' exist
CREATE TAG TABLE sensor_data ( name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED );

-- Apply the policy_1d_1h to the sensor_data table
ALTER TABLE sensor_data ADD RETENTION policy_1d_1h;
```

## Monitoring Retention Policies

Information about defined Retention Policies and their application status can be queried through system catalog views.

*   **`M$RETENTION`:** This view lists all Retention Policy objects defined in the database, showing their names and configured `DURATION` and `INTERVAL` values (represented internally in seconds).

    ```sql
    -- View all defined retention policies
    SELECT * FROM M$RETENTION;
    ```

*   **`V$RETENTION_JOB`:** This view displays which policies are currently applied to which tables, along with the status of the retention job (e.g., `WAITING`) and the timestamp of the last successful deletion execution (`LAST_DELETED_TIME`).

    ```sql
    -- View retention policies currently applied to tables
    SELECT * FROM V$RETENTION_JOB;
    ```

## Detaching and Removing Policies

A Retention Policy can be detached from a table, stopping the automatic deletion process for that specific table. The policy object itself can then be deleted if it's no longer needed and not applied to any other tables.

### Detaching from a Table

Use the `ALTER TABLE ... DROP RETENTION` statement to disassociate a policy from a table.

**Syntax:**

```sql
ALTER TABLE table_name DROP RETENTION;
```

*   `table_name`: The name of the table from which to detach the currently applied policy.

### Removing a Policy Object

Use the `DROP RETENTION` statement to delete the policy definition itself. This operation will fail if the policy is still applied to any table.

**Syntax:**

```sql
DROP RETENTION policy_name;
```

*   `policy_name`: The name of the Retention Policy object to be deleted.

**Dependency Example:**

```sql
-- Assume 'policy_1d_1h' is applied to 'sensor_data'

-- Attempting to drop the policy while it's in use will fail
DROP RETENTION policy_1d_1h;
-- Expected Error: [ERR-02702: Policy (POLICY_1D_1H) is in use.]

-- First, detach the policy from the table
ALTER TABLE sensor_data DROP RETENTION;

-- Now, dropping the policy object will succeed
DROP RETENTION policy_1d_1h;
```

## Examples

This section provides a step-by-step example of using the Retention Policy feature.

**1. Schema Setup:**

```sql
-- Ensure a clean state (drop table and potentially dependent rollup tables)
DROP TABLE IF EXISTS ret_tag CASCADE;

-- Create a sample TAG table (with Rollup for context, though not required for Retention)
CREATE TAG TABLE ret_tag (
    name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
) WITH ROLLUP(MIN) TAG_PARTITION_COUNT=1;
```

**2. Create Retention Policy:**

```sql
-- Define a policy to keep data for 1 day, checking hourly
CREATE RETENTION policy_1d_1h DURATION 1 DAY INTERVAL 1 HOUR;

-- Verify policy creation
SELECT * FROM M$RETENTION WHERE POLICY_NAME = 'POLICY_1D_1H';
```

**3. Apply Policy to Table:**

```sql
-- Apply the created policy to the 'ret_tag' table
ALTER TABLE ret_tag ADD RETENTION policy_1d_1h;

-- Verify policy application
SELECT * FROM V$RETENTION_JOB WHERE TABLE_NAME = 'RET_TAG';
-- Expected: A row showing RET_TAG, POLICY_1D_1H, state (likely WAITING), and NULL last_deleted_time initially.
```

**4. Load Data (Including Old Data):**

```tql
-- Use TQL FAKE function to simulate loading 150,000 records
-- spanning roughly the last 2 days (some older than 1 day).
-- Adjust timeAdd parameters as needed to ensure data older than DURATION exists.
FAKE(range(1, 150000, 1))
MAPVALUE(1, sin((2*PI*value(0)/100))) -- Sample value generation
MAPVALUE(0, timeAdd("now-2d", strSprintf("+%.fs", value(0)*100))) -- Generate timestamps over ~2 days ending now
PUSHVALUE(0, "sensor-a") -- Assign a tag name
APPEND(table("ret_tag")) -- Append to the target table
```

**5. Verify Initial Data Load:**

```sql
-- Check the total number of records inserted
SELECT COUNT(*) FROM ret_tag;
-- Expected: 150000 (or close to it, depending on exact FAKE generation)
```

**6. Wait for Retention Execution:**

Wait for a duration longer than the policy's `INTERVAL` (1 hour in this case). The background retention job will automatically run.

**7. Verify Data Deletion:**

```sql
-- Check the retention job status again; LAST_DELETED_TIME might be updated
SELECT * FROM V$RETENTION_JOB WHERE TABLE_NAME = 'RET_TAG';

-- Check the record count again. It should be lower than the initial count,
-- as records older than 1 day (relative to when the job ran) should have been deleted.
SELECT COUNT(*) FROM ret_tag;
-- Expected: A number less than 150000.
```

**8. Detach and Drop Policy:**

```sql
-- Stop automatic deletion for 'ret_tag'
ALTER TABLE ret_tag DROP RETENTION;

-- Verify detachment (the row for ret_tag should disappear)
SELECT * FROM V$RETENTION_JOB WHERE TABLE_NAME = 'RET_TAG';

-- Remove the policy definition itself
DROP RETENTION policy_1d_1h;

-- Verify removal
SELECT * FROM M$RETENTION WHERE POLICY_NAME = 'POLICY_1D_1H';
-- Expected: No rows returned.
```