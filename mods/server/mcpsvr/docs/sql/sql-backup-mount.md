# Machbase Neo SQL Backup and Mount

## Introduction

The exponential growth of time-series data, often termed "Industrial Big Data" or associated with Smart-X initiatives, presents significant challenges for traditional data management strategies. Persistently storing vast quantities of sensor readings necessitates robust mechanisms for data archival, disaster recovery, and historical analysis. Conventional database backup and restore processes, while essential, often suffer from limitations concerning scope flexibility, prolonged restoration times, and the inability to access backup contents without a full restore operation, which can be disruptive and resource-intensive.

Machbase addresses these challenges by providing a comprehensive and flexible suite of Backup and Mount functionalities specifically tailored for time-series data workloads. This includes various backup strategies (full, incremental, time-based, table-specific) and, critically, a **Mount** feature that allows read-only, online access to backup data without requiring a time-consuming restore process.

## Core Concepts

*   **Backup:** The process of creating a physical copy of database data (either the entire database or specific tables/time ranges) to an external storage location. Machbase backups are stored as a directory structure containing the necessary data and metadata files.
*   **Restore:** The process of copying data from a backup back into a live Machbase database instance. This typically overwrites existing data and is used primarily for disaster recovery or setting up replica environments.
*   **Mount:** A unique Machbase feature allowing a backup directory structure to be attached to a running Machbase instance as an ephemeral, read-only database. This provides immediate query access to the historical "fossilized" data contained within the backup, bypassing the need for a lengthy restore operation.
*   **Unmount:** The process of detaching a previously mounted backup database, releasing the association without deleting the backup files.
*   **Live Data:** The current, active data within the operational Machbase instance, subject to real-time reads and writes.
*   **Fossilized Data:** Data contained within a backup, representing an immutable snapshot at a specific point in time (or time range). When mounted, this data is accessible for read operations only.

## Backup Operations

Machbase provides granular control over the backup process, allowing users to select the appropriate scope and type based on requirements.

### Full Backup (Database or Table)

Creates a complete copy of either the entire database instance or specified tables at the time of execution.

**Syntax:**

```sql
BACKUP DATABASE INTO DISK = 'path/to/backup_directory_name';

BACKUP TABLE table_name INTO DISK = 'path/to/backup_directory_name';
```

*   `DATABASE`: Specifies a backup of the entire database instance.
*   `TABLE table_name`: Specifies a backup of only the named table.
*   `INTO DISK = 'path/...'`: Defines the target directory where the backup files will be created. This path can be absolute or relative to the `$MACHBASE_HOME/dbs` directory. The specified directory will be created if it doesn't exist.

**Considerations:**
*   A full backup captures the state of the data at the moment the backup operation commences.
*   The output is a directory containing multiple files and subdirectories representing the database structure.

### Incremental Backup (Database or Table)

Captures only the data that has changed since a previous backup (typically a full backup or a prior incremental backup). This significantly reduces backup time and storage space for subsequent backups.

**Syntax:**

```sql
BACKUP DATABASE AFTER 'path/to/previous_backup' INTO DISK = 'path/to/incremental_backup_dir';

BACKUP TABLE table_name AFTER 'path/to/previous_backup' INTO DISK = 'path/to/incremental_backup_dir';
```

*   `AFTER 'path/...'`: Specifies the path to the directory of the *immediately preceding* backup (full or incremental) in the chain. This path **must** exist and be accessible.
*   `INTO DISK = 'path/...'`: Defines the target directory for the *new* incremental backup files.

**Considerations:**
*   Primarily applicable to Log and Tag tables where data is typically appended.
*   Lookup tables are **always** fully backed up, even during an incremental operation, due to their potential for non-append-only modifications.
*   Requires the previous backup directory to be present and intact.

### Time-Based Backup (Database or Table)

Allows backing up data within a specific time window, particularly useful for archiving historical time-series data based on date ranges.

**Syntax:**

```sql
BACKUP DATABASE
    FROM time_expression_start
    TO time_expression_end
    INTO DISK = 'path/to/backup_directory_name';

BACKUP TABLE table_name
    FROM time_expression_start
    TO time_expression_end
    INTO DISK = 'path/to/backup_directory_name';
```

*   `FROM time_expression_start`: Defines the inclusive start timestamp for the backup window (e.g., `TO_DATE('YYYY-MM-DD HH24:MI:SS')`).
*   `TO time_expression_end`: Defines the inclusive end timestamp for the backup window.

**Considerations:**
*   Ideal for segmenting large time-series tables into manageable backup units (e.g., monthly, quarterly).

## Mount Operations

The Mount feature provides read-only access to backup data without a restore.

### Mounting a Backup

Attaches a backup directory to the running Machbase instance as a queryable, read-only database.

**Syntax:**

```sql
MOUNT DATABASE 'path/to/backup_directory' TO mount_name;
```

*   `'path/to/backup_directory'`: The full path to the directory containing the Machbase backup files (created via `BACKUP ... INTO DISK`).
*   `mount_name`: A user-defined alias for this mounted database. This name is used to qualify object names when querying the mounted data.

**Considerations:**
*   The Machbase server process must have read access to the backup directory path.
*   Multiple backups can be mounted concurrently, each with a unique `mount_name`.

### Querying Mounted Data

Accessing tables within a mounted backup requires qualifying the table name with the mount name and the original schema/user name (typically `sys` for standard tables).

**Syntax:**

```sql
SELECT column_list
FROM mount_name.user_name.table_name
WHERE [conditions];
```

*   `mount_name`: The alias assigned during the `MOUNT DATABASE` command.
*   `user_name`: The schema owner of the original table (commonly `sys`).
*   `table_name`: The name of the table within the backup.

**Example:**

```sql
-- Querying table 'sensor_data' (owned by 'sys') from a backup mounted as 'backup_jan'
SELECT *
FROM backup_jan.sys.sensor_data
WHERE time BETWEEN TO_DATE('2024-01-05') AND TO_DATE('2024-01-06');
```

### Unmounting a Backup

Detaches a mounted backup database, making its contents inaccessible via the mount point. The backup files themselves remain untouched on disk.

**Syntax:**

```sql
UNMOUNT DATABASE mount_name;
```

*   `mount_name`: The alias of the mounted database to detach.

## Restore Operations

Restoring replaces the current database state with the state captured in a backup. This is primarily used for disaster recovery or setting up identical instances.

**Syntax (using `machbase-neo` utility):**

```bash
machbase-neo restore --data <machbase_home_dir> <path/to/backup_directory>
```

*   `--data <machbase_home_dir>`: Specifies the `$MACHBASE_HOME` directory of the target Machbase instance where the restore should occur. **Caution: This process typically overwrites existing data in the target instance.**
*   `<path/to/backup_directory>`: The path to the backup directory to restore from.
    *   For full restores, this is the directory of the full backup.
    *   For restores involving incremental backups, this **must** be the path to the **last** incremental backup directory in the chain. The `restore` process will automatically locate and utilize the preceding backups in the chain (full and intermediate incrementals).

**Considerations:**
*   Restore is an offline operation or requires careful handling on a live instance, as it generally overwrites the existing database state.
*   Ensure the target `$MACHBASE_HOME` is correct to avoid unintended data loss.
*   If read/write access to the backup data is required, a full restore is necessary; the Mount feature only provides read-only access.

## Advantages and Considerations of Mounting

### Advantages

*   **Rapid Data Access:** Provides near-instantaneous access to historical data within backups, eliminating the potentially lengthy durations associated with traditional restore processes (especially for multi-terabyte datasets).
*   **Index Preservation:** Backups retain the original time-series indexing structures. Mounted databases leverage these indexes, ensuring high-performance queries on historical data, comparable to querying live data.
*   **Rollup Structure Preservation:** Any Rollup tables associated with the backed-up tables are also preserved within the backup and are queryable via the mount point, allowing consistent statistical analysis across live and historical ("fossilized") data.
*   **Online Operation:** Mounting and unmounting occur while the primary database instance is online and operational.
*   **Resource Efficiency:** Avoids the significant disk I/O and CPU resources required for a full restore operation simply to query historical data.

### Considerations

*   **Read-Only Access:** Mounted databases are strictly read-only. `INSERT`, `UPDATE`, `DELETE`, and DDL operations are prohibited. A `RESTORE` operation is required if modifications to the backup state are needed.
*   **`v$mount_table_STAT` Unavailability:** System views providing real-time statistics (like `v$mount_table_STAT`, if it were to exist analogously to `v$sys_table_STAT`) are generally not applicable or available for statically mounted backup data. Query the data directly for counts or aggregates.
*   **Filesystem Access:** The Machbase server requires filesystem-level read access to the backup directory location.

## Examples

This section provides practical scenarios demonstrating the Backup and Mount workflow.

**Setup:** Assume two TAG tables, `EQPT_A` and `EQPT_B`, exist and are populated with time-series data.

```sql
-- Initial Schema (Example)
CREATE TAG TABLE IF NOT EXISTS EQPT_A (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED) tag_partition_count=1;
CREATE TAG TABLE IF NOT EXISTS EQPT_B (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED) tag_partition_count=1;

-- Assume data is loaded into EQPT_A and EQPT_B covering dates from 2024-01-01 to 2024-06-30
-- Example verification of live data range
SELECT TO_CHAR(MIN(time)), TO_CHAR(MAX(time)) FROM EQPT_A;
SELECT COUNT(*) FROM EQPT_A;
```

### Example 1: Full Database Backup and Mount

```sql
-- 1. Perform a full database backup into a specified directory
-- Ensure the target directory path exists or Machbase has permissions to create it.
BACKUP DATABASE INTO DISK = '/backup/full_db_20240630'; -- Use an appropriate path

-- 2. Mount the backup with an alias 'mount_fulldb'
MOUNT DATABASE '/backup/full_db_20240630' TO mount_fulldb;

-- 3. Query data from the mounted backup
-- Check time range in mounted EQPT_A
SELECT TO_CHAR(MIN(time)), TO_CHAR(MAX(time)) FROM mount_fulldb.sys.EQPT_A;
-- Check row count in mounted EQPT_A
SELECT COUNT(*) FROM mount_fulldb.sys.EQPT_A;
-- Query specific data from mounted EQPT_B
SELECT name, TO_CHAR(time), value FROM mount_fulldb.sys.EQPT_B LIMIT 5;

-- 4. Unmount the backup when access is no longer needed
UNMOUNT DATABASE mount_fulldb;
```

### Example 2: Time-Range Database Backup and Mount

```sql
-- 1. Backup data only from Jan 1st, 2024 to Mar 31st, 2024
BACKUP DATABASE
    FROM TO_DATE('2024-01-01 00:00:00', 'YYYY-MM-DD HH24:MI:SS')
    TO TO_DATE('2024-03-31 23:59:59', 'YYYY-MM-DD HH24:MI:SS')
    INTO DISK = '/backup/db_2024Q1'; -- Use an appropriate path

-- 2. (Optional) Simulate data aging by deleting older data from the live tables
DELETE FROM EQPT_A BEFORE TO_DATE('2024-03-31 23:59:59', 'YYYY-MM-DD HH24:MI:SS');
DELETE FROM EQPT_B BEFORE TO_DATE('2024-03-31 23:59:59', 'YYYY-MM-DD HH24:MI:SS');
-- Verify live data count has decreased
SELECT COUNT(*) FROM EQPT_A;

-- 3. Mount the time-range backup
MOUNT DATABASE '/backup/db_2024Q1' TO mount_q1;

-- 4. Query the mounted backup - it should contain the original data for Q1
SELECT COUNT(*) FROM mount_q1.sys.EQPT_A; -- Should match original Q1 count
SELECT TO_CHAR(MIN(time)), TO_CHAR(MAX(time)) FROM mount_q1.sys.EQPT_A; -- Should show Q1 range

-- Compare counts between live (post-delete) and mounted (pre-delete)
SELECT COUNT(*) AS live_count FROM EQPT_A;
SELECT COUNT(*) AS mounted_q1_count FROM mount_q1.sys.EQPT_A;

-- 5. Unmount the backup
UNMOUNT DATABASE mount_q1;
```

### Example 3: Table-Specific, Time-Range Backup and Mount

```sql
-- 1. Backup only table EQPT_A for the period April 1st to May 15th, 2024
BACKUP TABLE EQPT_A
    FROM TO_DATE('2024-04-01 00:00:00', 'YYYY-MM-DD HH24:MI:SS')
    TO TO_DATE('2024-05-15 23:59:59', 'YYYY-MM-DD HH24:MI:SS')
    INTO DISK = '/backup/eqpta_20240401_20240515'; -- Use an appropriate path

-- 2. Mount the table-specific backup
MOUNT DATABASE '/backup/eqpta_20240401_20240515' TO mount_eqpta_partial;

-- 3. Query the mounted backup
-- Verify time range and count match the specified backup window
SELECT TO_CHAR(MIN(time)), TO_CHAR(MAX(time)) FROM mount_eqpta_partial.sys.EQPT_A;
SELECT COUNT(*) FROM mount_eqpta_partial.sys.EQPT_A;
-- Query raw data sample
SELECT name, TO_CHAR(time), value FROM mount_eqpta_partial.sys.EQPT_A LIMIT 5;

-- Attempting to query EQPT_B from this mount will fail (it wasn't included)
-- SELECT COUNT(*) FROM mount_eqpta_partial.sys.EQPT_B; -- Expected error: table not found

-- 4. Unmount the backup
UNMOUNT DATABASE mount_eqpta_partial;
```

### Example 4: Mounting Multiple Backups Concurrently

```sql
-- Assuming backups from previous examples exist:
-- '/backup/db_2024Q1' (Full DB, Q1)
-- '/backup/full_db_20240630' (Full DB, up to Jun 30)
-- '/backup/eqpta_20240401_20240515' (EQPT_A only, Apr 1 - May 15)

-- 1. Mount all three backups with unique aliases
MOUNT DATABASE '/backup/db_2024Q1' TO mount_q1;
MOUNT DATABASE '/backup/full_db_20240630' TO mount_jun30;
MOUNT DATABASE '/backup/eqpta_20240401_20240515' TO mount_eqpta_partial;

-- 2. Query data across different mounts
-- Count from EQPT_A in Q1 backup
SELECT COUNT(*) FROM mount_q1.sys.EQPT_A;
-- Count from EQPT_B in the full backup
SELECT COUNT(*) FROM mount_jun30.sys.EQPT_B;
-- Count from EQPT_A in the partial table backup
SELECT COUNT(*) FROM mount_eqpta_partial.sys.EQPT_A;
-- Attempt to count EQPT_B from the partial backup (will fail)
-- SELECT COUNT(*) FROM mount_eqpta_partial.sys.EQPT_B;

-- 3. Unmount all backups when finished
UNMOUNT DATABASE mount_q1;
UNMOUNT DATABASE mount_jun30;
UNMOUNT DATABASE mount_eqpta_partial;
```