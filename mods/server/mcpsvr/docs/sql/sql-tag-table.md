# Machbase Neo SQL Tag Table

## Overview of the Tag Table Data Model

This document provides a comprehensive overview of the Machbase Tag Table, a specialized table structure optimized for storing, retrieving, and managing time-series sensor data within the Machbase time-series database system.

### Conceptual Data Model

Traditional data modeling for sensor data often resembles a wide, CSV-like format where each row represents a single timestamp, and columns correspond to different sensor measurements (tags).

**Traditional Data Model (Wide Format):**

| timestamp           | temperature | humidity | pressure | vibration |
| :------------------ | :---------- | :------- | :------- | :-------- |
| 2023-04-15 09:34:12 | 23.5        | 78.9     | 11       | 55        |
| 2023-04-15 09:34:13 | 23.7        | 75.6     | 12       | 51        |
| ...                 | ...         | ...      | ...      | ...       |

*   **Characteristics:**
    *   Manages measurements taken at the same time as a single record.
    *   Facilitates viewing data in its original, wide format.
    *   Schema modifications (adding/removing sensors/tags) are relatively inflexible and often require table alterations.

The Machbase Tag Table employs a different paradigm, structuring data in a tall/narrow format where each row represents a single measurement from a specific sensor (tag) at a particular time.

**Machbase Tag Table Data Model (Tall/Narrow Format):**

| TAGID         | timestamp           | value |
| :------------ | :------------------ | :---- |
| temperature   | 2023-04-15 09:34:12 | 23.5  |
| humidity      | 2023-04-15 09:34:12 | 78.9  |
| pressure      | 2023-04-15 09:34:12 | 11    |
| vibration     | 2023-04-15 09:34:12 | 55    |
| temperature   | 2023-04-15 09:34:13 | 23.7  |
| humidity      | 2023-04-15 09:34:13 | 75.6  |
| ...           | ...                 | ...   |

*   **Characteristics:**
    *   Transforms and stores each measurement as an individual record.
    *   Offers maximum flexibility for schema evolution regarding tags (sensors); adding or removing tags does not require table structure changes.
    *   Enables efficient aggregation and statistical analysis on a per-tag basis.
    *   While the row count increases compared to the wide model, query and ingestion performance are typically enhanced due to the specialized architecture.

### Schema-Based Data Model Comparison

The difference in data modeling is reflected in the table creation syntax.

**Traditional Schema (Example):**

```sql
CREATE TABLE Vibration (
    time      DATETIME,
    temp      DOUBLE,
    humidity  DOUBLE,
    pressure  INTEGER,
    rms       LONG,
    tick      DOUBLE
    -- Additional columns for each new sensor type
);
```
*This represents a common design approach but faces challenges with schema rigidity in dynamic IoT environments.*

**Machbase Tag Table Schema:**

```sql
CREATE TAG TABLE Vibration (
    name  VARCHAR(80) PRIMARY KEY, -- Identifier for the specific tag/sensor
    time  DATETIME    BASETIME,    -- Timestamp of the measurement
    value DOUBLE                   -- The actual measured value
);
```
*This structure simplifies the core data schema, focusing on the fundamental elements of time-series data: identifier, time, and value. Additional context is managed via metadata.*

## Tag Table Fundamentals

### Structure

A Tag Table is an optimized table construct designed for the efficient ingestion, retrieval, and compression of structured sensor data. Its fundamental record structure consists of three core components:

1.  **Identifier (`name` column by default):** A unique string that identifies the specific sensor or data source (e.g., `"sensor-A"`, `"factory1-machine2-temp"`). This identifier serves as the primary key within the associated metadata structure.
2.  **Time (`time` column by default):** The timestamp indicating when the data point was generated or recorded. It is stored as a 64-bit integer, supporting nanosecond precision.
3.  **Value (`value` column by default):** The actual measurement or event data associated with the identifier at the specified time. While various data types are supported, `DOUBLE` (64-bit floating-point) is common and enables diverse analytical functions.

Internally, a Tag Table separates metadata (descriptive information about tags) from the actual time-series data points.

```
       Tag Table: Vibration
+--------------------------------------+------------------------------------------+
|        Meta (Sensor Attributes)      |            Data (Sensor Readings)        |
| +---------+-----------+------------+ | +----+---------------------------+-----+ |
| | NAME    | Attribute1| Attribute2 | | | ID | TIME (nanoseconds)        |VALUE| |
| +---------+-----------+------------+ | +----+---------------------------+-----+ |
| | Sensor-A| LocationX | TypeY      | | | 0  | 1719292147529850600       |-1.3 | |
| | Sensor-B| LocationZ | TypeW      | | | 1  | 1719292148529850600       |-2.3 | |
| | Sensor-C| LocationX | TypeY      | | | 2  | 1719292149529850600       |-3.3 | |
| | ...     | ...       | ...        | | | 0  | 1719292150000000000       |-4.3 | |
| +---------+-----------+------------+ | | 0  | 1719292167529850600       |-5.3 | |
|                                      | | 2  | 1719292177529850600       |-6.3 | |
| (Managed in _Vibration_META table)   | | 1  | 1719292187529850600       |-7.3 | |
|                                      | | .. | ...                       | ... | |
|                                      | +----+---------------------------+-----+ |
|                                      | (Managed in _Vibration_DATA_N partitions)|
+--------------------------------------+------------------------------------------+
```

The basic `CREATE` statement reflects this structure:

```sql
CREATE TAG TABLE Vibration (
    name  VARCHAR(80) PRIMARY KEY, -- Links to Meta table, unique identifier
    time  DATETIME    BASETIME,    -- Core time column for indexing
    value DOUBLE                   -- Core value column
    -- Optional additional data columns can be defined here
);
-- Metadata columns are defined separately in the METADATA clause
```

### Supported Data Types

Machbase Tag Tables support the following data types for the `value` column and any additional data columns:

| Type     | Description                      | Range / Representation                                          | NULL Representation           |
| :------- | :------------------------------- | :-------------------------------------------------------------- | :---------------------------- |
| `SHORT`    | 16-bit signed integer            | -32767 to 32767                                                 | -32768                        |
| `USHORT`   | 16-bit unsigned integer          | 0 to 65534                                                      | 65535                         |
| `INTEGER`  | 32-bit signed integer            | -2147483647 to 2147483647                                       | -2147483648                   |
| `UINTEGER` | 32-bit unsigned integer          | 0 to 4294967294                                                 | 4294967295                    |
| `LONG`     | 64-bit signed integer            | -9223372036854775807 to 9223372036854775807                     | -9223372036854775808          |
| `ULONG`    | 64-bit unsigned integer          | 0 to 18446744073709551614                                      | 18446744073709551615          |
| `FLOAT`    | 32-bit floating-point            | ±1.175494e-38 to ±3.402823e+38                                  | 3.402823466e+38               |
| `DOUBLE`   | 64-bit floating-point            | ±2.225074e-308 to ±1.797693e+308                                | 1.7976931348623158e+308       |
| `DATETIME` | Date and Time (nanosec precision)| From 1970-01-01 00:00:00 000:000:000 UTC                        | N/A                           |
| `VARCHAR`  | Variable-length string (UTF-8) | 1 byte to 32KB (32767 bytes)                                    | NULL                          |
| `IPV4`     | IPv4 address                     | "0.0.0.0" to "255.255.255.255"                                  | NULL                          |
| `IPV6`     | IPv6 address                     | "::" to "FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF"               | NULL                          |
| `JSON`     | JSON data type                   | Data length: 1 byte to 32KB; Path length: 1 to 512 characters | NULL                          |

**Note:** The `TEXT` and `BINARY` data types are **not supported** within Tag Tables.

## Tag Table Creation and Internal Architecture

### Creating Tag Tables

The fundamental syntax for creating a Tag Table is as follows:

```sql
CREATE TAG TABLE table_name (
    name_column VARCHAR(size) PRIMARY KEY, -- Tag identifier column
    time_column DATETIME BASETIME,         -- Time column with BASETIME property
    value_column datatype [SUMMARIZED]     -- Value column(s)
    [, additional_data_column datatype ...] -- Optional extra data columns
)
METADATA (
    meta_column1 datatype,                 -- Metadata columns
    meta_column2 datatype
    [, ...]
)
[ table_property = value [, ...] ];        -- Optional table properties
```

**Key Components:**

| Element                      | Description                                                                                                                                 | Area       |
| :--------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------ | :--------- |
| `name_column` (`PRIMARY KEY`)  | The column holding the unique tag identifier (e.g., sensor name). Must be `VARCHAR` type with a specified maximum length. Declared as `PRIMARY KEY`. | Data/Meta  |
| `time_column` (`BASETIME`)   | The column storing the timestamp for each data point, typically `DATETIME`. Must have the `BASETIME` property, indicating it's the primary time index. | Data       |
| `value_column` [`SUMMARIZED`] | The column(s) holding the measurement values. Common types include `DOUBLE`, `LONG`. The optional `SUMMARIZED` keyword enables built-in statistical aggregations for this column. | Data       |
| `additional_data_column`   | Optional columns to store supplementary data alongside the primary value for the same timestamp (e.g., quality flags, batch numbers).              | Data       |
| `METADATA` clause            | Defines columns that store descriptive attributes (metadata) for each unique tag specified in the `name_column`. These attributes are linked via the `name_column`. | Meta       |
| `table_property`             | Optional key-value pairs to configure table behavior and resource allocation (e.g., partitioning, statistics).                              | Table      |

### Tag Table Properties

Several properties can be configured during Tag Table creation to optimize performance and resource usage:

| Property                         | Description                                                                                                                               | Default | Notes                                                                                         |
| :------------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------- | :------ | :-------------------------------------------------------------------------------------------- |
| `TAG_PARTITION_COUNT`            | Number of internal data partitions (sub-tables) created. Affects parallelism for ingestion and querying.                                    | 4       | Higher values improve concurrency but increase memory usage. Use lower values (1 or 2) on resource-constrained edge devices. |
| `TAG_DATA_PART_SIZE`             | Target size (in bytes) for data storage units within partitions.                                                                          | 16MB    | Influences memory allocation related to data buffering and indexing.                          |
| `TAG_STAT_ENABLE`                | Enables/disables the collection of statistical metadata (min, max, count, sum) per tag. Required for `V$tableName_STAT` view.              | 1 (ON)  | Set to 0 to disable if statistics are not needed, potentially saving minor overhead.           |
| `TAG_DUPLICATE_CHECK_DURATION`   | Time window (in nanoseconds) within which duplicate records (same name, time, value) are potentially ignored during ingestion.            | 0       | Helps manage redundant data from sources that might occasionally resend data points.          |
| `VARCHAR_FIXED_LENGTH_MAX`       | Maximum length (in bytes) for `VARCHAR` data to be stored inline within the primary data storage. Longer strings may be stored externally. | 15      | Affects storage efficiency and retrieval performance for variable-length strings.               |

**Example with Properties:**

```sql
CREATE TAG TABLE basic (
    name VARCHAR(32) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
)
METADATA (
    factory VARCHAR(32),
    equipment VARCHAR(64)
)
TAG_PARTITION_COUNT=2,
TAG_STAT_ENABLE=0,
TAG_DUPLICATE_CHECK_DURATION=3;
```

### Internal Table Structure

Creating a Tag Table (e.g., `MYTAG`) results in the internal creation and management of several related objects:

1.  **`MYTAG` (Virtual Table):** The primary interface for querying data. It presents a unified view combining metadata and time-series data.
2.  **`_MYTAG_META` (Metadata Table):** Stores the metadata attributes defined in the `METADATA` clause. The `name` column acts as the primary key here, ensuring uniqueness for each tag's metadata entry. This table is typically memory-resident for fast lookups.
3.  **`_MYTAG_DATA_N` (Data Partition Tables):** Internal tables (where `N` ranges from 0 to `TAG_PARTITION_COUNT - 1`) that store the actual time-series data (`time`, `value`, additional data columns). Data is distributed across these partitions based on the tag `name`.
4.  **`V$MYTAG_STAT` (Statistics View):** A system view (if `TAG_STAT_ENABLE=1`) providing summary statistics (min/max time, min/max value, count, sum) for each tag, derived from the data partitions.

```
      << Internal Structure of MYTAG >>

+---------------------------------------------------+
|                  MYTAG (Virtual Table)            |
|  (Query Interface)                                |
+---------------------+-----------------------------+
                      |                             |
+---------------------v-----------------------------+ +-----------------------+
|            _MYTAG_META (Metadata Table)           | |   V$MYTAG_STAT        |
| +-------+-----------+-----------+-----+           | | (Statistics View)     |
| | _ID   | NAME      | factory   | equip |         | +-----------------------+
| +-------+-----------+-----------+-----+           |           ^
| | 1     | sensor-A  | fac1      | eq1   | <------lookup-----+
| | 2     | sensor-B  | fac1      | eq2   |         |           |
| | ...   | ...       | ...       | ...   |         |           | (Aggregated From)
+---------+-----------+-----------+-------+         |           |
       (Memory Resident Lookup)                     |           |
                                                    |           |
                      +-----------------------------+-----------+
                      | (Data distributed by hash(NAME))
                      |
        +-------------+-------------+ ... +-------------+
        |             |             |     |             |
+-------v-------+ +---v-----------+ +-----+-------------v---+
| _MYTAG_DATA_0 | | _MYTAG_DATA_1 | | ... | | _MYTAG_DATA_3 |
| +---+ T | V + | | +---+ T | V + | |     | | +---+ T | V + |
| | 0 |...|...| | | | 1 |...|...| | |     | | | 3 |...|...| |
| | 0 |...|...| | | | 1 |...|...| | |     | | |.. |...|...| |
| +---+---+---+ | | +---+---+---+ | |     | | +---+---+---+ |
+---------------+ +---------------+ +-----+ +---------------+
   (Data Partition) (Data Partition)         (Data Partition)
```

## Metadata Management in Tag Tables

### The Role of Metadata

Metadata provides essential context to raw time-series data points. By associating descriptive attributes (e.g., location, equipment type, manufacturer, unit of measure) with each tag (`name`), metadata enables:

*   **Structured Search:** Filtering and querying data based on characteristics rather than just cryptic tag names.
*   **Hierarchical Organization:** Representing relationships between sensors, equipment, locations, etc.
*   **Enhanced Analysis:** Grouping and aggregating data across meaningful categories defined by metadata.

**Conceptual Hierarchy Example:**

```
Company
├── city1 Plant
│   ├── Air Conditioner
│   │   ├── Tag (Current Sensor)
│   │   ├── Tag (Voltage Sensor)
│   │   └── ...
│   ├── Refrigerator
│   ├── Compressor
│   └── Crane
├── city2 Plant
│   ├── ... (similar structure)
└── city3 Plant
    └── ... (similar structure)
```

Each tag inherently possesses information about its context (e.g., plant, equipment).

**Example Use Cases:**

*   "Retrieve the last minute of data for all 'Current Sensors' associated with 'Cranes' in the 'Ulsan Plant'."
*   "Fetch all data from January 31st, 2022, between 11:00 and 12:00 for sensors whose names start with 'Current' belonging to 'Refrigerators'."
*   "Find the maximum value recorded last month for the tag named 'Current-3' across all equipment starting with 'Air Conditioner' in all plants."

### Defining and Utilizing Metadata Columns

Metadata columns are defined within the `METADATA` clause of the `CREATE TAG TABLE` statement.

```sql
CREATE TAG TABLE MYTAG (
    name VARCHAR(32) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
)
METADATA ( -- Define metadata columns here
    factory VARCHAR(32),
    equipment VARCHAR(64)
);
```

Metadata columns can also be added to an existing Tag Table's metadata structure using `ALTER TABLE` on the internal metadata table (`_tableName_META`).

```sql
ALTER TABLE _mytag_meta ADD COLUMN (line VARCHAR(16) DEFAULT 'op01');
```

Metadata resides in the `_tableName_META` table, which is typically kept in memory for efficient joining during queries against the main virtual Tag Table. The `name` column serves as the unique key linking metadata attributes to the time-series data.

```
       Metadata Area (_mytag_meta)             Data Area (_mytag_data_N)
+---------+----------+------------+----------+   +----+---------------------+-------+
| NAME    | factory  | equipment  | line     |   | ID | TIME                | VALUE |
+---------+----------+------------+----------+   +----+---------------------+-------+
| Sensor-A| Seoul    | drill      | op01     |   | 0  | ...                 | -1.3  | <= Data for Sensor-A
| Sensor-B| Seoul    | punch      | op01     |   | 1  | ...                 | -2.3  | <= Data for Sensor-B
| Sensor-C| Ulsan    | rolling    | op01     |   | 2  | ...                 | -3.3  | <= Data for Sensor-C
+---------+----------+------------+----------+   | ...| ...                 | ...   |
       (Unique entries per NAME)                      (Time-series measurements)
```

### Metadata Ingestion

Metadata for a new tag is typically provided during the initial data ingestion for that tag. When appending data, if the tag `name` does not already exist in the `_tableName_META` table, a new metadata record is created using the values supplied in that append operation.

**Important Consideration:** If a tag `name` already exists in the metadata table, subsequent data append operations for that tag **will not** update its existing metadata attributes. Metadata updates must be performed explicitly using the `UPDATE ... METADATA` command.

### Retrieving Data with Metadata Filters

Queries against the virtual Tag Table can include predicates (conditions) on both data columns (`time`, `value`, etc.) and metadata columns (`factory`, `equipment`, etc.). The database engine automatically joins the data partitions with the metadata table based on the tag `name`.

```sql
-- Retrieve data for a specific tag in a specific factory and equipment
-- within a given time range.
SELECT name, time, value, factory, equipment
FROM mytag
WHERE factory = 'Seoul'            -- Metadata filter
  AND equipment LIKE '%chill%'     -- Metadata filter (LIKE supported)
  AND name = 'tag-1'               -- Data/Tag identifier filter
  AND time BETWEEN TO_DATE('2022-01-01 00:00:00')
               AND TO_DATE('2022-12-31 23:59:59'); -- Time filter
```

### Modifying Metadata Entries

Existing metadata attributes for a specific tag can be modified using the `UPDATE ... METADATA SET` syntax.

```sql
UPDATE mytag METADATA SET equipment = 'chiller_unit_01', factory = 'Busan'
WHERE name = 'tag-existing'; -- MUST specify the target tag via 'name = ...'
```

**Constraints:**

*   The `WHERE` clause **must** contain an equality predicate on the `name` column (`WHERE name = 'specific_tag_name'`).
*   Other conditions in the `WHERE` clause are not permitted for metadata updates due to the key-value nature of the underlying metadata storage.
*   Bulk updates based on metadata attribute values are not directly supported via this command (future enhancements may address this).

### Deleting Metadata Entries

Metadata entries can be deleted using the `DELETE FROM ... METADATA` syntax.

```sql
DELETE FROM mytag METADATA WHERE name = 'tag_to_remove';
```

**Constraint:**

*   A metadata entry **cannot** be deleted if corresponding time-series data still exists for that tag in the data partitions.
*   Any associated time-series data must be deleted first using the standard `DELETE FROM table_name WHERE name = '...'` command before the metadata entry can be removed.

### Use Case: Dynamic Tag Categorization via Metadata

Metadata columns provide a powerful mechanism for dynamically classifying or annotating tags without altering the core data structure.

**Scenario:** Track tags that frequently generate errors or are used in specific reports.

1.  **Add an `alias` metadata column:**
    ```sql
    ALTER TABLE _basic_meta ADD COLUMN (alias VARCHAR(128) DEFAULT 'normal');
    ```

2.  **Update metadata for specific tags:**
    ```sql
    UPDATE basic METADATA SET alias = 'error' WHERE name = 'tag-2';
    UPDATE basic METADATA SET alias = 'report' WHERE name = 'tag-4';
    ```

3.  **Query data based on the dynamic category:**
    ```sql
    -- Find data for tags marked as 'error' within a specific time range
    SELECT * FROM basic
    WHERE alias = 'error'
      AND time BETWEEN '2022-01-01' AND '2022-12-31';

    -- Find data for tags marked for 'report'
    SELECT * FROM basic
    WHERE alias = 'report'
      AND time BETWEEN '2022-01-01' AND '2022-12-31';
    ```

### The Uniqueness and Usage of the `name` Column

The `name` column (or the column designated as `PRIMARY KEY` in the Tag Table definition) plays a critical role:

*   **Primary Key for Metadata:** It uniquely identifies each tag within the `_tableName_META` table, enabling CRUD (Create, Read, Update, Delete) operations on metadata attributes. Tag names must be unique.
*   **Link for Data Retrieval:** It connects the metadata attributes to the corresponding time-series data points during queries.
*   **Direct Data Filtering:** It allows direct selection or filtering of raw and aggregated data for specific tags.

**Tips for Constructing `name` Values:**

*   **Low Tag Cardinality (< 100 tags), No Metadata:** Use simple, human-readable unique strings (e.g., `'tag_001'`, `'temp_sensor_main'`). Direct querying by `name` is common.
*   **High Tag Cardinality (>> 1000 tags), Rich Metadata:** Direct querying by the full `name` might be less frequent. Constructing the `name` by concatenating key metadata fields (e.g., `'factoryA-equipmentX-sensorTypeZ-instance01'`) can ensure uniqueness and provide some context, but primary querying should leverage the dedicated metadata columns for filtering (e.g., `WHERE factory = 'factoryA' AND equipment = 'equipmentX'`). This approach scales better for discovery and filtering in large, complex systems.

## Tag Table Utilization

### Example Tag Table Design

Consider a manufacturing scenario tracking various sensor readings associated with specific production lots.

```sql
-- Tag table definition
CREATE TAG TABLE tag (
    name                   VARCHAR(100) PRIMARY KEY, -- Unique identifier, potentially combination of factory/equip/tag_id
    time                   DATETIME BASETIME,
    value                  DOUBLE SUMMARIZED,
    lot_no                 VARCHAR(32)             -- Additional data column specific to each reading
)
METADATA (
    factory_id             VARCHAR(16),             -- Metadata: Factory identifier
    equipment_id           VARCHAR(16),             -- Metadata: Equipment identifier
    tag_id                 VARCHAR(32)              -- Metadata: Base sensor identifier
);

-- Optional: Create an index on the additional data column for faster lookups by lot_no
CREATE INDEX idx_tag_lot_no ON tag (lot_no) INDEX_TYPE TAG;
```

**Example Queries:**

```sql
-- Retrieve all tag data for a specific factory
SELECT * FROM tag WHERE factory_id = 'fac01';

-- Retrieve data for a specific equipment within a specific factory
SELECT * FROM tag WHERE factory_id = 'fac01' AND equipment_id = 'equip01';

-- Retrieve specific columns for data associated with a particular production lot
SELECT name, time, value FROM tag WHERE lot_no = 'lot2001'; -- Uses idx_tag_lot_no if beneficial

-- Retrieve data for specific tags on specific equipment/factory within a time range
SELECT * FROM tag
WHERE factory_id = 'fac01'
  AND equipment_id = 'equip01'
  AND tag_id IN ('tag01', 'tag02', 'tag03') -- Filter using metadata tag_id
  AND time BETWEEN TO_DATE('2023-08-15 00:00:00') AND TO_DATE('2023-08-15 23:59:59');
```

### Basic Data Retrieval Operations

Standard SQL `SELECT` statements are used, leveraging the implicit indexing on `name` and `time`.

```sql
-- Get total record count
SELECT count(*) FROM tag;

-- Get overall time range of data
SELECT min(time), max(time) FROM tag;

-- Retrieve raw data for a specific tag within a time range (ordered chronologically)
SELECT time, value FROM tag
WHERE name = 'TAG_00001'
  AND time BETWEEN TO_DATE('2023-01-01') AND TO_DATE('2023-01-31');

-- Retrieve raw data for multiple specific tags, ordered reverse chronologically
SELECT /*+ SCAN_BACKWARD(tag) */ time, value FROM tag
WHERE name IN ('TAG1', 'TAG2')
  AND time BETWEEN TO_DATE('2023-01-01') AND TO_DATE('2023-01-31');
```

**Note:** Predicates on the `name` column generally support equality (`=`) and `IN` list comparisons efficiently.

### Complex Analytical Scenarios

Tag Tables, especially when combined with metadata and optional rollup features, enable sophisticated analyses.

**Example Scenario Setup:**

```sql
CREATE TAG TABLE MYTAG (
    name VARCHAR(32) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
)
METADATA (
    factory VARCHAR(32),
    equipment VARCHAR(64),
    alias VARCHAR(64) -- For dynamic tagging
)
WITH ROLLUP; -- Enable automatic time-based aggregation (details in Rollup documentation)
```

**Types of Queries:**

1.  **Raw Data Extraction:**
    *   All tag data from 'Seoul' factory for Feb 12, 2024.
    *   Data between 12:00 and 13:00 on Jul 12, 2024, for 'Compressors' in 'Seoul' factory.
    *   Data between 13:20 and 13:30 on Sep 13, 2024, for tags starting with 'Current' associated with 'Cooling Units' across all factories.
    *   Data from Dec 23 to Dec 29, 2024, for all tags marked with alias 'CriticalSensor'.

2.  **Statistical Data Extraction (using Rollup):**
    *   Monthly average `value` for all tags on 'Cranes' in 'Seoul' factory for the entire year 2024.
    *   Daily maximum `value` for tags containing 'Current' in their name in 'Cheongju' factory during June 2024.
    *   Monthly average `value` from 2020 to 2024 for all tags marked with alias 'CriticalSensor' across all factories.

3.  **Analysis Based on Statistical Data (using Rollup):**
    *   For all sensors containing 'Power' over the last 5 years, find the week and the corresponding maximum value recorded during that week.
    *   For all sensors containing 'Temperature' in 'Cheongju' factory over the last year, find the day(s) with the highest daily maximum temperature and the average temperature on those days.
    *   For sensors containing 'Pressure' across all factories over the last 3 months, identify the day(s) with the highest daily average pressure and the corresponding average value.

**Example Query (Hourly Average and Last Value):**

```sql
-- Get hourly average and last value for all tags in 'factory1' for a 12-hour period
SELECT
    name,
    ROLLUP('hour', 1, time) AS rollup_time, -- Aggregate time to the hour
    AVG(value) AS avg_value,
    LAST(time, value) AS last_value -- Get the last value within the hour
FROM mytag
WHERE name IN (SELECT name FROM _mytag_meta WHERE factory_id = 'factory1') -- Filter tags by metadata
  AND time BETWEEN TO_DATE('2000-01-01 00:00:00') AND TO_DATE('2000-01-01 11:59:59') -- Time range
GROUP BY name, rollup_time -- Group by tag and aggregated time interval
ORDER BY name, rollup_time;
```

### Data Model Transformation using PIVOT

The `PIVOT` clause allows transforming the tall/narrow Tag Table format back into a wide format, similar to the traditional model, for specific analysis or reporting needs.

```sql
-- Pivot selected tag values into columns based on time
SELECT *
FROM (
    -- Subquery selecting relevant data
    SELECT time, name, value -- Assuming name corresponds to tagid, value to dvalue
    FROM mytag
    WHERE time BETWEEN TO_DATE('2018-12-07 00:00:00') AND TO_DATE('2018-12-08 05:00:00')
      AND name IN ('FRONT_AXIS_TORQUE', 'REAR_AXIS_TORQUE', 'HOIST_AXIS_TORQUE', 'SLIDE_AXIS_TORQUE')
)
PIVOT (
    SUM(value) -- Aggregation function applied if multiple values exist for the same time/tag
    FOR name -- The column whose unique values become the new column headers
    IN ('FRONT_AXIS_TORQUE', 'REAR_AXIS_TORQUE', 'HOIST_AXIS_TORQUE', 'SLIDE_AXIS_TORQUE') -- List of tag names to pivot into columns
)
WHERE "FRONT_AXIS_TORQUE" >= 40 AND "REAR_AXIS_TORQUE" >= 20; -- Optional filtering on pivoted columns
```

**Example Output (Conceptual):**

```
time                          'FRONT_AXIS_TORQUE' 'REAR_AXIS_TORQUE' 'HOIST_AXIS_TORQUE' 'SLIDE_AXIS_TORQUE'
----------------------------- ------------------- ------------------ ------------------- -------------------
2018-12-07 16:42:29 840:000:000 12158               7244               NULL                NULL
2018-12-07 14:56:26 220:000:000 3308                663                NULL                NULL
...                           ...                 ...                ...                 ...
```
*(Note: Pivoted column names might need quoting if they match keywords or contain special characters).*

### Data Deletion Operations

Data deletion in Tag Tables is primarily time-based or tag-based. Point updates or deletions of individual records are generally not supported due to the append-optimized architecture.

**Deletion Syntax Examples:**

```sql
-- Delete all data BEFORE a specific timestamp across all tags
DELETE FROM table_name BEFORE TO_DATE('2023-01-15 00:00:00');

-- Delete ALL data from the table (use with extreme caution)
DELETE FROM table_name;

-- Delete all data for a SPECIFIC tag
DELETE FROM table_name WHERE name = 'TAG01';

-- Delete data for a SPECIFIC tag BEFORE a specific timestamp
DELETE FROM table_name WHERE name = 'TAG01' AND time < TO_DATE('2023-02-01 00:00:00');
```

## Indexing in Tag Tables

### Internal versus External Indexes

Tag Tables incorporate highly optimized **internal indexes** automatically created on the (`name`, `time`) columns. These indexes are fundamental to the performance of typical time-series queries.

*   **Query `WHERE name = '...'`:** Utilizes the internal index to efficiently locate all data for the specified tag, returned in chronological order.
*   **Query `WHERE time BETWEEN ... AND ...`:** Utilizes the internal index to scan data across all tags within the specified time range, returned in chronological order.
*   **Query `WHERE name = '...' AND time BETWEEN ... AND ...`:** Utilizes the internal index for highly efficient retrieval of data for a specific tag within a specific time range.
*   **Query `WHERE name = '...' AND time BETWEEN ... AND ... AND value > ...`:** Uses the internal index to find the relevant (`name`, `time`) data blocks, then applies the `value` filter to the retrieved data.

**Challenge:** Queries that filter *only* on `value` columns (or additional data columns) without a `name` or `time` constraint cannot effectively use the primary internal index.

*   **Query `WHERE name = '...' AND value > ...` (without time constraint):** This requires scanning *all* data blocks associated with `'tag-1'` to apply the `value` filter. Performance degrades proportionally to the total amount of data for that tag.

To address such scenarios, **external indexes** can be created.

### External Index Creation and Usage

External indexes can be explicitly created on `value` columns or other additional data columns to accelerate queries that filter primarily on these columns.

**Syntax:**

```sql
CREATE INDEX index_name ON table_name (column_name) [INDEX_TYPE TAG];
-- INDEX_TYPE TAG is specific for optimizing indexes on Tag Table data columns.
```

**Example:**

```sql
CREATE TAG TABLE mytag (
    name VARCHAR(100) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED,
    lot_no VARCHAR(32)
);

-- Create external indexes on 'value' and 'lot_no' columns
CREATE INDEX idx_mytag_value ON mytag(value) INDEX_TYPE TAG;
CREATE INDEX idx_mytag_lotno ON mytag(lot_no) INDEX_TYPE TAG;

-- This query can now potentially use the external index idx_mytag_value
SELECT * FROM mytag WHERE name = 'TAG-2' AND value > 33;

-- This query can potentially use the external index idx_mytag_lotno
SELECT * FROM mytag WHERE lot_no = 'LOTXYZ' AND time > TO_DATE('2024-01-01');
```

**Characteristics of External Indexes:**

*   **Asynchronous:** Index updates may lag slightly behind data ingestion. There can be a small time gap where newly ingested data is not yet reflected in the external index.
*   **Local Nature:** These indexes are typically partitioned locally alongside the data partitions. Query performance using external indexes may still exhibit some degradation as the overall data volume grows, although significantly better than a full scan without the index.
*   **Resource Consumption:** External indexes consume additional storage space and incur some overhead during data ingestion.

## Data Ingestion into Tag Tables

### Ingestion Methods Overview

Machbase Neo provides multiple pathways for ingesting data into Tag Tables, catering to different performance requirements and client environments.

```
+-------------------+      +-------------------+      +-------------------+
|    ODBC/JDBC/     |      |    MQTT/gRPC/     |      |    Machbase       |
|   .NET Clients    | ---> |   HTTP Clients    | ---> |     Native        | ---> Machbase Neo
+-------------------+      +-------------------+      |  CLI/SDK (C/Py)   |      (Tag Table)
                                                     +-------------------+
 (Standard SQL INSERT,      (REST API /append,       (High-Throughput
  or Append Protocol)        MQTT Subscription)        Append Protocol)
```

### Detailed Ingestion Approaches

1.  **SQL `INSERT` Statements:**
    *   Uses standard `INSERT INTO table_name VALUES (...)` syntax.
    *   Operates via request/response mechanism.
    *   Suitable for low-volume or infrequent insertions.
    *   **Not recommended** for high-throughput, high-volume time-series data due to performance limitations.

2.  **Append Protocol:**
    *   A specialized, high-performance Machbase protocol optimized for bulk data ingestion.
    *   Minimizes network overhead and server-side processing per record.
    *   Accessible via:
        *   **Machbase CLI (Command Line Interface):** Utilities for bulk loading from files.
        *   **ODBC/JDBC/.NET:** Extended APIs provided by Machbase drivers enable Append operations.
        *   **C/C++/Go SDKs:** Native libraries offer direct access to the Append API for maximum performance.
        *   **Python (`machbaseAPI`):** Wrapper library providing access to Append functionality.
    *   **Recommended** for most time-series ingestion scenarios requiring high throughput.

3.  **REST API:**
    *   Machbase Neo exposes HTTP endpoints for data interaction.
    *   The data loading endpoint supports an `append` method parameter, which utilizes the efficient Append protocol internally.
    *   Suitable for web-based clients or systems integrating via HTTP.

4.  **Other Languages (Python, Go, R):**
    *   Typically leverage wrappers around the CLI or ODBC/Native SDKs to utilize the efficient Append protocol.

**Performance Note:** For demanding use cases like high-frequency vibration data (requiring hundreds of thousands to millions of inserts per second), utilizing the native C/C++ SDK with the Append API is often necessary to achieve peak ingestion rates.

## Operational Considerations

### Key Usage Precautions

*   **Memory Consumption:** Each Tag Table consumes a baseline amount of memory related to its partitions (`TAG_PARTITION_COUNT`) and data buffers (`TAG_DATA_PART_SIZE`). Creating numerous Tag Tables can significantly impact overall server memory usage. Plan table creation considering available resources.
*   **Query Performance:** `SELECT` queries lacking predicates on indexed columns (`name`, `time`, or columns with external indexes) will result in full table scans or large partial scans, leading to performance degradation proportional to data volume. Always include `name` and/or `time` range filters where possible.
*   **External Indexes:** Only create external indexes on data/value columns if queries frequently filter *solely* on those columns without time constraints. They add storage and ingestion overhead.
*   **Data Immutability:** Tag Tables are designed for append-only data. Updates to existing data records are not supported. Deletion is primarily time-based or whole-tag based.
*   **Ingestion Method:** Select the appropriate ingestion method based on performance requirements. Use the Append protocol (via SDKs, CLI, drivers, or REST API `append` method) for high-volume data.

### Memory Consumption Considerations

The memory footprint of a Tag Table is influenced by several factors:

*   **Ingestion Buffers:** Proportional to `TAG_DATA_PART_SIZE` (default 16MB). Multiple buffers are used internally.
*   **Number of Partitions:** `TAG_PARTITION_COUNT` (default 4). Each partition maintains its own buffers and index structures.
*   **Index Space:** Dynamically allocated based on the volume and cardinality of data within each partition. Roughly related to `TAG_DATA_PART_SIZE` and average row size.

**Approximate Memory Formula per Table:**

`Memory ≈ (TAG_DATA_PART_SIZE * BufferFactor) + ((IndexSizeFactor * TAG_DATA_PART_SIZE / AvgRowSize) * IndexOverheadFactor) * TAG_PARTITION_COUNT`

*(Internal factors and dynamic allocation make precise calculation complex, but this illustrates the key drivers).*

With default settings (`TAG_PARTITION_COUNT=4`, `TAG_DATA_PART_SIZE=16MB`), a Tag Table can dynamically consume roughly **up to 4 GB** of memory (approx. 1GB per partition) under load, primarily for indexing and buffering.

**Managing Memory Usage:**

*   **Reduce `TAG_PARTITION_COUNT`:** Lowering the partition count (e.g., to 1 or 2) directly reduces the parallelism factor and associated memory. This can be adjusted dynamically via `ALTER TABLE` properties. Suitable for resource-constrained environments but may impact peak concurrent performance.
*   **Tune `TAG_DATA_PART_SIZE`:** Reducing this property (e.g., to 4MB or 8MB, must be >= 1MB) via server configuration reduces the size of internal buffers and index segments, lowering memory pressure. This requires a server restart to take effect.

## Summary

The Machbase Tag Table is a specialized database object engineered for the efficient management of time-series sensor data. Key characteristics include:

*   **Optimized Structure:** Employs a tall/narrow data model ([identifier, time, value]) ideal for sensor readings.
*   **Metadata/Data Separation:** Decouples descriptive attributes (metadata) from raw time-series measurements (data), allowing flexible metadata management and efficient data storage.
*   **Metadata Management:** Metadata is associated via a unique tag `name` (primary key) and supports flexible querying, addition, modification, and deletion (contingent on data deletion).
*   **Data Operations:** Optimized for high-speed append operations. Retrieval is highly efficient when filtering by tag `name` and/or `time`. Data updates are not supported; deletion is primarily time-horizon or tag-based.
*   **Extensibility:** Both metadata and data areas can be extended with additional columns to store richer contextual or measurement information.
*   **Performance:** Leverages internal partitioning and specialized indexing for high ingestion throughput and fast time-based query performance.

The Tag Table provides a robust and performant foundation for building scalable time-series applications within the Machbase ecosystem.