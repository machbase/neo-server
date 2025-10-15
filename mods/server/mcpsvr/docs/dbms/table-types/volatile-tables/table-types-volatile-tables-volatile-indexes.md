# Creating and Managing Volatile Index

##  Create and Use Index

The volatile table provides a RED-BLACK Tree optimized for real-time search. Indexes can be set for all data types. However, one index can be created for one column, and no composite index is provided.

```sql
Mach> create volatile table vtable (id integer, name varchar(10));
Created successfully.
Mach> create index idx_vrb on vtable (name) index_type redblack;
Created successfuly.
Mach> desc vtable;
----------------------------------------------------------------
NAME                          TYPE                LENGTH       
----------------------------------------------------------------
ID                            integer             11             
NAME                          varchar             10                 
 
[ INDEX ]                             
----------------------------------------------------------------
NAME                          TYPE                COLUMN
----------------------------------------------------------------
IDX_VRB                       REDBLACK            NAME               
iFlux>
```

##  Primary Key Index

When a primary key is assigned to a specific column of a volatile table, a RED-BLACK Tree index is automatically generated. In this case, a special index with a Uniqueness attribute is created and does not allow duplicate values.

```sql
Mach> create volatile table vtable (id integer primary key, name varchar(20));
Created successfully.
Mach> desc vtable;
----------------------------------------------------------------
NAME                          TYPE                LENGTH       
----------------------------------------------------------------
ID                            integer             11             
NAME                          varchar             20                 
 
[ INDEX ]                             
----------------------------------------------------------------
NAME                          TYPE                COLUMN
----------------------------------------------------------------
__PK_IDX_VTABLE               REDBLACK            ID  
 
iFlux>
```

##  Other Index Types

The bitmap or keyword index used in the log table can not be used in a volatile table.

```sql
Mach> create bitmap   index idx_1237 on vtable(id1);
[ERR-02069 : Error in index for invalid table. BITMAP Index can only be created for LOG Table.]
Mach> create keyword  index idx_1238 on vtable(name);
[ERR-02069 : Error in index for invalid table. KEYWORD Index can only be created for LOG Table.]
```
