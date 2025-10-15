# Inserting Tag Data

## Overview

Machbase provides multiple methods to insert tag data, each optimized for different use cases. Choose the method that best fits your data volume and application requirements.

## Method 1: INSERT Statement

The simplest way to insert data - ideal for small datasets and testing.

### Basic INSERT Example

```sql
Mach> create tag table TAG (name varchar(20) primary key, time datetime basetime, value double summarized);
Executed successfully.

Mach> insert into tag metadata values ('TAG_0001');
1 row(s) inserted.

-- Insert single values
Mach> insert into tag values('TAG_0001', now, 0);
1 row(s) inserted.

Mach> insert into tag values('TAG_0001', now, 1);
1 row(s) inserted.

Mach> insert into tag values('TAG_0001', now, 2);
1 row(s) inserted.

Mach> select * from tag where name = 'TAG_0001';
NAME                  TIME                            VALUE
--------------------------------------------------------------------------------------
TAG_0001              2018-12-19 17:41:37 806:901:728 0
TAG_0001              2018-12-19 17:41:42 327:839:368 1
TAG_0001              2018-12-19 17:41:43 812:782:202 2
[3] row(s) selected.
```

> **When to use**: Testing, low-volume inserts, interactive data entry

## Method 2: CSV File Import

Load large amounts of data quickly from CSV files using the `csvimport` tool.

### CSV File Format

Create a CSV file (`data.csv`) with tag name, timestamp, and value:

```csv
TAG_0001, 2009-01-28 07:03:34 0:000:000, -41.98
TAG_0001, 2009-01-28 07:03:34 1:000:000, -46.50
TAG_0001, 2009-01-28 07:03:34 2:000:000, -36.16
```

### Using csvimport

```bash
csvimport -t TAG -d data.csv -F "time YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn" -l error.log
```

**Options explained**:
- `-t TAG`: Target table name
- `-d data.csv`: Data file path
- `-F`: Time format specification
- `-l error.log`: Error log file

> **When to use**: Bulk loading, data migration, batch imports

> **Important**: Tag names must exist in tag metadata before importing data.

## Method 3: RESTful API

Insert data via HTTP requests - perfect for IoT devices and web applications.

### API Syntax

```json
{
  "values": [
    [TAG_NAME, TAG_TIME, VALUE],
    [TAG_NAME, TAG_TIME, VALUE],
    ...
  ],
  "date_format": "YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn"
}
```

If `date_format` is omitted, the default format `YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn` is used.

### API Example

```bash
curl -X POST http://localhost:5654/db/write/TAG \
  -H "Content-Type: application/json" \
  -d '{
    "values": [
      ["TAG_0001", "2024-01-01 10:00:00", 25.5],
      ["TAG_0001", "2024-01-01 10:01:00", 26.0],
      ["TAG_0002", "2024-01-01 10:00:00", 30.2]
    ]
  }'
```

> **When to use**: IoT devices, real-time data streaming, web applications

## Method 4: SDK Integration

Use Machbase SDKs for programmatic data insertion from your applications.

### Supported Languages

- **[C/C++ library](/dbms/sdk/cli-odbc)** - High-performance native integration
- **[Java library](/dbms/sdk/jdbc)** - Enterprise Java applications
- **[Python library](/dbms/sdk/python)** - Data science and automation
- **[C# library](/dbms/sdk/dotnet)** - .NET applications

### Python Example

```python
import machbaseAPI as mach

# Connect to Machbase
conn = mach.connect(host='localhost', port=5656)

# Insert data
cursor = conn.cursor()
cursor.execute("""
    INSERT INTO tag VALUES (?, ?, ?)
""", ('TAG_0001', '2024-01-01 10:00:00', 25.5))

conn.commit()
conn.close()
```

> **When to use**: Application integration, automated data collection, custom tools

## Choosing the Right Method

| Method | Best For | Pros | Cons |
|--------|----------|------|------|
| **INSERT** | Testing, small datasets | Simple, interactive | Slow for large data |
| **CSV Import** | Bulk loading, migration | Very fast, efficient | Requires file preparation |
| **RESTful API** | IoT, web apps | Flexible, platform-independent | Network overhead |
| **SDK** | Applications | Full control, type-safe | Requires development |

## Working with Additional Columns

If your tag table has additional columns, include them in your insert:

```sql
-- Tag table with additional columns
CREATE TAG TABLE sensors (
    name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE,
    location VARCHAR(50),
    status SHORT
);

-- Insert with additional columns
INSERT INTO sensors VALUES (
    'TEMP_001',
    '2024-01-01 10:00:00',
    25.5,
    'Building A',
    1
);
```

## Best Practices

1. **Register Tags First**: Always insert tag metadata before inserting data
2. **Use Batch Operations**: For large datasets, use CSV import or batch API calls
3. **Handle Errors**: Always check return values and log errors
4. **Time Precision**: Be consistent with timestamp precision across your data
5. **Validate Data**: Ensure tag names exist before insertion to avoid errors

## Performance Tips

- **CSV Import**: Fastest for bulk data (millions of rows)
- **Batch Inserts**: Group multiple INSERT statements in transactions
- **Parallel Loading**: Use multiple csvimport processes for parallel ingestion
- **Prepared Statements**: Use parameterized queries in SDKs for better performance

## Next Steps

- Learn about [Querying Tag Data](../querying-data) for data retrieval
- Explore [Tag Indexes](../tag-indexes) for query optimization
- Understand [Rollup Tables](../rollup-tables) for time-series aggregation
