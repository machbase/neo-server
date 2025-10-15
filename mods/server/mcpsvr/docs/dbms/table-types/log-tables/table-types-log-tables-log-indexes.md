# Index of Log Table

Two index types can be created in the Machbase log table. 

For more information, refer to the DDL page CREATE INDEX section of the SQL Reference  .

* BITMAP Index: bitmap index can be created in every column except Text, Binary type.
* KEYWORD Index: Used to search strings as it can be generated only for Varchar and Text column.

##  Create Index

Create an index on a specific column using the CREATE INDEX statement.

```sql
CREATE INDEX index_name ON table_name (column_name) [index_type] [tablespace] [index_prop_list]
    index_type ::= INDEX_TYPE { LSM | KEYWORD }
    tablespace ::= TABLESPACE tablesapce_name
    index_prop_list ::= value_pair, value_pair, ...
    value_pair ::= property_name = property_value
```

```sql
Mach> CREATE INDEX id_index ON log_data(id) INDEX_TYPE LSM TABLESPACE tbs_data MAX_LEVEL=3;
Created successfully.
```

##  Change Index

Change the index attribute using the ALTER INDEX statement.

```sql
ALTER INDEX index_name SET KEY_COMPRESS = { 0 | 1 }
```

```sql
Mach> ALTER INDEX id_index SET KEY_COMPRESS = 1;
```

##  Delete Index

Delete the specified index using the DROP INDEX statement. However, if there is another session in which the table is being searched, it will fail with an error.

```sql
DROP INDEX index_name;
```

```sql
Mach> DROP INDEX id_index;
Dropped successfully.
```
