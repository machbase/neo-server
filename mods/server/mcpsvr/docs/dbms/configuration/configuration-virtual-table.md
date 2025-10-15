# Virtual Table

The Virtual Tables are virtual tables that represent various operational information of the Machbase server in the form of a table. The names of these tables begin with "V$".

This data is used to know what state the Machbase server is operating in.
In addition, various information can be obtained through JOIN operation with other tables in this virtual table.

Virtual Tables are read-only and can not be added / deleted / updated by the user.

## Index

- [Session/System](#sessionsystem)
  - [V$PROPERTY](#vproperty)
  - [V$SESSION](#vsession)
  - [V$SESMEM](#vsesmem)
  - [V$SESSTAT](#vsesstat)
  - [V$SESTIME](#vsestime)
- [V$SYSMEM](#vsysmem)
  - [V$SYSSTAT](#vsysstat)
  - [V$SYSTIME](#vsystime)
  - [V$STMT](#vstmt)
  - [V$VERSION](#vversion)
- [Result Cache](#result-cache)
  - [V$RS\_CACHE\_LIST](#vrs_cache_list)
  - [V$RS\_CACHE\_STAT](#vrs_cache_stat)
- [Storage](#storage)  
  - [V$STORAGE](#vstorage)
  - [V$STORAGE\_MOUNT\_DATABASES](#vstorage_mount_databases)
  - [V$CACHE](#vcache)
  - [V$CACHE\_OBJECTS](#vcache_objects)
  - [V$STORAGE\_DC\_TABLESPACES](#vstorage_dc_tablespaces)
  - [V$STORAGE\_DC\_TABLESPACE\_DISKS](#vstorage_dc_tablespace_disks)
  - [V$STORAGE\_DC\_DWFILES](#vstorage_dc_dwfiles)
  - [V$STORAGE\_DC\_PAGECACHE](#vstorage_dc_pagecache)
  - [V$STORAGE\_DC\_PAGECACHE\_LRU\_LST](#vstorage_dc_pagecache_lru_lst)
  - [V$STORAGE\_USAGE](#vstorage_usage)
  - [V$STORAGE\_TABLES](#vstorage_tables)
- [Log Table](#log-table)
  - [V$STORAGE\_DC\_TABLES](#vstorage_dc_tables)
  - [V$STORAGE\_DC\_TABLES\_STAT](#vstorage_dc_tables_stat)
  - [V$STORAGE\_DC\_TABLE\_COLUMNS](#vstorage_dc_table_columns)
  - [V$STORAGE\_DC\_TABLE\_COLUMN\_PARTS](#vstorage_dc_table_column_parts)
  - [V$STORAGE\_DC\_TABLE\_INDEXES](#vstorage_dc_table_indexes)
- [LSM(Log Structured Merge) Index](#lsmlog-structured-merge-index)
  - [V$STORAGE\_DC\_LSMINDEX\_LEVEL\_PARTS](#vstorage_dc_lsmindex_level_parts)
  - [V$STORAGE\_DC\_LSMINDEX\_LEVEL\_PARTS\_CACHE](#vstorage_dc_lsmindex_level_parts_cache)
  - [V$STORAGE\_DC\_LSMINDEX\_LEVELS](#vstorage_dc_lsmindex_levels)
  - [V$STORAGE\_DC\_LSMINDEX\_FILES](#vstorage_dc_lsmindex_files)
  - [V$STORAGE\_DC\_LSMINDEX\_AGER\_JOBS](#vstorage_dc_lsmindex_ager_jobs)
- [Volatile Table](#volatile-table)
  - [V$STORAGE\_DC\_VOLATILE\_TABLE](#vstorage_dc_volatile_table)
- [Tag Table](#tag-table)
  - [V$STORAGE\_TAG\_TABLES](#vstorage_tag_tables)
  - [V$STORAGE\_TAG\_CACHE](#vstorage_tag_cache)
  - [V$STORAGE\_TAG\_CACHE\_OBJECTS](#vstorage_tag_cache_objects)
  - [V$STORAGE\_TAG\_TABLE\_FILES](#vstorage_tag_table_files)
  - [V$STORAGE\_TAG\_INDEX](#vstorage_tag_index)
- [Tag Rollup](#tag-rollup)
  - [V$ROLLUP](#vrollup)
- [Stream](#stream)
  - [V$STREAMS](#vstreams)
- [License](#license)
  - [V$LICENSE\_INFO](#vlicense_info)
  - [V$LICENSE\_STATUS](#vlicense_status)
- [Mutex](#mutex)
  - [V$MUTEX](#vmutex)
  - [V$MUTEX\_WAIT\_STAT](#vmutex_wait_stat)
- [Cluster](#cluster)
  - [V$NODE\_STATUS](#vnode_status)
  - [V$DDL\_INFO](#vddl_info)
  - [V$REPLICATION](#vreplication)
  - [V$REPL\_SENDER](#vrepl_sender)
  - [V$REPL\_SENDER\_META](#vrepl_sender_meta)
  - [V$REPL\_RECEIVER](#vrepl_receiver)
  - [V$REPL\_RECEIVER\_META](#vrepl_receiver_meta)
  - [V$REPL\_READER](#vrepl_reader)
  - [V$REPL\_READER\_META](#vrepl_reader_meta)
  - [V$REPL\_WRITER](#vrepl_writer)
  - [V$REPL\_WRITER\_META](#vrepl_writer_meta)
- [Others](#others)
  - [V$TABLES](#vtables)
  - [V$COLUMNS](#vcolumns)
  - [V$RETENTION\_JOB](#vretention_job)

## Session/System

### V$PROPERTY
---

Displays the property information set in the server.

|Column Name|Description|
|--|--|
|NAME|Property name|
|VALUE|Property value|
|TYPE|Data type|
|DEFLT|Default value|
|MIN|Minimum set value|
|MAX|Maximum set value|

### V$SESSION
---

Displays session information connected to the Machbase server.

|Column Name|Description|
|--|--|
|HOSTNAME (Cluster Only)|Name of the HOST which the session is connected.|
|ID|Session identifier|
|CLOSED|Whether connection is closed|
|USER_ID|User identifier|
|LOGIN_TIME|Connection time|
|CLIENT_TYPE|Connected client type|
|USER_NAME|User name|
|USER_IP|User IP address|
|SQL_LOGGING|Leave message in session trace log status<br><br>1: Leaves errors occurring in the Parsing, Validation, Optimization steps<br>2: Leaves result performance of DDL<br>3: (Leaves both cases above)|
|SHOW_HIDDEN_COLS|Whether hidden columns are shown upon SELECT|
|FEEDBACK_APPEND_ERROR|Whether there is feedback to client on APPEND error|
|DEFAULT_DATE_FORMAT|Default input format upon Datetime input |
|HASH_BUCKET_SIZE|Number of Buckets in Temp Hashtable created when performing query|
|MAX_QPX_MEM|Maximum memory size available when performing query|
|RS_CACHE_ENABLE|Whether Result Cache in in use|
|RS_CACHE_TIME_BOUND_MSEC|Maximum elapsed time to store results when using Result Cache|
|RS_CACHE_MAX_MEMORY_PER_QUERY|Maximum size of memory used per query when using Result Cache|
|RS_CACHE_MAX_RECORD_PER_QUERY|Maximum number of results used per query when using Result Cache|
|RS_CACHE_APPROXIMATE_RESULT_ENABLE|Whether to cache approximate query results when using Result Cache|
|IDLE_TIMEOUT|Terminate the session if the client does nothing for that time after the session connected.|
|QUERY_TIMEOUT|Response waiting time for query execution|

### V$SESMEM
---

Displays session memory information.

|Column Name|Description|
|--|--|
|SID|Session identifier|
|ID|Memory manager identifier|
|USAGE|Usage size|

### V$SESSTAT
---

Displays statistical information of the session.

|Column Name|Description|
|--|--|
|SID|Session identifier|
|ID|Statistical information identifier|
|VALUE|Statistical information value|

### V$SESTIME
---

Displays the time information of the session.

|Column Name|Description|
|--|--|
|SID|Session identifier|
|ID|Performance unit identifier|
|ACCUM_TICK|Cumulative time|
|MAX_TICK|Maximum time (per each performance unit)|

## V$SYSMEM

Displays memory information of the system.

|Column Name|Description|
|--|--|
|ID|Memory manager identifier|
|NAME|Memory manager name|
|USAGE|Current usage|
|MAX_USAGE|(Recorded) Maximum usage|

### V$SYSSTAT
---

Displays statistical information of the system.

|Column Name|Description|
|--|--|
|ID|Statistical information identifier|
|NAME|Statistical information name|
|VALUE|Statistical information value|

### V$SYSTIME
---

Displays the time information of the system.

|Column Name|Description|
|--|--|
|ID|Performance unit identifier|
|NAME|Performance unit name|
|ACCUM_TICK|Cumulative time|
|AVG_TICK|Average time (per each performance unit)|
|MIN_TICK|Minimum Time (per each performance unit)|
|MAX_TICK|Maximum Time (per each performance unit)|
|COUNT|Performance frequency|

### V$STMT
---

Displays information about the query statement that the user is currently executing.

|Column Name|Description|
|--|--|
|ID|Query identifier|
|SESS_ID|Performed query session identifier|
|STATE|Query status|
|RECORD_SIZE|Resulting record size of select statements|
|QUERY|Query statement|
 

### V$VERSION
---

Displays information about Machbase version.

|Column Name|Description|
|--|--|
|BINARY_DB_MAJOR_VERSION|Database major version|
|BINARY_DB_MINOR_VERSION|Database minor version|
|BINARY_META_MAJOR_VERSION|META major version|
|BINARY_META_MINOR_VERSION|META minor version|
|BINARY_CM_MAJOR_VERSION|Client (Communication Level) major version|
|BINARY_CM_MINOR_VERSION|Client (Communication Level) minor version|
|BINARY_SIGNATURE|Version name of DB data files.|
|FILE_DB_MAJOR_VERSION|File DB major version|
|FILE_DB_MINOR_VERSION|File DB minor version|
|FILE_META_MAJOR_VERSION|File META major version|
|FILE_META_MINOR_VERSION|File META minor version|
|FILE_CM_MAJOR_VERSION|File Client (Communication Level) major version|
|FILE_CM_MINOR_VERSION|File Client (Communication Level) minor version|
|FILE_CREATE_TIME|File creation time|
|EDITION|Machbase type|

## Result Cache

### V$RS_CACHE_LIST
---

Display the result cache list.

|Column Name|Description|
|--|--|
|TOUCH_TIME|Time cache was used or created|
|USER_ID|Cache user identifier|
|QUERY|Cache query statement|
|TIME_SPENT|Time spent producing result|
|TABLE_COUNT|Number of tables associated with query statement|
|RECORD_COUNT|Number of result records|
|REFERENCE_COUNT|Number of sessions currently being referenced|
|HIT_COUNT|Cache hit count|
|AGGR_TOUCH_TIME|Time the cache was used or created for aggregate results|
|AGGR_HIT_COUNT|Cache hit count for aggregate results|

### V$RS_CACHE_STAT
---

Display statistical information of result cache in one session.

|Column Name|Description|
|--|--|
|CACHE_COUNT|Number of result caches|
|CACHE_HIT|Total cache hit count|
|AGGR_HIT|Total cache hit count for aggregate results|
|CACHE_REPLACED|Cache replacement count|
|CACHE_MEMORY_USAGE|Size of cache memory used|

## Storage

### V$STORAGE
---

Displays internal information of the storage system.

|Column Name|Description|
|--|--|
|DC_TABLE_FILE_SIZE|Total capacity of disk column data|
|DC_INDEX_FILE_SIZE|Total capacity of index file data|
|DC_TABLESPACE_DWFILE_SIZE|Total capacity of DWFILE for all column data|
|DC_KV_TABLE_FILE_SIZE|Total number of data files of TAGDATA table partition tables|

### V$STORAGE_MOUNT_DATABASES
---

Displays the information of the mounted backup database using the mount function.

|Column Name|Description|
|--|--|
|NAME|Mounted database name|
|PATH|Backup file location|
|BACKUP_TBSID|Backup database tablespace identifier|
|BACKUP_SCN|Backup database identifier|
|MOUNTDB|Backup time|
|DB_BEGIN_TIME|Backup database first entry time|
|DB_END_TIME|Backup database last entry time|
|BACKUP_BEGIN_TIME|Backup begin time|
|BACKUP_END_TIME|Backup end time|
|FLAG|Property flag|

### V$CACHE
---

Displays the comprehensive information on the cache objects containing the results read from the storage system.

|Column Name|Description|
|--|--|
|OBJ_COUNT|Current number of result set cache objects|

### V$CACHE_OBJECTS
---

Displays information about each cache object that contains the results read from the storage system.

|Column Name|Description|
|--|--|
|OID|Object identifier|
|REF_COUNT|Reference count|
|FLAG|(Internal server use flag)|

### V$STORAGE_DC_TABLESPACES
---

Displays the table space information of the storage system.

|Column Name|Description|
|--|--|
|NAME|Tablespace name|
|ID|Tablespace identifier|
|FLAG|Flag indicating tablespace property|
|REF_COUNT|Tablespace reference count|
|DISK_COUNT|Tablespace disk count|

### V$STORAGE_DC_TABLESPACE_DISKS
---

Displays the table space information of the storage system.

|Column Name|Description|
|--|--|
|NAME|Disk name|
|ID|Disk identifier|
|TABLESPACE_ID|Disk tablespace identifier|
|PATH|Disk path|
|IO_THREAD_COUNT|I/O Thread count|
|IO_JOB_COUNT|I/O Job count|
|VIRTUAL_DISK_COUNT|Virtual disk count|

### V$STORAGE_DC_DWFILES
---

Displays the information of the double-write file (DW File) operated by the storage system.

|Column Name|Description|
|--|--|
|TBS_ID|Tablespace identifier|
|DISK_ID|Disk identifier|
|FILE|File path|
|TABLE_ID|Table identifier|
|COLUMN_ID|Column identifier|
|PARTITION_ID|Partition identifier|
|PAGE_ID|Page identifier|
|DISK_OFFSET|Disk offset|
|DISK_IMAGE_SIZE|Disk image size|
|HEAD_CRC32CODE_IMAGE|Head CRC32 Code Image|
|TAIL_CRC32CODE_IMAGE|Tail CRC32 Code Image|
|CRC32CODE_PAGE|CRC32 Code Page|
|HEAD_TIMESTAMP_PAGE|Head Timestamp Page|
|TAIL_TIMESTAMP_PAGE|Tail Timestamp Page|

### V$STORAGE_DC_PAGECACHE
---

Displays information about the Page Cache operating on the storage system

|Column Name|Description|
|--|--|
|MAX_MEM_SIZE|Maximum memory size of Page Cache|
|CUR_MEM_SIZE|Current memory size of Page Cache|
|PAGE_CNT|Number of cached pages|
|CHECK_TIME|Check time|

### V$STORAGE_DC_PAGECACHE_LRU_LST
---

Displays information about the LRU List of Page Cache operated by the storage system.

|Column Name|Description|
|--|--|
|OBJECT_ID|Object identifier|
|LEVEL|Partition level|
|PARTITION_ID|Partition identifier|
|OFFSET|Page Cache Offset|
|SIZE|Page size|
|REF_CNT|Reference count|

### V$STORAGE_USAGE
---

Displays the amount of storage used by the storage system.

|Column Name|Description|
|--|--|
|TOTAL_SPACE|Total storage capacity where the $MACHBASE_HOME/dbs directory is located|
|USED_SPACE|Total storage usage where the $MACHBASE_HOME/dbs directory is located|
|USED_RATIO|Percentage of usage(%)|
|RATIO_CAP|Storage usage limit. Data input/index construction stops when USED_RATIO reaches this limit.|

### V$STORAGE_TABLES
---

Display table details.

|Column Name|Description|
|--|--|
|ID|Table ID|
|TYPE|Table type<br> - Persistent: LOG / TAG Table<br> - Volatile: Volatile Table - Key-Value: Accompanying table of TAG table|
|STATUS|Current Status<br> - Creating...: Creating table by CREATE TABLE query<br> - Normal: normal<br> - Predrop: DROP TABLE query accepted<br> - Dropping...: DROP TABLE query processing<br> - Dropped: DROP TABLE query completed<br> - Mounted: The backed up database loaded with the MOUNT query|
|STORAGE_USAGE|Capacity occupied by the table in storage|

## Log Table

### V$STORAGE_DC_TABLES
---

Displays internal information about Log Table.

|Column Name|Description|
|--|--|
|ID|Table identifier|
|DATABASE_ID|Database identifier|
|CREATE_SCN|System Change Number at time of creation|
|UPDATE_SCN|System Change Number at time of most recent update|
|DDL_REF_COUNT|Number of sessions referencing table in DDL syntax execution|
|BEGIN_RID|Minimum table RID|
|END_RID|Last row ID of table + 1|
|BEGIN_META_RID|ID at start of recording meta information|
|END_META_RID|ID at end of recording meta information|
|END_SYNC_RID|Last row ID recorded on disk + 1|
|FLAG|Flag indicating table property|
|COLUMN_COUNT|Table column count|
|INDEX_COUNT|Table index count|
|INDEX_MIN_END_RID|Last RID recorded in index + 1|
|LAST_ARRIVAL_TIME|Last recorded _arrival_time value|
|LAST_CHECKPOINT_TIME|Last checkpoint time|
|TYPE|Table type|

### V$STORAGE_DC_TABLES_STAT
---

Displays internal information about Log Table.

|Column Name|Description|
|--|--|
|TABLESPACE_ID|Tablespace identifier|
|TABLE_ID|Table identifier|
|COLUMN_ID|Column identifier|
|COUNT|Record count|

### V$STORAGE_DC_TABLE_COLUMNS
---

Displays information about the columns in the Log Table.

|Column Name|Description|
|--|--|
|TABLE_ID|Table identifier|
|DATABASE_ID|Database identifier|
|ID|Column identifier|
|FLAG|Property flag|
|SIZE|Column data size|
|PARTITION_VALUE_COUNT|Maximum number of data stored in partition|
|PAGE_VALUE_COUNT|Maximum number of data stored in page|
|CACHE_VALUE_COUNT|Maximum number of cache values|
|MINMAX_CACHE_SIZE|Maximum size of MIN / MAX cache for column partitions|
|CUR_APPEND_PARTITION_ID|Current partition in progress of input identifier|
|CUR_CACHE_PARTITION_COUNT|Number of partitions that have read data in current cache|
|CUR_MINMAX_CACHE_SIZE|Current Min / MAX cache size|
|END_RID_FOR_DEFAULT_VALUE|Location value of end rid maintaining default value|
|DISK_FILE_SIZE|Total size of column partition data file for that column|
|MEMORY_TOTAL_SIZE|Memory size used by table|
|MEMORY_ALLOC_SIZE|Memory size allocated by table|

### V$STORAGE_DC_TABLE_COLUMN_PARTS
---

Displays column partition information of log table.

|Column Name|Description|
|--|--|
|TABLE_ID|Table identifier|
|DATABASE_ID|Database identifier|
|COLUMN_ID|Column identifier|
|ID|Partition identifier|
|FLAG|Flag indicating column property|
|BEGIN_RID|First RID stored in partition|
|END_RID|Last RID stored in partition|
|END_SYNC_RID|Last RID SYNC ended.<br><br>Data with a RID greater than the starting RID and less than the last SYNC RID is recorded in the partition file.|
|MIN_TIME|First time data was entered into column partition|
|MAX_TIME|Last time data was entered into column partition|
|MAX_VALUE_COUNT_PER_PARTITION|Maximum partition data count|
|MAX_VALUE_COUNT_PER_PAGE|Maximum page data count|
|MAX_PAGE_COUNT|Maximum partition page count|
|PAGE_SIZE|Page size stored in column partition|
|PAGE_COUNT|Page count created in current column partition|
|COMPRESS_RATIO|Column partition compression ratio. If it is 0, data compression has not been performed yet.|
|DISK_FILENAME|Partition file name|
|EXTERNAL_PART_SIZE|A large amount of data is written to the external partition file, indicating the size of the file|
|MIN_VALUE|Minimum column partition value|
|MAX_VALUE|Maximum column partition value|

### V$STORAGE_DC_TABLE_INDEXES
---

Displays index information generated in Log Table.

|Column Name|Description|
|--|--|
|TABLE_ID|Table identifier|
|DATABASE_ID|Database identifier|
|ID|Index identifier|
|FLAG|Flag indicating index property|
|TABLE_BEGIN_RID|First RID entered into table|
|TABLE_END_RID|Last table RID|
|BEGIN_RID|First index RID|
|END_RID|Last index RID|
|END_SYNC_RID|Last recorded RID in file + 1|
|COLUMN_COUNT|Index column count|
|BEGIN_PART_ID|Index first partition identifier|
|END_PART_ID|Index last partition identifier|
|FLUSH_REQUEST_COUNT|Number of index partitions requested to reflect on disk|
|MAX_KEY_SIZE|Maximum key size|
|INDEX_TYPE|Index type|
|DISK_FILE_SIZE|Total size of index partition file for that index|
|LAST_CHECKPOINT_TIME|Last checkpoint time|

## LSM(Log Structured Merge) Index

### V$STORAGE_DC_LSMINDEX_LEVEL_PARTS
---

Displays information about LSM Index partitions.

|Column Name|Description|
|--|--|
|TABLE ID|Index table identifier|
|TABLESPACE_ID|Tablespace identifier|
|INDEX_ID|Index identifier|
|LEVEL|Index partition LSM level|
|PARTITION_ID|Partition identifier|
|BEGIN_RID|First RID entered into partition|
|END_RID|Last RID entered into partition + 1|
|KEY_VALUE_COUNT|Key value count entered into partition|
|KEY_VALUE_TABLE_SIZE|Size of page storing key value|
|KEY_VALUE_TABLE_PAGE_COUNT|Number of pages storing key value|
|MIN_KEY_VALUE|Minimum key value|
|MAX_KEY_VALUE|Maximum key value|
|BITMAP_TABLE_SIZE|Total size of page storing bitmap value|
|BITMAP_TABLE_PAGE_COUNT|Number of pages storing bitmap value|
|META_SIZE|Total size of page storing meta information|
|META_PAGE_COUNT|Number of pages storing meta information|
|TOTAL_BUILD_MSEC|Total time to complete partition|
|KEYVAL_BUILD_MSEC|Total time to complete partition for KeyValue Mode|
|BITMAP_BUILD_MSEC|Total time to complete partition for Bitmap Mode|

### V$STORAGE_DC_LSMINDEX_LEVEL_PARTS_CACHE
---

Displays information about the LSM Index partition cache.

|Column Name|Description|
|--|--|
|TABLESPACE_ID|Tablespace identifier|
|TABLE_ID|Index Table identifier|
|INDEX_ID|Index identifier|
|LEVEL|Index partition LSM level|
|PARTITION_ID|Partition identifier|
|BEGIN_RID|First RID entered into partition|
|END_RID|Last RID entered into partition + 1|
|KEY_VALUE_COUNT|Number of key values entered into partition|
|KEY_VALUE_TABLE_SIZE|Size of page storing key value|
|KEY_VALUE_TABLE_PAGE_COUNT|Number of pages storing key value|
|BITMAP_TABLE_SIZE|Total size of page storing bitmap value|
|BITMAP_TABLE_PAGE_COUNT|Number of pages storing bitmap value|
|META_SIZE|Total size of page storing meta information|
|META_PAGE_COUNT|Number of pages storing meta information|
|MEMORY_SIZE|Memory usage|
|MEMORY_SIZE_RBTREE|Redblack Tree memory usage|

### V$STORAGE_DC_LSMINDEX_LEVELS
---

Displays information about the level of the LSM index.

|Column Name|Description|
|--|--|
|TABLE ID|Table identifier|
|DATABASE_ID|Database identifier|
|INDEX_ID|Index identifier|
|LEVEL|Level|
|BEGIN_RID|First partition RID|
|END_RID|Last partition RID + 1|
|META_BEGIN_RID|RID at start time of recording meta information|
|META_END_RID|RID at end time of recording meta information|
|DELETE_END_RID|Maximum deleted RID + 1|

### V$STORAGE_DC_LSMINDEX_FILES
---

Displays information about the files that make up the LSM Index.

|Column Name|Description|
|--|--|
|TABLE_ID|Table identifier|
|DATABASE_ID|Database identifier|
|INDEX_ID|Index identifier|
|LEVEL|Index partition LSM level|
|PARTITION_ID|Partition identifier|
|BEGIN_RID|Partition first RID|
|END_RID|Partition last RID + 1|
|PATH|Index file location|

### V$STORAGE_DC_LSMINDEX_AGER_JOBS
---

Displays working status of Ager responsible for LSM Index deletion.

|Column Name|Description|
|--|--|
|TABLE_ID|Table identifier|
|INDEX_ID|Index identifier|
|LEVEL|Index partition LSM level|
|BEGIN_RID|First partition RID|
|END_RID|Last partition RID + 1|
|STATE|Index Ager working status|

## Volatile Table

### V$STORAGE_DC_VOLATILE_TABLE
---

Displays information about Volatile Table.

|Column Name|Description|
|--|--|
|MAX_MEM_SIZE|Maximum Volatile Tablespace size|
|CUR_MEM_SIZE|Current Volatile Tablespace size|

## Tag Table

### V$STORAGE_TAG_TABLES
---

Displays information about the partition table in the Tagdata Table.

|Column Name|Description|
|--|--|
|ID|Table identifier|
|TABLE_BEGIN_RID|Table start RID|
|TABLE_END_RID|Table end RID|
|WRITE_END_RID|Last RID which is written to data file.|
|EXT_ROW_COUNT|Number of entries to external partitions in VARCHAR records|
|EXT_WRITE_COUNT|Number of entries to data files in VARCHAR records|
|DISK_INDEX_END_RID|Index end RID stored in storage|
|MEMORY_INDEX_END_RID|Table end RID in memory index|
|DELETE_MIN_DATE|Minimum time of deleted data by execute  DELETE BETWEEN query|
|DELETE_MAX_DATE|Maximum time of deleted data by execute  DELETE BETWEEN/BEFORE query|
|INDEX_STATE|Current Index Build State<br> - IDLE: Build Complete, waiting<br> - PROGRESS: Build in progress<br> - IOWAIT: Waiting for I/O operation in storage<br> - PENDING: Waiting for table read lock<br> - SHUTDOWN:  Stopped. DELETE operation or DROP operation in progress.<br> - ABNORMAL: Abnormal end|
|DELETE_STATE|Current DELETE operation state.<br>There is no IDLE because it is performed only when a DELETE command is entered.<br> - PROGRESS: Deletion in progress<br> - IOWAIT: Waiting for I/O operation in storage<br> - PENDING: Waiting for table read/write lock<br> - SHUTDOWN: Stopped. DELETE operation or DROP operation in progress.<br> - ABNORMAL: Abnormal end|
|SAVE_STATE|Current Table Save operation state.<br> - IDLE: Save Complete, waiting<br> - PROGRESS: Save in progress<br> - IOWAIT: Waiting for I/O operation in storage<br> - PENDING: Waiting for table read lock<br> - SHUTDOWN: Stopped. DELETE operation or DROP operation in progress.<br> - ABNORMAL: Abnormal end|
|VINDEX_STATE|Current VARCHAR Index Build State<br> - IDLE: Build Complete, waiting<br> - PROGRESS: Build in progress<br> - IOWAIT: Waiting for I/O operation in storage<br> - PENDING: Waiting for table read lock<br> - SHUTDOWN:  Stopped. DELETE operation or DROP operation in progress.<br> - ABNORMAL: Abnormal end|

### V$STORAGE_TAG_CACHE
---

Displays the cache information used in the partition table of the Tagdata Table.

|Column Name|Description|
|--|--|
|CATEGORY|Type of object in cache|
|USED_MEMORY|Size of memory in use|
|BLOCK_COUNT|Data cache count|
|CACHE_HIT|Data cache hit count|
|CACHE_MISS|Data cache miss count|
|FLUSHOUT|Number of page flushouts due to data cache crash|
|COLDREAD|Number of data pages read directly from storage|
|MEMORY_WAIT|Number of times data memory waited for cache crash|
|IO_WAIT|Data read operation wait count|

### V$STORAGE_TAG_CACHE_OBJECTS
---

Displays detailed information about each cache block used in the partition table of the Tagdata Table.

|Column Name|Description|
|--|--|
|CATEGORY|Object classification being cached|
|LATEST_HIT|Last approach time|
|STATUS|Cache status<br> - None: Memory allocation done<br> - Resides: Already stored in cache<br> - Loading: Loading table data from storage<br> - ERROR!: Error appears while loading data|
|WAIT_COUNT|The number of waiting times because the cache could not be read in the Loading state|
|REF_COUNT|Number of sessions currently referencing the cache block|
|HIT_COUNT|Number of times a cache block was referenced|
|TABLE_ID|Table Identifier|
|FILE_ID|File Identifier|
|PART_ID|Partition identifier inside the datafile|
|SAVE_SCN|SCN of table save|
|VSAVE_SCN|SCN of table save|
|DELETE_SCN|SCN of delete operation|
|OFFSET|Datafile offset|
|DATA_SIZE|Data size before compression, or 0|

### V$STORAGE_TAG_TABLE_FILES
---

Displays the file information of the partition table of the Tag Table.

|Column Name|Description|
|--|--|
|TABLE_ID|Table identifier|
|FILE_ID|File identifier|
|STATE|Index status<br> - COMPLETE: Data stored, index build complete<br> - INDEXING: Index build in progress<br> - FILLED: Data is full, waiting for Index build<br> - PARTIAL: Data not yet full, waiting for Index build|
|REF_COUNT|Number of sessions currently referencing the file|
|ROW_COUNT|Number of records stored in the file, including those that were deleted|
|DEL_COUNT|Number of records deleted from the file|
|MIN_DATE|Minimum datatime value of this data file.|
|MAX_DATE|Maximum datatime value of this data file.|

### V$STORAGE_TAG_INDEX
---

Displays index information generated in Tag Table.

|Column Name|Description|
|--|--|
|TABLE_ID|Table identifier|
|INDEX_ID|Index identifier (if INDEX_ID is 4294967295 it is a default index that is created automatically when the tag table is created.)|
|INDEX_STATE|Current index build state<br> - IDLE: Build Complete, waiting<br> - INDEXING: Build in progress<br> - STORAGE FULL: Stopped because of disk full|
|DISK_INDEX_END_RID|Index end RID stored in storage|
|MEMORY_INDEX_END_RID|Table end RID in memory index|
|TABLE_END_RID|Table end RID|

## Tag Rollup

### V$ROLLUP
---

Displays the Rollup information that stores information of the Tagdata table.

|Column Name|Description|
|--|--|
|ID|Rollup job ID|
|ROLLUP_TABLE_NAME|Table name to store Rollup information|
|SOURCE_TABLE_NAME|Name of tag table that Rollup will query|
|USER_ID|User ID of Rollup Table and Source Table|
|ROOT_TABLE|Root Source Table Name|
|INTERVAL|Rollup execution cycle (sec)|
|END_RID|Source Table end RID|
|ENABLED|Indicates Rollup progress status|
|LAST_ELAPSED_MSEC|Number of recently entered records|

## Stream

### V$STREAMS
---

|Column Name|Description|
|--|--|
|NAME|The name of stream query.|
|LAST_EX_TIME|Last execution time of this query.|
|TABLE_NAME|The name of table which searched from the query|
|END_RID|The last RID read by stream query|
|STATE|Current state of stream query|
|QUERY_TXT|Query text|
|ERROR_MSG|Error message of the last stream execution|
|FREQUENCY|Minimum wait time for query execution. If it is 0, it is executed every record. If it is not 0, it is executed each time. The unit is nanoseconds.|

## License

### V$LICENSE_INFO
---

Displays license information.

|Column Name|Description|
|--|--|
|INSTALL_DATE|Installation date|
|TYPE|License type|
|POLICY|License policy type|
|CUSTOMER|Customer name|
|ISSUE_DATE|Issue date|
|ID|Host ID|
|EXPIRY_DATE|Expiration date|
|SIZE_LIMIT|Work input limit|
|ADDENDUM|Additional data rate|
|VIOLATION_ACTION|Indicates license violation|
|VIOLATION_LIMIT|Number of violations to suspend service (monthly update)|
|STOP_ACTION|Indicates database is terminated in the event of license violation|
|RESET_FLAG|(Internal server use)|

### V$LICENSE_STATUS
---

Displays the license status.

|Column Name|Description|
|--|--|
|USER_DATA_PER_DAY|Amount of data that can be entered per day|
|PREVIOUS_CHECK_DATE|Previous license check date|
|VIOLATION_COUNT|License violation count|
|STOP_ENABLED|Display whether license restrictions are enabled or not.|

## Mutex

### V$MUTEX
---

Displays current mutex status.

|Column Name|Description|Note|
|--|--|--|
|OBJECT|Address of the mutex object| |
|NAME|The name given when creating the mutex| |
|TYPE|Mutex type| - Mutex: pmuMutex<br> - RW Mutex: pmuRWMutex|
|OWNER|ID of the thread that acquired the mutex| - Mutex: 0 if no thread acquired the mutex.<br> - RW Mutex w/ Read-Lock: 0<br> - RW Mutex w/ Write-Lock: ID of the thread that acquired the write lock.|
|LOCK_COUNT|Number of threads that acquired the mutex| - RW Mutex can be 2 or more.|
|PEND_COUNT|Number of threads waiting to acquire a mutex| - Collect only when TRACE_MUTEX_WAIT_STATUS=1|
|TRY_COUNT|Number of attempts to acquire the mutex| - Collect only when TRACE_MUTEX_WAIT_STATUS=1|
|CONFLICT_COUNT|Number of failed to acquire mutex| - Collect only when TRACE_MUTEX_WAIT_STATUS=1|
|WAIT_TICK|Sum of waiting time to acquire mutex| - Collect only when TRACE_MUTEX_WAIT_STATUS<br> - Do not write to RW Mutex|
|WAIT_TICK_AVG|Average time to success after an attempt to acquire a mutex| - Collect only when TRACE_MUTEX_WAIT_STATUS=1<br> - Do not write to RW Mutex|
|HELD_TICK|Total time from acquiring the mutex to releasing it| - Collect only when TRACE_MUTEX_WAIT_STATUS=1<br> - Do not write to RW Mutex|
|HELD_TICK_AVG|Average time from acquisition to release of mutex| - Collect only when TRACE_MUTEX_WAIT_STATUS=1<br> - Do not write to RW Mutex|

### V$MUTEX_WAIT_STAT
---

Shows the call stack of the currently waiting mutex.

|Column Name|Description|Note|
|--|--|--|
|THREAD_ID|ID of the thread waiting to acquire the mutex| |
|OBJECT|Address of the mutex being acquired| - Same as OBJECT in V$MUTEX|
|DEPTH|Call stack depth| - Collect only when TRACE_MUTEX_WAIT_STATUS=1|
|SYMBOL|Symbol of function that called acquire mutex| - Collect only when TRACE_MUTEX_WAIT_STATUS=1|

## Cluster

### V$NODE_STATUS
---

Displays the Node status for each Cluster. Only one is displayed.

|Column Name|Description|
|--|--|
|NODETYPE|Node type. There are two types that can be viewed by queries.<br> - Broker<br> - Warehouse|
|STATE|Node status|

### V$DDL_INFO
---

Displays DDL information performed by Cluster.

|Column Name|Description|
|--|--|
|SEQUENCENUMBER|DDL sequence number|
|TIME|DDL execution time|
|VALUE|DDL query result value (Internal server use)|
|CLIENT|Client name|
|BROKER|Lead Broker Node name|
|USER|User name|
|SQL|DDL query value|

### V$REPLICATION
---

Displays information about the replication operation.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|MODE|(Internal server use)|
|STATE|Node status|
|ADDR|Replication Manager address|
|PORT_NO|Replication Manager port number|
|MAX_SENDER_COUNT|Maximum number of Senders that can be created|
|RUN_SENDER_COUNT|Maximum number of active Senders|

### V$REPL_SENDER
---

Displays Sender replication when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|ID|Sender identifier|
|STATUS|Sender operational status|
|PAYLOAD_RECV_COUNT|Number of payloads received from sender|
|PAYLOAD_RECV_BYTES|Total payload size received from Sender|
|QUEUE_REMAIN_COUNT|Number of buffers remaining in the Receive Queue|
|NET_SEND_COUNT|Net send count|
|NET_SEND_SIZE|Net send size|
|NET_RECV_COUNT|Net receive count|
|NET_RECV_SIZE|Net receive size|

### V$REPL_SENDER_META
---

Displays Sender metadata when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|SENDER_ID|Sender identifier|
|TABLE_ID|Target table identifier|
|TABLE_TYPE|Target table type|
|BEGIN_RID|Target record start RID|
|END_RID|Target record end RID|

### V$REPL_RECEIVER
---

Displays Receiver information when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|STATUS|Receiver operational status|
|PAYLOAD_RECV_COUNT|Number of payloads received from sender|
|PAYLOAD_RECV_BYTES|Total payload size received from Sender|
|QUEUE_REMAIN_COUNT|Number of buffers remaining in the Receive Queue|
|NET_SEND_COUNT|Net send count|
|NET_SEND_SIZE|Net send size|
|NET_RECV_COUNT|Net receive count|
|NET_RECV_SIZE|Net receive size|

### V$REPL_RECEIVER_META
---

Displays Receiver metadata when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|TABLE_ID|Target table identifier|
|TABLE_TYPE|Target table type|
|BEGIN_RID|Target record start RID|
|END_RID|Target record end RID|

### V$REPL_READER
---

Displays Reader information when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|SENDER_ID|Sender identifier|
|ID|Reader identifier|
|STATUS|Reader operation status|
|FETCH_COUNT|FETCH count|

### V$REPL_READER_META
---

Displays Reader metadata when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|SENDER_ID|Sender identifier|
|ID|Reader identifier|
|TABLE_ID|Target table identifier|
|TABLE_TYPE|Target table type|
|BEGIN_RID|Target record start RID|
|END_RID|Target record end RID|

### V$REPL_WRITER
---

Displays Writer information when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|ID|Writer identifier|
|STATUS|Writer operational status|
|APPEND_COUNT|APPEND count|

### V$REPL_WRITER_META
---

Displays Writer metadata when running Replication.

|Column Name|Description|
|--|--|
|HOSTNAME|Replication Node Hostname|
|ID|Writer identifier|
|TABLE_ID|Target table identifier|
|TABLE_TYPE|Target table type|
|BEGIN_RID|Target record start RID|
|END_RID|Target record end RID|

## Others

### V$TABLES
---

Displays all Virtual Tables that start with "V$".

|Column Name|Description|
|--|--|
|NAME|Table name|
|TYPE|Table type|
|DATABASE_ID|Database identifier|
|ID|Table identifier|
|USER ID|User who created table|
|COLCOUNT|Column count|

### V$COLUMNS
---

Displays column information of Virtual Tables.

|Column Name|Description|
|--|--|
|NAME|Column name|
|TYPE|Column data type|
|DATABASE_ID|Database identifier|
|ID|Column identifier|
|LENGTH|Column size|
|TABLE_ID|Table identifier|
|FLAG|Private data|
|PART_PAGE_COUNT|Unused|
|PAGE_VALUE_COUNT|Unused|
|MINMAX_CACHE_SIZE|Unused|
|MAX_CACHE_PART_COUNT|Unused|

### V$RETENTION_JOB
---

Displays table information to which RETENTION POLICY is applied.

|Column Name|Description|
|-------------------|------------------------------------------|
| USER_NAME         | User name                              |
| TABLE_NAME        | applied table name                      |
| POLICY_NAME       | applied policy name                |
| STATE             | RETENTION state (RUNNING/WAITING/STOPPED) |
| LAST_DELETED_TIME | most recently deleted time                 |
