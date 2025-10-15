# Lookup Data Insert

Most things are the same as the input and update method of the volatile table.

There is one difference, when data is inserted through APPEND to the lookup table, if the primary key is duplicated, you can update the corresponding row by setting the 'LOOKUP_APPEND_UPDATE_ON_DUPKEY' property.

Details about 'LOOKUP_APPEND_UPDATE_ON_DUPKEY', [Property](/dbms/config-monitor/property) guide will help you.

## Lookup Table Reload

From machbase 6.7, Lookup Node manages Lookup table data.

If you want to reload the lookup table data from the lookup node, you can do this by using the EXEC TABLE_REFRESH command.

```sql
EXEC TABLE_REFRESH(lktable);
```
