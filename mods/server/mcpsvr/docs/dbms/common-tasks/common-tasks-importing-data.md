# Importing Data

Learn how to efficiently load data into Machbase using various methods: CSV import, bulk loading, and real-time ingestion. Choose the right method for your data volume and requirements.

## Import Methods Overview

| Method | Best For | Speed | Complexity |
|--------|----------|-------|------------|
| machloader (CSV) | One-time imports, files | Fast | Easy |
| APPEND API | High-volume continuous | Fastest | Medium |
| INSERT statements | Small datasets, testing | Slow | Easy |
| REST API | Web applications | Medium | Easy |

## CSV Import with machloader

### Basic CSV Import

```bash
# Import CSV file
machloader -t tablename -d csv -i data.csv

# With options
machloader -t sensors -d csv -i sensor_data.csv -F ','
```

### CSV File Format

```csv
sensor01,2025-10-10 14:00:00,25.3,55.2
sensor01,2025-10-10 14:00:01,25.4,55.1
sensor02,2025-10-10 14:00:00,22.1,60.5
```

**Important**: CSV columns must match table schema order.

### machloader Options

```bash
machloader \
  -s 127.0.0.1          # Server address
  -u SYS                # Username
  -p MANAGER            # Password
  -P 5656               # Port
  -t tablename          # Table name
  -d csv                # Data format
  -i data.csv           # Input file
  -F ','                # Field separator (default: comma)
  -R '\n'               # Record separator (default: newline)
  -e error.log          # Error log file
  -b 10000              # Batch size (rows per commit)
```

### Complete Example

```bash
# Create sample CSV
cat > sensors.csv <<EOF
sensor01,2025-10-10 14:00:00,25.3
sensor01,2025-10-10 14:00:01,25.4
sensor02,2025-10-10 14:00:00,22.1
EOF

# Create table
machsql -f - <<EOF
CREATE TABLE sensor_data (
    sensor_id VARCHAR(20),
    timestamp DATETIME,
    value DOUBLE
);
EOF

# Import CSV
machloader -t sensor_data -d csv -i sensors.csv

# Verify
machsql -f - <<EOF
SELECT COUNT(*) FROM sensor_data;
EOF
```

### Large File Import

```bash
# For large files, increase batch size
machloader -t sensors -d csv -i large_data.csv -b 50000

# Monitor progress
tail -f $MACHBASE_HOME/trc/machloader.log
```

## APPEND Protocol (Bulk Insert API)

### Why APPEND?

- **Fastest** method (millions of rows/second)
- **Batch commits** for efficiency
- **Non-blocking** writes
- **Best for** high-volume continuous data

### C/CLI Example

```c
#include "machbase_cli.h"

int main() {
    SQLHENV env;
    SQLHDBC conn;
    SQLHSTMT stmt;

    // Connect (omitted for brevity)...

    // Allocate statement
    SQLAllocStmt(conn, &stmt);

    // Open APPEND
    SQLAppendOpen(stmt, "sensor_data");

    // Append rows
    for (int i = 0; i < 100000; i++) {
        char sensor_id[20];
        SQL_TIMESTAMP_STRUCT time_val;
        double value;

        sprintf(sensor_id, "sensor%02d", i % 10);
        // ... set time_val and value

        SQLAppendDataV(stmt, sensor_id, &time_val, value);
    }

    // Close (commit batch)
    SQLAppendClose(stmt);

    // Cleanup...
    return 0;
}
```

### Java JDBC Example

```java
import com.machbase.jdbc.*;

Connection conn = DriverManager.getConnection(url, user, password);

// Create Appender
Appender appender = ((MachConnection)conn).createAppender("sensor_data");

// Append rows
for (int i = 0; i < 100000; i++) {
    appender.append(
        "sensor" + (i % 10),
        new Timestamp(System.currentTimeMillis()),
        25.3 + Math.random()
    );
}

// Commit
appender.close();
conn.close();
```

### Python Example

```python
import machbase

conn = machbase.connect('127.0.0.1', 5656, 'SYS', 'MANAGER')

# Create appender
appender = conn.create_appender('sensor_data')

# Append rows
for i in range(100000):
    appender.append(
        f'sensor{i % 10:02d}',
        '2025-10-10 14:00:00',
        25.3 + i * 0.1
    )

# Commit
appender.close()
conn.close()
```

## Regular INSERT Statements

### Single Insert

```sql
INSERT INTO sensor_data VALUES ('sensor01', NOW, 25.3);
```

### Batch Insert

```sql
-- Not recommended for large datasets
INSERT INTO sensor_data VALUES ('sensor01', NOW, 25.3);
INSERT INTO sensor_data VALUES ('sensor02', NOW, 22.1);
INSERT INTO sensor_data VALUES ('sensor03', NOW, 23.5);
-- Use APPEND API instead for >1000 rows
```

### Parameterized Insert

```python
cur = conn.cursor()

# Single parameterized insert
cur.execute("INSERT INTO sensor_data VALUES (?, ?, ?)",
            ('sensor01', '2025-10-10 14:00:00', 25.3))

# Batch parameterized insert
data = [
    ('sensor01', '2025-10-10 14:00:01', 25.4),
    ('sensor02', '2025-10-10 14:00:01', 22.1),
    ('sensor03', '2025-10-10 14:00:01', 23.5)
]
cur.executemany("INSERT INTO sensor_data VALUES (?, ?, ?)", data)

conn.commit()
```

## Tag Table Import

### CSV for Tag Table

```csv
sensor01,2025-10-10 14:00:00,25.3,55.2
sensor01,2025-10-10 14:00:01,25.4,55.1
sensor02,2025-10-10 14:00:00,22.1,60.5
```

```bash
machloader -t warehouse_sensors -d csv -i tag_data.csv
```

### APPEND for Tag Table

```c
SQLAppendOpen(stmt, "warehouse_sensors");

SQLAppendDataV(stmt,
    "sensor01",                    // Tag name
    &time_val,                     // BASETIME
    25.3,                          // temperature
    55.2);                         // humidity

SQLAppendClose(stmt);
```

## Error Handling

### Validate Data Before Import

```bash
# Check CSV format
head -10 data.csv

# Count rows
wc -l data.csv

# Check for invalid characters
file data.csv
```

### Handle Import Errors

```bash
# Capture errors
machloader -t sensors -d csv -i data.csv -e errors.log 2>&1 | tee import.log

# Check error log
cat errors.log

# Retry failed rows
# (Extract failed rows from error log and re-import)
```

### Common Issues

**Issue 1: Column count mismatch**
```
Error: Column count mismatch
```
**Solution**: Ensure CSV columns match table schema

**Issue 2: Data type mismatch**
```
Error: Invalid data type
```
**Solution**: Validate data types in CSV

**Issue 3: Duplicate primary keys** (Volatile/Lookup tables)
```
Error: Duplicate key
```
**Solution**: Remove duplicates or use UPDATE instead

## Performance Optimization

### 1. Use APPEND for Large Datasets

```
< 1,000 rows      → INSERT statements
1,000-100,000     → machloader (CSV)
> 100,000         → APPEND API
Continuous stream → APPEND API
```

### 2. Batch Size Tuning

```bash
# Small batches (safer)
machloader -t sensors -d csv -i data.csv -b 1000

# Large batches (faster)
machloader -t sensors -d csv -i data.csv -b 100000
```

### 3. Parallel Loading

```bash
# Split large file
split -l 1000000 large_data.csv chunk_

# Load in parallel
machloader -t sensors -d csv -i chunk_aa &
machloader -t sensors -d csv -i chunk_ab &
machloader -t sensors -d csv -i chunk_ac &
wait
```

### 4. Disable Indexing During Bulk Load

```sql
-- For Log/Lookup tables with indexes
-- Drop indexes before import
DROP INDEX idx_sensor_id;

-- Import data
-- (use machloader or APPEND)

-- Recreate indexes after import
CREATE INDEX idx_sensor_id ON sensors(sensor_id);
```

## Data Validation

### Pre-Import Checks

```sql
-- Check table schema
SHOW TABLE sensor_data;

-- Check table exists
SHOW TABLES;

-- Verify column types match CSV data
```

### Post-Import Validation

```sql
-- Count imported rows
SELECT COUNT(*) FROM sensor_data;

-- Check for NULL values
SELECT COUNT(*) FROM sensor_data WHERE value IS NULL;

-- Verify time range
SELECT MIN(_arrival_time), MAX(_arrival_time) FROM sensor_data;

-- Check data distribution
SELECT sensor_id, COUNT(*) FROM sensor_data GROUP BY sensor_id;
```

## Real-World Examples

### Example 1: Daily Log Import

```bash
#!/bin/bash
# daily_import.sh

DATE=$(date +%Y%m%d)
LOG_FILE="/logs/app_${DATE}.log"
TABLE="app_logs"

# Convert log to CSV
awk '{print $1","$2","$3}' $LOG_FILE > /tmp/import.csv

# Import
machloader -t $TABLE -d csv -i /tmp/import.csv -e /tmp/errors_${DATE}.log

# Check results
echo "Imported $(machsql -i -f - <<< "SELECT COUNT(*) FROM $TABLE WHERE _arrival_time >= SYSDATE - 1;" | tail -1) rows"

# Cleanup
rm /tmp/import.csv
```

### Example 2: Real-Time Sensor Stream

```python
import machbase
import time

conn = machbase.connect('127.0.0.1', 5656, 'SYS', 'MANAGER')

while True:
    # Create appender for batch
    appender = conn.create_appender('sensors')

    # Collect 1000 readings
    for i in range(1000):
        sensor_id = f'sensor{i % 100:03d}'
        value = read_sensor(sensor_id)  # Your sensor reading function
        appender.append(sensor_id, time.time(), value)

    # Commit batch
    appender.close()

    # Wait before next batch
    time.sleep(10)
```

### Example 3: CSV File Monitoring

```bash
#!/bin/bash
# monitor_csv.sh - Import new CSV files

WATCH_DIR="/data/csv"
ARCHIVE_DIR="/data/archive"
TABLE="sensor_data"

inotifywait -m -e create "$WATCH_DIR" --format '%f' | while read FILE; do
    if [[ $FILE == *.csv ]]; then
        echo "Importing $FILE..."
        machloader -t $TABLE -d csv -i "$WATCH_DIR/$FILE"

        if [ $? -eq 0 ]; then
            mv "$WATCH_DIR/$FILE" "$ARCHIVE_DIR/"
            echo "Success: $FILE"
        else
            echo "Error: $FILE" | mail -s "Import Error" admin@company.com
        fi
    fi
done
```

## Best Practices

1. **Use APPEND API** for high-volume continuous data
2. **Use machloader** for one-time CSV imports
3. **Batch operations** for better performance
4. **Validate data** before import
5. **Monitor errors** and retry failed rows
6. **Parallelize** large imports when possible
7. **Test with small dataset** first

## Troubleshooting

**Slow import speed**:
- Increase batch size
- Use APPEND instead of INSERT
- Check network latency
- Verify server resources (CPU, disk I/O)

**Out of memory**:
- Reduce batch size
- Split large files
- Check server memory settings

**Connection timeout**:
- Increase connection timeout
- Check network stability
- Verify server load

## Next Steps

- **Query Data**: [Querying Data](../querying/) - Query imported data
- **User Management**: [User Management](../user-management/) - Permissions for import users
- **Tutorials**: [IoT Sensor Data](../../tutorials/iot-sensor-data/) - Complete import example

---

Master data import and efficiently load your time-series data into Machbase!
