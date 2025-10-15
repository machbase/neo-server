# VARCHAR Storage Optimization

## Overview

Optimize VARCHAR column storage by controlling when data is stored in fixed vs. variable areas, improving both performance and storage efficiency.

## VARCHAR Storage Option
Maximum size that varchar data can be stored in a fixed area.
Any varchar value longer than this value is stored in the variable area.
This value can be specified from 15 to 127, with a default value of 15.

```sql
-- If the size of the input VARCHAR data is 15 or less, it is stored in the fixed data file instead of the extended file.
  
CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED, strval VARCHAR(100)) VARCHAR_FIXED_LENGTH_MAX = 15;
```
  
The property of the VARCHAR storage option value is shown in the table m$sys_table_property.
```sql
SELECT * FROM m$sys_table_property WHERE id={table_id} AND name = 'VARCHAR_FIXED_LENGTH_MAX';
```
