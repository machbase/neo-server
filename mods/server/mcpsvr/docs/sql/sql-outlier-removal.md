# Machbase Neo SQL Automatic Outlier Removal

## Introduction

Time-series data originating from physical sensors, particularly in industrial or environmental settings, is frequently susceptible to noise, transient spikes, vibrational interference, or other anomalous readings. These outliers, while potentially numerous, often represent deviations from the expected operational range and can significantly impede data analysis, consume storage resources unnecessarily, and increase processing time.

Manual or application-level filtering of such outliers can be complex and computationally expensive. Machbase provides an integrated mechanism for automatic outlier removal during data ingestion by leveraging Specification Limits defined within the Tag Metadata associated with TAG tables. This allows users to declaratively define valid operational ranges for specific sensors, ensuring that only data points falling within these bounds are persisted.

## Core Concept: Specification Limits (LSL/USL)

The Automatic Outlier Removal feature operates by defining bounds on the expected values for a given Tag Identifier (Tag ID). These bounds are established using two special attributes within the Tag Metadata table:

*   **LSL (Lower Specification Limit):** Defines the minimum acceptable value for a sensor reading associated with a specific Tag ID.
*   **USL (Upper Specification Limit):** Defines the maximum acceptable value for a sensor reading associated with a specific Tag ID.

When a new data row is being inserted into the main TAG table, Machbase performs the following validation against the LSL and USL values defined in the corresponding Tag ID's metadata entry:

1.  **Metadata Lookup:** The system retrieves the LSL and USL values associated with the `name` (Tag ID) of the incoming row from the dependent Tag Metadata table (`_TableName_meta`).
2.  **Value Comparison:** The value being inserted into the designated `value` column (the one marked with `SUMMARIZED`) is compared against the retrieved LSL and USL.
3.  **Validation Rule:** The insertion is permitted **only if** the incoming value satisfies the condition: `LSL <= incoming_value <= USL`.
    *   If LSL is `NULL`, the lower bound check is skipped (`incoming_value <= USL`).
    *   If USL is `NULL`, the upper bound check is skipped (`LSL <= incoming_value`).
    *   If both LSL and USL are `NULL`, no validation occurs, and the value is accepted.
4.  **Action:** If the validation succeeds, the row is inserted into the TAG table. If the validation fails (the value falls outside the defined LSL/USL range), the insertion operation for that specific row is rejected, and an appropriate error is typically returned (e.g., `ERR-02342` for value < LSL, `ERR-02341` for value > USL).

This mechanism effectively filters incoming data based on pre-defined acceptable ranges specific to each tag, directly at the ingestion point.

## Configuration

Specification Limits (LSL/USL) are configured by defining specific columns with special keywords within the `METADATA` section of a `CREATE TAG TABLE` statement, or by adding such columns later using `ALTER TABLE`.

### Defining Limits during Table Creation

Columns intended to hold the LSL and USL values are defined within the `METADATA` clause, using the `LOWER LIMIT` and `UPPER LIMIT` keywords respectively.

**Syntax:**

```sql
CREATE TAG TABLE table_name (
    name_column VARCHAR(...) PRIMARY KEY,
    time_column DATETIME BASETIME,
    value_column numeric_datatype SUMMARIZED, -- Crucial: SUMMARIZED is required
    ...
)
METADATA (
    lsl_column_name numeric_datatype LOWER LIMIT, -- Column for LSL
    usl_column_name numeric_datatype UPPER LIMIT, -- Column for USL
    ... -- Other metadata columns
);
```

*   `value_column`: Must be a numeric type and **must** include the `SUMMARIZED` keyword. Outlier validation applies specifically to values inserted into this column.
*   `lsl_column_name`, `usl_column_name`: User-chosen names for the metadata columns storing the limits.
*   `numeric_datatype`: The data type for the LSL/USL columns must be compatible with the `value_column`'s data type.

**Example (Both LSL and USL):**

```sql
CREATE TAG TABLE sensor_readings (
    tag_id VARCHAR(50) PRIMARY KEY,
    ts DATETIME BASETIME,
    reading DOUBLE SUMMARIZED
)
METADATA (
    min_acceptable DOUBLE LOWER LIMIT,
    max_acceptable DOUBLE UPPER LIMIT,
    location VARCHAR(100) -- Regular metadata column
);
```

**Example (Only LSL):**

It is permissible to define only one limit if validation is only required against a minimum or maximum threshold.

```sql
CREATE TAG TABLE pressure_monitor (
    tag_id VARCHAR(50) PRIMARY KEY,
    event_time DATETIME BASETIME,
    pressure_kpa INTEGER SUMMARIZED
)
METADATA (
    min_pressure INTEGER LOWER LIMIT -- Only validate against a minimum pressure
);
```

### Adding Limits to an Existing Table

LSL/USL columns can be added to the metadata definition of an existing TAG table using `ALTER TABLE` on the dependent metadata table (`_TableName_meta`). Note that `DROP COLUMN` is **not** supported for metadata tables.

**Syntax:**

```sql
-- Adding an LSL column
ALTER TABLE _table_name_meta ADD COLUMN ( lsl_column_name numeric_datatype LOWER LIMIT );

-- Adding a USL column
ALTER TABLE _table_name_meta ADD COLUMN ( usl_column_name numeric_datatype UPPER LIMIT );
```

**Example:**

```sql
-- Assume 'sensor_readings' table exists without LSL/USL initially
ALTER TABLE _sensor_readings_meta ADD COLUMN ( min_acceptable DOUBLE LOWER LIMIT );
ALTER TABLE _sensor_readings_meta ADD COLUMN ( max_acceptable DOUBLE UPPER LIMIT );
```

When added via `ALTER TABLE`, these columns will initially have `NULL` values for all existing metadata rows.

### Setting Limit Values

Once the LSL/USL columns are defined, the actual limit values for each Tag ID are set by inserting or updating rows in the dependent Tag Metadata table (`_TableName_meta`).

```sql
-- Set limits when inserting a new tag's metadata
INSERT INTO sensor_readings metadata (tag_id, min_acceptable, max_acceptable, location)
VALUES ('TEMP_SENSOR_01', 10.0, 90.0, 'Boiler Room');

-- Update limits for an existing tag
UPDATE sensor_readings metadata
SET min_acceptable = 15.0, max_acceptable = 85.0
WHERE tag_id = 'TEMP_SENSOR_01';
```

## Behavior and Constraints

*   **`SUMMARIZED` Requirement:** The Automatic Outlier Removal feature **mandates** that the target `value` column in the TAG table definition includes the `SUMMARIZED` keyword. Validation is performed exclusively against values inserted into this specific column.
*   **Data Type Compatibility:** The data types of the metadata columns designated as `LOWER LIMIT` and `UPPER LIMIT` must be numerically compatible with the data type of the `SUMMARIZED` value column in the main TAG table.
*   **LSL <= USL:** When both LSL and USL are defined and have non-`NULL` values for a specific Tag ID, the LSL value must be less than or equal to the USL value (`LSL <= USL`).
*   **Scope of Validation:** Validation occurs **only** during the `INSERT` operation into the main TAG table. It does not apply retroactively to data already present in the table before the LSL/USL limits were defined or updated in the metadata.
*   **Metadata Updates:** Updating LSL/USL values in the metadata table changes the validation rules for *subsequent* inserts but does **not** trigger a re-validation or removal of existing data in the main TAG table that might now fall outside the new limits.
*   **NULL Handling:** If the LSL value for a Tag ID is `NULL` in the metadata, the lower bound check is bypassed for incoming data for that tag. Similarly, if the USL value is `NULL`, the upper bound check is bypassed. If both are `NULL`, no outlier validation is performed for that Tag ID.
*   **Partial Limit Usage:** Defining only an LSL column enforces a minimum value check (`value >= LSL`). Defining only a USL column enforces a maximum value check (`value <= USL`).
*   **Metadata Table Dependency:** The feature relies entirely on the structure and content of the dependent Tag Metadata table (`_TableName_meta`).

## Examples

This section provides practical examples of configuring and utilizing the Automatic Outlier Removal feature.

**1. Schema Definition with LSL/USL:**

```sql
-- Drop if exists from previous runs
DROP TABLE IF EXISTS out_tag CASCADE;

-- Create TAG table with metadata for LSL and USL
CREATE TAG TABLE out_tag (
    tag_id VARCHAR(50) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED -- Value column where filtering applies
)
METADATA (
    lsl DOUBLE LOWER LIMIT, -- Lower Specification Limit column
    usl DOUBLE UPPER LIMIT  -- Upper Specification Limit column
) TAG_PARTITION_COUNT=1;
```

**2. Defining Limits in Metadata:**

```sql
-- Set the operational range for TAG_01: 100.0 <= value <= 200.0
INSERT INTO out_tag metadata (tag_id, lsl, usl) VALUES ('TAG_01', 100.0, 200.0);

-- Verify metadata entry
SELECT * FROM _out_tag_meta WHERE tag_id = 'TAG_01';
/* Expected Output:
_ID | TAG_ID | LSL   | USL
--- | ------ | ----- | -----
1   | TAG_01 | 100.0 | 200.0
*/
```

**3. Inserting Data and Observing Filtering:**

```sql
-- Attempt to insert data points for TAG_01

-- Value below LSL (Rejected)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 95.2);
-- Expected Error: [ERR-02342: SUMMARIZED value is less than LOWER LIMIT.]

-- Value equal to LSL (Accepted)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 100.0);
-- Expected Output: 1 row(s) inserted.

-- Value within range (Accepted)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 150.5);
-- Expected Output: 1 row(s) inserted.

-- Value equal to USL (Accepted)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 200.0);
-- Expected Output: 1 row(s) inserted.

-- Value above USL (Rejected)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 205.5);
-- Expected Error: [ERR-02341: SUMMARIZED value is greater than UPPER LIMIT.]

-- Verify accepted data
SELECT * FROM out_tag WHERE tag_id = 'TAG_01';
/* Expected Output: (Timestamps will vary)
TAG_ID | TIME                              | VALUE | LSL   | USL
------ | --------------------------------- | ----- | ----- | -----
TAG_01 | 2024-XX-XX XX:XX:XX XXX:XXX:XXX | 100.0 | 100.0 | 200.0
TAG_01 | 2024-XX-XX XX:XX:XX XXX:XXX:XXX | 150.5 | 100.0 | 200.0
TAG_01 | 2024-XX-XX XX:XX:XX XXX:XXX:XXX | 200.0 | 100.0 | 200.0
*/
```

**4. Updating Limits in Metadata:**

```sql
-- Change the limits for TAG_01 to 10.0 <= value <= 100.0
UPDATE out_tag metadata SET lsl = 10.0, usl = 100.0 WHERE tag_id = 'TAG_01';

-- Verify the change in metadata
SELECT * FROM _out_tag_meta WHERE tag_id = 'TAG_01';
/* Expected Output:
_ID | TAG_ID | LSL  | USL
--- | ------ | ---- | -----
1   | TAG_01 | 10.0 | 100.0
*/

-- Attempt new insertions based on updated limits

-- Value 150.5 (Accepted previously) is now above the new USL (Rejected)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 150.5);
-- Expected Error: [ERR-02341: SUMMARIZED value is greater than UPPER LIMIT.]

-- Value 95.2 (Rejected previously) is now within the new range (Accepted)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 95.2);
-- Expected Output: 1 row(s) inserted.

-- Note: The previously inserted values (100.0, 150.5, 200.0) remain in the 'out_tag' table.
-- The update to metadata limits does not affect existing data.
```

**5. Disabling Filtering using NULL:**

```sql
-- Disable outlier filtering for TAG_01 by setting limits to NULL
UPDATE out_tag metadata SET lsl = NULL, usl = NULL WHERE tag_id = 'TAG_01';

-- Verify metadata
SELECT * FROM _out_tag_meta WHERE tag_id = 'TAG_01';
/* Expected Output:
_ID | TAG_ID | LSL  | USL
--- | ------ | ---- | ----
1   | TAG_01 | NULL | NULL
*/

-- Insert values previously rejected (These should now succeed)
INSERT INTO out_tag VALUES ('TAG_01', NOW, 9.0);   -- Below previous LSL
-- Expected Output: 1 row(s) inserted.
INSERT INTO out_tag VALUES ('TAG_01', NOW, 250.0); -- Above previous USL
-- Expected Output: 1 row(s) inserted.
```