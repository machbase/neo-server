# Automatic Duplicate Removal

## Overview

Machbase can automatically detect and remove duplicate sensor readings within a configurable time window, ensuring data quality without manual intervention.

## Configuring Duplicate Removal

When creating the TAG table, the duration for duplicate removal is passed as a table property. The maximum configurable duration for duplicate removal is 43200 minutes (30 days).

```sql
-- If the newly inserted data duplicates existing data within 1440 minutes(one day) from system time those data will be deleted.

CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED) TAG_DUPLICATE_CHECK_DURATION=1440;
```

The property of the duplication removal is shown in the table m$sys_table_property.
```sql
SELECT * FROM m$sys_table_property WHERE id={table_id} AND name = 'TAG_DUPLICATE_CHECK_DURATION';
```

Data insert/select example - duplication removal duration is 1440 minutes(one day)
```sql
-- Total inserted data are 6 and 4 of them are duplicates but 1 duplicated record was inserted 1440 minutes(one day) before
-- system time(1970-01-03 09:00:00 000:000:003).
-- Newly inserted duplicated data within the configured duration 1440 minutes(one day) are not displayed.

INSERT INTO tag VALUES('tag1', '1970-01-01 09:00:00 000:000:001', 0);
INSERT INTO tag VALUES('tag1', '1970-01-02 09:00:00 000:000:001', 0);
INSERT INTO tag VALUES('tag1', '1970-01-02 09:00:00 000:000:002', 0);
INSERT INTO tag VALUES('tag1', '1970-01-02 09:00:00 000:000:002', 1);
INSERT INTO tag VALUES('tag1', '1970-01-03 09:00:00 000:000:003', 0);
INSERT INTO tag VALUES('tag1', '1970-01-01 09:00:00 000:000:001', 0);

SELECT * FROM tag WHERE name = 'tag1';
NAME                  TIME                            VALUE                       
--------------------------------------------------------------------------------------
tag1                  1970-01-01 09:00:00 000:000:001 0
tag1                  1970-01-02 09:00:00 000:000:001 0                           
tag1                  1970-01-02 09:00:00 000:000:002 0
tag1                  1970-01-03 09:00:00 000:000:003 0      
tag1                  1970-01-01 09:00:00 000:000:001 0
  
```
## Changing configuration
TAG_DUPLICATE_CHECK_DURATION can be modified as shown below.

```sql
ALTER TABLE {table_name} set TAG_DUPLICATE_CHECK_DURATION={duration in minutes};
```

## Constraints of duplication removal

* The duplication removal setting can be configured on a minute basis, with a maximum limit of 43200 minutes (30 days).
* If the existing input data has already been deleted, any subsequent occurrence of the same data will not be considered as a duplicate for the purpose of duplication removal.
