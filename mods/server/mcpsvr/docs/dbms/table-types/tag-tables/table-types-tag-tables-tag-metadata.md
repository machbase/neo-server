# Managing Tag Metadata

## Understanding Tag Metadata

Tag metadata represents the identity and additional information of sensors or data sources in Machbase. Think of it as a registry of all your sensors - each tag has a unique name and can have associated descriptive information.

## Basic Tag Metadata Operations

### Creating Simple Tag Metadata

When you create a tag table, you define the structure. To actually use it, you need to register tag names:

```sql
-- Create the tag table first
create tag table TAG (name varchar(20) primary key, time datetime basetime, value double summarized);

Mach> desc tag;
[ COLUMN ]
----------------------------------------------------------------
NAME                          TYPE                LENGTH
----------------------------------------------------------------
NAME                          varchar             20
TIME                          datetime            31
VALUE                         double              17
```

### Inserting Tag Names

Register a new sensor/tag:

```sql
Mach> insert into tag metadata values ('TAG_0001');
1 row(s) inserted.
```

### Viewing Tag Metadata

Machbase provides a special table `_tag_meta` to view all registered tags:

```sql
Mach> select * from _tag_meta;
ID                   NAME
----------------------------------------------
1                    TAG_0001
[1] row(s) selected.
```

The ID is automatically assigned by the system.

### Updating Tag Names

You can modify tag names when needed:

```sql
Mach> update tag metadata set name = 'NEW_0001' where NAME = 'TAG_0001';
1 row(s) updated.

Mach> select * from _tag_meta;
ID                   NAME
----------------------------------------------
1                    NEW_0001
[1] row(s) selected.
```

### Deleting Tag Metadata

Remove tag metadata when it's no longer needed:

```sql
Mach> delete from tag metadata where name = 'NEW_0001';
1 row(s) deleted.

Mach> select * from _tag_meta;
ID                   NAME
----------------------------------------------
[0] row(s) selected.
```

> **Important**: You can only delete tag metadata if no actual sensor data references it.

## Working with Additional Metadata

### Creating Rich Metadata Structure

Add descriptive information beyond just the tag name:

```sql
create tag table TAG (name varchar(20) primary key, time datetime basetime, value double summarized)
metadata (type short, create_date datetime, srcip ipv4);

Mach> desc tag;
[ COLUMN ]
----------------------------------------------------------------
NAME                          TYPE                LENGTH
----------------------------------------------------------------
NAME                          varchar             20
TIME                          datetime            31
VALUE                         double              17
[ META-COLUMN ]
----------------------------------------------------------------
NAME                          TYPE                LENGTH
----------------------------------------------------------------
TYPE                          short               6
CREATE_DATE                   datetime            31
SRCIP                         ipv4                15
```

### Inserting with Partial Metadata

You can insert just the tag name - other fields will be NULL:

```sql
Mach> insert into tag metadata(name) values ('TAG_0001');
1 row(s) inserted.

Mach> select * from _tag_meta;
ID                   NAME                  TYPE        CREATE_DATE                     SRCIP
-------------------------------------------------------------------------------------------------------------
1                    TAG_0001              NULL        NULL                            NULL
[1] row(s) selected.
```

### Inserting Complete Metadata

Or provide all metadata fields:

```sql
Mach> insert into tag metadata values ('TAG_0002', 99, '2010-01-01', '1.1.1.1');
1 row(s) inserted.

Mach> select * from _tag_meta;
ID                   NAME                  TYPE        CREATE_DATE                     SRCIP
-------------------------------------------------------------------------------------------------------------
1                    TAG_0001              NULL        NULL                            NULL
2                    TAG_0002              99          2010-01-01 00:00:00 000:000:000 1.1.1.1
[2] row(s) selected.
```

### Updating Metadata Values

Update any metadata field:

```sql
Mach> update tag metadata set type = 11 where name = 'TAG_0001';
1 row(s) updated.

Mach> select * from _tag_meta;
ID                   NAME                  TYPE        CREATE_DATE                     SRCIP
-------------------------------------------------------------------------------------------------------------
2                    TAG_0002              99          2010-01-01 00:00:00 000:000:000 1.1.1.1
1                    TAG_0001              11          NULL                            NULL
[2] row(s) selected.
```

> **Note**: When updating metadata, you must include the NAME column in the WHERE clause.

## RESTful API for Tag Metadata

### Getting All Tags

Retrieve a list of all tags via HTTP:

```bash
$ curl -G "http://192.168.0.148:5001/machiot-rest-api/tags/list"
{"ErrorCode": 0,
 "ErrorMessage": "",
 "Data": [{"NAME": "TAG_0001"},
          {"NAME": "TAG_0002"}]}
```

### Getting Tag Time Ranges

Find the min and max timestamp for a tag (useful for charting):

```bash
# Time range for all tags
$ curl -G "http://192.168.0.148:5001/machiot-rest-api/tags/range/"
{"ErrorCode": 0,
 "ErrorMessage": "",
 "Data": [{"MAX": "2018-02-10 10:00:00 000:000:000", "MIN": "2018-01-01 01:00:00 000:000:000"}]}

# Time range for a specific tag
$ curl -G "http://192.168.0.148:5001/machiot-rest-api/tags/range/TAG_0001"
{"ErrorCode": 0,
 "ErrorMessage": "",
 "Data": [{"MAX": "2018-01-10 10:00:00 000:000:000", "MIN": "2018-01-01 01:00:00 000:000:000"}]}
```

## Real-World Example

Here's a complete example showing how to set up temperature sensor metadata:

```sql
-- Create tag table with metadata
CREATE TAG TABLE sensors (
    name VARCHAR(20) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
) METADATA (
    location VARCHAR(50),
    sensor_type VARCHAR(20),
    installed_date DATETIME,
    ip_address IPV4
);

-- Register sensors with full metadata
INSERT INTO sensors METADATA VALUES (
    'TEMP_BUILDING_A_FLOOR1', 'Building A - Floor 1', 'Temperature', '2024-01-15', '192.168.1.101'
);

INSERT INTO sensors METADATA VALUES (
    'TEMP_BUILDING_A_FLOOR2', 'Building A - Floor 2', 'Temperature', '2024-01-15', '192.168.1.102'
);

-- View all registered sensors
SELECT * FROM _sensors_meta;
```

## Best Practices

1. **Use Descriptive Names**: Tag names should be meaningful and follow a consistent naming convention
2. **Leverage Metadata**: Store static information in metadata columns to avoid redundancy in sensor data
3. **Plan Your Schema**: Define all needed metadata columns when creating the tag table
4. **Regular Cleanup**: Remove unused tag metadata to keep the registry clean
5. **API Access**: Use the RESTful API for integration with external applications

## Next Steps

- Learn about [Inserting Tag Data](../inserting-data) to start recording sensor readings
- Explore [Querying Tag Data](../querying-data) for data retrieval
- Understand [Tag Indexes](../tag-indexes) for performance optimization
