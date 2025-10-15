# Data Retention

This function automatically deletes the data after the designated data retention period.

You can create a retention policy that specifies the retention period and deletion cycle, and apply/release it to the table through the ALTER statement.

## Create Retention Policy

Create a RETENTION POLICY by specifying the retention period and deletion cycle.

The retention period can be specified in units of months and days, and the deletion cycle can be specified in units of days and hours.

POLICY information can be checked by querying the **M$RETENTION** table.

**Syntax:**

```sql
CREATE RETENTION policy_name DURATION duration {MONTH|DAY} INTERVAL interval {DAY|HOUR}
```

* policy_name : Policy name to create
* duration : Retention period of data to be deleted (based on system time)
* interval : Retention period checking cycle

**Example:**

```sql
-- Data older than one day is deleted, and the update cycle is set to one hour.
Mach> CREATE RETENTION policy_1d_1h DURATION 1 DAY INTERVAL 1 HOUR;
Executed successfully.

-- Data older than one month is deleted, and the renewal cycle is set to three days.
Mach> CREATE RETENTION policy_1m_3d DURATION 1 MONTH INTERVAL 3 DAY;
Executed successfully.

Mach> SELECT * FROM M$RETENTION;
USER_ID     POLICY_NAME                               DURATION             INTERVAL             
-----------------------------------------------------------------------------------------------------
1           POLICY_1D_1H                              86400                3600                 
1           POLICY_1M_3D                              2592000              259200               
[2] row(s) selected.
```

## Apply Retention Policy

Apply the previously created RETENTION POLICY to the table.

After application, the retention period is checked and deleted every deletion cycle.

Table information to which the RETENTION POLICY is applied can be checked by querying the **V$RETENTION_JOB** table.

**Syntax:**

```sql
ALTER TABLE table_name ADD RETENTION policy_name
```

* table_name : table name to apply
* policy_name : policy name to apply

**Example:**

```sql
Mach> CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED);
Executed successfully.

Mach> ALTER TABLE tag ADD RETENTION policy_1d_1h;
Altered successfully.

Mach> SELECT * FROM V$RETENTION_JOB;
USER_NAME                                                                         TABLE_NAME                                                                        
-----------------------------------------------------------------------------------------------------------------------------------------------------------------------
POLICY_NAME                                                                       STATE                                                                             LAST_DELETED_TIME               
--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
SYS                                                                               TAG                                                                               
POLICY_1D_1H                                                                      WAITING                                                                           NULL                            
[1] row(s) selected.

```

## Release Retention Policy

Release the RETENTION POLICY applied to the table.

After release, the data is not deleted and is permanently preserved.

**Syntax:**

```sql
ALTER TABLE table_name DROP RETENTION;
```

* table_name : table name to release

**Example:**

```sql
Mach> ALTER TABLE tag DROP RETENTION;
Altered successfully.
```

## Drop Retention Policy

If a table to which the RETENTION POLICY is being applied exists, it cannot be dropped.

You must release the RETENTION of the table being applied and delete it.

**Syntax:**

```sql
DROP RETENTION policy_name
```

* policy_name : policy name to remove

**Example:**

```sql
Mach> ALTER TABLE tag ADD RETENTION policy_1d_1h;
Altered successfully.

-- ERROR
Mach> DROP RETENTION policy_1d_1h;
[ERR-02702: Policy (POLICY_1D_1H) is in use.]

Mach> ALTER TABLE tag DROP RETENTION;
Altered successfully.

-- SUCCESS
Mach> DROP RETENTION policy_1d_1h;
Dropped successfully.
```
