# Property

The properties are the settings used by the Machbase server and stored as key-value pairs in the $MACHBASE_HOME/conf/machbase.conf file. 
These values are set when the Machbase server starts and are used continuously during runtime. To change this value for performance tuning, you must understand the meaning of these values and set them carefully.

## Index

- [Index](#index)
- [CPU_AFFINITY_BEGIN_ID](#cpu_affinity_begin_id)
- [CPU_AFFINITY_COUNT](#cpu_affinity_count)
- [CPU_COUNT](#cpu_count)
- [CPU_PARALLEL](#cpu_parallel)
- [DBS_PATH](#dbs_path)
- [DEFAULT_LSM_MAX_LEVEL](#default_lsm_max_level)
- [DISK_BUFFER_COUNT](#disk_buffer_count)
- [DISK_COLUMNAR_INDEX_CHECKPOINT_INTERVAL_SEC](#disk_columnar_index_checkpoint_interval_sec)
- [DISK_COLUMNAR_INDEX_FDCACHE_COUNT](#disk_columnar_index_fdcache_count)
- [DISK_COLUMNAR_INDEX_SHUTDOWN_BUILD_FINISH](#disk_columnar_index_shutdown_build_finish)
- [DISK_COLUMNAR_PAGE_CACHE_MAX_SIZE](#disk_columnar_page_cache_max_size)
- [DISK_COLUMNAR_TABLE_CHECKPOINT_INTERVAL_SEC](#disk_columnar_table_checkpoint_interval_sec)
- [DISK_COLUMNAR_TABLE_COLUMN_FDCACHE_COUNT](#disk_columnar_table_column_fdcache_count)
- [DISK_COLUMNAR_TABLE_COLUMN_MINMAX_CACHE_SIZE](#disk_columnar_table_column_minmax_cache_size)
- [DISK_COLUMNAR_TABLE_COLUMN_PART_FLUSH_MODE](#disk_columnar_table_column_part_flush_mode)
- [DISK_COLUMNAR_TABLE_COLUMN_PART_IO_INTERVAL_MIN_SEC](#disk_columnar_table_column_part_io_interval_min_sec)
- [DISK_COLUMNAR_TABLE_TIME_INVERSION_MODE](#disk_columnar_table_time_inversion_mode)
- [DISK_COLUMNAR_TABLESPACE_DWFILE_EXT_SIZE](#disk_columnar_tablespace_dwfile_ext_size)
- [DISK_COLUMNAR_TABLESPACE_DWFILE_INT_SIZE](#disk_columnar_tablespace_dwfile_int_size)
- [DISK_COLUMNAR_TABLESPACE_MEMORY_EXT_SIZE](#disk_columnar_tablespace_memory_ext_size)
- [DISK_COLUMNAR_TABLESPACE_MEMORY_MAX_SIZE](#disk_columnar_tablespace_memory_max_size)
- [DISK_COLUMNAR_TABLESPACE_MEMORY_MIN_SIZE](#disk_columnar_tablespace_memory_min_size)
- [DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_HIGH_LIMIT_PCT](#disk_columnar_tablespace_memory_slowdown_high_limit_pct)
- [DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_MSEC](#disk_columnar_tablespace_memory_slowdown_msec)
- [DISK_IO_THREAD_COUNT](#disk_io_thread_count)
- [DISK_TABLESPACE_DIRECT_IO_FSYNC](#disk_tablespace_direct_io_fsync)
- [DISK_TABLESPACE_DIRECT_IO_READ](#disk_tablespace_direct_io_read)
- [DISK_TABLESPACE_DIRECT_IO_WRITE](#disk_tablespace_direct_io_write)
- [DUMP_APPEND_ERROR](#dump_append_error)
- [DUMP_TRACE_INFO](#dump_trace_info)
- [DURATION_BEGIN](#duration_begin)
- [DURATION_GAP](#duration_gap)
- [FEEDBACK_APPEND_ERROR](#feedback_append_error)
- [GRANT_REMOTE_ACCESS](#grant_remote_access)
- [HTTP_THREAD_COUNT](#http_thread_count)
- [INDEX_BUILD_MAX_ROW_COUNT_PER_THREAD](#index_build_max_row_count_per_thread)
- [INDEX_BUILD_THREAD_COUNT](#index_build_thread_count)
- [INDEX_FLUSH_MAX_REQUEST_COUNT_PER_INDEX](#index_flush_max_request_count_per_index)
- [INDEX_LEVEL_PARTITION_AGER_THREAD_COUNT](#index_level_partition_ager_thread_count)
- [INDEX_LEVEL_PARTITION_BUILD_MEMORY_HIGH_LIMIT_PCT](#index_level_partition_build_memory_high_limit_pct)
- [INDEX_LEVEL_PARTITION_BUILD_THREAD_COUNT](#index_level_partition_build_thread_count)
- [LOOKUP_APPEND_UPDATE_ON_DUPKEY](#lookup_append_update_on_dupkey)
- [MAX_QPX_MEM](#max_qpx_mem)
- [MEMORY_ROW_TEMP_TABLE_PAGESIZE](#memory_row_temp_table_pagesize)
- [PID_PATH](#pid_path)
- [PORT_NO](#port_no)
- [PROCESS_MAX_SIZE](#process_max_size)
- [QUERY_PARALLEL_FACTOR](#query_parallel_factor)
- [ROLLUP_FETCH_COUNT_LIMIT](#rollup_fetch_count_limit)
- [RS_CACHE_APPROXIMATE_RESULT_ENABLE](#rs_cache_approximate_result_enable)
- [RS_CACHE_ENABLE](#rs_cache_enable)
- [RS_CACHE_MAX_MEMORY_PER_QUERY](#rs_cache_max_memory_per_query)
- [RS_CACHE_MAX_MEMORY_SIZE](#rs_cache_max_memory_size)
- [RS_CACHE_MAX_RECORD_PER_QUERY](#rs_cache_max_record_per_query)
- [RS_CACHE_TIME_BOUND_MSEC](#rs_cache_time_bound_msec)
- [SHOW_HIDDEN_COLS](#show_hidden_cols)
- [TABLE_SCAN_DIRECTION](#table_scan_direction)
- [TAGDATA_AUTO_META_INSERT](#tagdata_auto_meta_insert)
- [TAG_TABLE_META_MAX_SIZE](#tag_table_meta_max_size)
- [TAG_PARTITION_COUNT](#tag_partition_count)
- [TAG_DATA_PART_SIZE](#tag_data_part_size)
- [TRACE_LOGFILE_COUNT](#trace_logfile_count)
- [TRACE_LOGFILE_PATH](#trace_logfile_path)
- [TRACE_LOGFILE_SIZE](#trace_logfile_size)
- [UNIX_PATH](#unix_path)
- [VOLATILE_TABLESPACE_MEMORY_MAX_SIZE](#volatile_tablespace_memory_max_size)

## CPU_AFFINITY_BEGIN_ID

This is the start number of the CPU used by the Machbase server. It is used to control the CPU usage of the Machbase server.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## CPU_AFFINITY_COUNT

This is the number of CPUs that the Machbase server will use. If set to 0, the Machbase server uses all CPUs.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## CPU_COUNT

Specifies the number of CPUs set in the system. Based on this value, the Machbase Thread determines the number. If set to 0, all CPUs in the system are used.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0(auto detect the physically installed count of CPU on  the system)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## CPU_PARALLEL

Specifies the number of threads to spawn per CPU. If this value is 2 and the number of CPUs is 2, then two parallel threads are created per CPU, so the number of parallel processing threads is four. If this value is too large, memory can be consumed quickly.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1</td>
    </tr>
  </tbody>
</table>

## DBS_PATH

Specifies the path where the basic data of the Machbase server will be stored. The default is "? Dbs",  which means $MACHBASE_HOME/dbs.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>?/dbs</td>
    </tr>
  </tbody>
</table>

## DEFAULT_LSM_MAX_LEVEL

Sets the base level of the LSM index. If you do not enter a MAX_LEVEL value when creating an index, this value applies.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>3</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>2</td>
    </tr>
  </tbody>
</table>

## DISK_BUFFER_COUNT

Specifies the number of buffers for disk I/O.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>4G (4 * 1024 * 1024 * 1024)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>16</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_INDEX_CHECKPOINT_INTERVAL_SEC

Sets the checkpoint interval for the index. If set too long, errors may occur during index creation.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1 (sec)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 -1 (sec)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>120 (sec)</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_INDEX_FDCACHE_COUNT

Specifies the number of opened index partition file descriptors.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_INDEX_SHUTDOWN_BUILD_FINISH

Sets whether or not to reflect index information on the disk when the Machbase server is shutdown. If this value is set to '1', all index information is reflected on the disk and ends, so waiting times may be long.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (false)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1 (True)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0 (False)</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_PAGE_CACHE_MAX_SIZE

Sets the maximum size of the page cache.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>2 * 1024 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLE_CHECKPOINT_INTERVAL_SEC

Sets checkpoint period of table data. If this value is too large, the recovery time will be longer at restart. If this value is too small, I/O will frequently occur and the overall performance may be degraded.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1 (sec)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 32 - 1 (sec)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>120 (sec)</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLE_COLUMN_FDCACHE_COUNT

Specifies the maximum number of open file descriptors for column data in the table.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLE_COLUMN_MINMAX_CACHE_SIZE
Sets the size of the default MINMAX cache set in the _ARRIVAL_TIME column.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2 ^ 64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>100 *1024 * 1024</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLE_COLUMN_PART_FLUSH_MODE

Sets the automatic flush interval for column partition files.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (sec)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32-1 (sec)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>60 (sec)</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLE_COLUMN_PART_IO_INTERVAL_MIN_SEC
Sets the frequency with which the partition file is reflected on the disk. When more data is input than the number of partitions set, it is reflected on the disk regardless of this period.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (sec)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32-1 (sec)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3 (sec)</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLE_TIME_INVERSION_MODE

If set to 1, the input is allowed even if the value of the _ARRIVAL_TIME column is reduced. If it is 0, a value smaller than the Maximum of the _ARRIVAL_TIME column value is entered as an error.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (False)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1 (True)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1 (True)</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLESPACE_DWFILE_EXT_SIZE

Specifies the size at which the double write file used for recovery at startup increases at one time.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1024 * 1024</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLESPACE_DWFILE_INT_SIZE
Specifies the amount of space secured by the double write file when the file is created.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>2 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLESPACE_MEMORY_EXT_SIZE

Specifies the block size of the memory to reserve for the column partition.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>2 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLESPACE_MEMORY_MAX_SIZE

Specifies the maximum amount of memory allocated by the log table. If the server allocates more than this amount of memory, the memory allocation will wait until the memory usage drops below this value. It is recommended to set this value to 50 ~ 80% of physical memory.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>256 * 1024 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>8 * 1024 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLESPACE_MEMORY_MIN_SIZE

When the Machbase server starts, it pre-allocates memory by this value to prevent performance degradation due to memory allocation. Since this memory is used only as a data input buffer, it is recommended to use it only when memory is sufficient.

Table 24. Range of values

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>100 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_HIGH_LIMIT_PCT

Limits the performance when the memory usage exceeds the set value when data is input to the log table.  

```c
DISK_COLUMNAR_TABLESPACE_MEMORY_MAX_SIZE * (DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_HIGH_LIMIT_PCT / 100)
```

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>100</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>80</td>
    </tr>
  </tbody>
</table>

## DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_MSEC

Sets the next wait time for each record entry if the memory usage for the column data file exceeds the criterion.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (msec)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1 (msec)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1 (msec)</td>
    </tr>
  </tbody>
</table>

## DISK_IO_THREAD_COUNT

Sets the number of I/O threads that write data to disk.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3</td>
    </tr>
  </tbody>
</table>

## DISK_TABLESPACE_DIRECT_IO_FSYNC

When running Direct I/O, fsync is unnecessary for data files. Disable fsync when using Direct I/O to improve data I/O performance (Set to 0). 
Although fsync is unncessary, fsync must be set to perform in case of failure situations such as a power outage because in a normal situation there is no data loss,

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## DISK_TABLESPACE_DIRECT_IO_READ

Sets whether to use DIRECT I/O for data read operation.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## DISK_TABLESPACE_DIRECT_IO_WRITE

Sets whether to use DIRECT I/O for data write operation. If DIRECT I/O is not supported on the file system (ex: ZFS), it must be set to 0.

||Value|
|-|----|
|Minimum|    0|  
|Maximum|    1|  
|Default|    1|

## DUMP_APPEND_ERROR
If this value is set to 1, the $MACHBASE_HOME/trc/machbase.trc file will record the error if the Append API fails.
In this situation, the append performance is very low, so it is recommended to use for testing purposes only.

If you want to check for errors in the user application,  it is helpful to use the SQLAppendSetErrorCallback API.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## DUMP_TRACE_INFO

The server periodically records the DBMS system status information in the machbase.trc file at regular intervals, and sets this period. 
If it is set to 0, it is not recorded.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (sec)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1 (sec)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>60 (sec)</td>
    </tr>
  </tbody>
</table>

## DURATION_BEGIN

Sets the start time of the duration value that sets the default for the SELECT statements that do not specify the DURATION clause.
If set to 60, data will be retrieved 60 seconds before the current time.

The default is 0 to retrieve all data.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## DURATION_GAP
Sets the start time of the duration value that sets the default for the SELECT statements that do not specify the DURATION clause.

* If set to 60, data will be retrieved for 60 seconds from the current time.
* If the DURATION_BEGIN value is 60, the data is retrieved from 60 seconds before to 60 seconds from the current time.

The default is 0 to retrieve all data.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>Non-zero</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## FEEDBACK_APPEND_ERROR

Sets whether to send error data to the client when an Append API error occurs. If 0, no error data is sent to the client. If it is 1, error information is sent to the client.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1</td>
    </tr>
  </tbody>
</table>

## GRANT_REMOTE_ACCESS

Determines whether the database can be accessed remotely. If 0, the remote connection is blocked.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (False)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1 (True)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1 (True)</td>
    </tr>
  </tbody>
</table>

## HTTP_THREAD_COUNT

Set the number of threads to be used by the Machbase web server.

||Value|
|-|----|
|Minimum| 0|  
|Maximum| 1024|
|Default| 32| 

## INDEX_BUILD_MAX_ROW_COUNT_PER_THREAD
If the number of records not indexed is greater than this value, the index build thread begins to add indexes.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>100000</td>
    </tr>
  </tbody>
</table>

## INDEX_BUILD_THREAD_COUNT
Specifies the number of index creation threads. If set to 0, no index is created.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3</td>
    </tr>
  </tbody>
</table>

## INDEX_FLUSH_MAX_REQUEST_COUNT_PER_INDEX
Specifies the maximum number of flush requests per index.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3</td>
    </tr>
  </tbody>
</table>

## INDEX_LEVEL_PARTITION_AGER_THREAD_COUNT
Specifies the number of threads to delete index files that are not needed when creating LSM indexes.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1</td>
    </tr>
  </tbody>
</table>

## INDEX_LEVEL_PARTITION_BUILD_MEMORY_HIGH_LIMIT_PCT
Sets the maximum memory usage for LSM index creation as a percent. This percent is set based on the maximum memory usage used by Machbase. If the memory usage exceeds the limit, the LSM partition merge is stopped.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>100</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>70</td>
    </tr>
  </tbody>
</table>

## INDEX_LEVEL_PARTITION_BUILD_THREAD_COUNT
Determines the number of threads performing the merge operation for the creation of the LSM index.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3</td>
    </tr>
  </tbody>
</table>

## LOOKUP_APPEND_UPDATE_ON_DUPKEY
When appending to the lookup table, it specifies how to handle duplicate primary keys.

* 0 : Append fail
* 1 : Update Row for the corresponding Primary Key.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## MAX_QPX_MEM

Sets the maximum amount of memory used by the query processor to perform the GROUP BY, DISTINCT, and ORDER BY clauses. 
If one query uses memory with a larger value, the query is canceled. At this time, an error message is sent to the client, and the relevant content is recorded in the machbase.trc file.

||Value|
|--|----|
|Minimum|    1024 * 1024|
|Maximum|    2^64 - 1|
|Default|    500 * 1024 * 1024|

## MEMORY_ROW_TEMP_TABLE_PAGESIZE
Sets the page size of the temporary tablespace for volatile tables and lookup tables. Because this page stores volatile tables and lookup table records, it should be larger than the maximum record size for volatile tables.
If you want to enter N records into the page, you should set this value to the maximum record size * N.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>8 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>32 * 1024</td>
    </tr>
  </tbody>
</table>

## PID_PATH
Specifies the location where the PID file of the Machbase server process is to be written. The default is "?/Conf", which means $MACHBASE_HOME/conf.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>?/conf</td>
    </tr>
  </tbody>
</table>

<table>
  <thead>
    <th style="background-color: lightyellow;">PID_PATH Value</th>
    <th style="background-color: lightyellow;">PID File Location Path</th>
  </thead>
  <tbody>
    <tr>
      <td>Not Specified</td>
      <td>$MACHBASE_HOME/conf/machbase.pid</td>
    </tr>
    <tr>
      <td>?/test</td>
      <td>$MACHBASE_HOME/test/machbase.pid</td>
    </tr>
    <tr>
      <td>/tmp</td>
      <td>/tmp/machbase.pid</td>
    </tr>
  </tbody>
</table>

## PORT_NO
Specifies the TCP/IP port for the Machbase server process to communicate with the client. The Default is 5656.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>65535</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>5656</td>
    </tr>
  </tbody>
</table>

## PROCESS_MAX_SIZE
Specifies the maximum memory size used by machbased programs that are Machbase server processes. If you try to use more memory than the set limit, the server operates as follows to reduce the memory usage.

* Stops data insert or treats it as an error
* Decreased index creation speed

In this case, the performance is greatly degraded, so the cause of overuse of the memory must be found and solved.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024 * 1024 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>8 * 1024 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## QUERY_PARALLEL_FACTOR
Specifies the number of execution threads of the parallel query executor.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>100</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>8</td>
    </tr>
  </tbody>
</table>

## ROLLUP_FETCH_COUNT_LIMIT
Limits the amount of data the rollup thread can fetch at one time.

If set to 0, there is no limit.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3000000</td>
    </tr>
  </tbody>
</table>

## RS_CACHE_APPROXIMATE_RESULT_ENABLE
Determines whether to use the approximate result mode of the result cache. If this value is 1, the speculative value is obtained (very fast but the data may be inaccurate) when using the result cache, and if it is 0, the correct value is obtained.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (false)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1 (True)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0 (False)</td>
    </tr>
  </tbody>
</table>

## RS_CACHE_ENABLE
Determines whether to use the result cache.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (false)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1 (True)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1 (True)</td>
    </tr>
  </tbody>
</table>

## RS_CACHE_MAX_MEMORY_PER_QUERY
Sets the amount of memory the result cache will use. If the memory usage of a particular query result exceeds this value, the result of the query is not stored in the result cache.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>16 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## RS_CACHE_MAX_MEMORY_SIZE
Specifies the maximum memory usage of the result cache.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>32 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>512 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## RS_CACHE_MAX_RECORD_PER_QUERY
The maximum number of records to be stored in the result cache. If the number of records resulting from the query is greater than this value, the query result is not stored in the cache.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>10000</td>
    </tr>
  </tbody>
</table>

## RS_CACHE_TIME_BOUND_MSEC
If a particular query is executed very quickly, it is better not to store it in the result cache because it can reduce memory usage.
This value determines how fast the query executed should not be stored in the cache. When set to 0, all query results are stored in the result cache.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1 (msec)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1 (msec)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000 (msec)</td>
    </tr>
  </tbody>
</table>

## SHOW_HIDDEN_COLS
If set to the Default of 0, the _ARRIVAL_TIME column is not displayed by the SELECT * FROM query. If this value is set to 1, the corresponding column is displayed.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## TABLE_SCAN_DIRECTION
You can set the scan direction of the tag table. The property value is one of -1, 0, and 1, and the default value is 0.

* -1 : Reverse scan
* 0  : Tag Table(Forward scan), Log Table(Reverse scan)
* 1  : Forward scan

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>-1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## TAGDATA_AUTO_META_INSERT

In 5.5 version, this property name was TAGDATA_AUTO_NAME_INSERT and supports only 0 or 1.
Below 5.7 version, default value is 1.

When entering data through APPEND / INSERT into the TAGDATA table, specify how to handle it if there is no matching TAG_NAME.

* 0: Input fails.
* 1: Input TAG_NAME value to input. If there are additional metadata columns, the values of all columns are entered as NULL.
* 2: Enter the additional metadata column value along with the TAG_NAME value you want to enter.
    * This setting is valid only in APPEND. INSERT works like 1 because you cannot enter additional metadata column values.
    * After this setting, the APPEND parameter must include the metadata column value in APPEND.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1</td>
    </tr>
  </tbody>
</table>

## TAG_TABLE_META_MAX_SIZE

When creating the TAGDATA table, set the maximum size of memory to store the metadata area.

||Value|
|-|----|
|Minimum|    1024*1024|
|Maximum|    2^32-1|
|Default|    100\*1024\*1024|

## TAG_PARTITION_COUNT

Specify the number of Key Value tables that consist the tag table.

||Value|
|--|--|
|Minimum| 1|
|Maximum| 4|
|Default| 1024 |

## TAG_DATA_PART_SIZE

Determines the partition size in tag data storage.

||Value|
|--|--|
|Minimum| 1048576 (1MB)|
|Maximum| 1073741824 (1GB)|
|Default| 16777216 (16MB) |

## TRACE_LOGFILE_COUNT

Specifies the maximum number of log trace files generated in TRACE_LOGFILE_PATH. To save disk space, delete the oldest log file if more than the maximum number of log files are created.

If more than the maximum number of log trace files is created and the oldest file is deleted, the name of the deleted file is saved as the newest log file.

||Value|
|-|----|
|Minimum|    1|
|Maximum|    2^32 - 1|
|Default|    1000|

## TRACE_LOGFILE_PATH
Set the path of the log trace files (machbase.trc, machadmin.trc, machsql.trc). 
These files continuously record internal information at the start, end, and run of Machbase. The default ?/trc  means $MACHBASE_HOME/trc.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>?/conf</td>
    </tr>
  </tbody>
</table>

<table>
  <thead>
    <th style="background-color: lightyellow;">TRACE_LOGFILE_PATH </th>
    <th style="background-color: lightyellow;">trc direction location</th>
  </thead>
  <tbody>
    <tr>
      <td>Not Specified</td>
      <td>$MACHBASE_HOME/trc/</td>
    </tr>
    <tr>
      <td>?/test</td>
      <td>$MACHBASE_HOME/test/</td>
    </tr>
    <tr>
      <td>/tmp</td>
      <td>/tmp/</td>
    </tr>
  </tbody>
</table>

## TRACE_LOGFILE_SIZE
Sets the maximum size of the log trace file. If it is necessary to record more data than the size, a new log file is created.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>10 * 1024 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>10 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>

## UNIX_PATH
Sets the path to the Unix domain socket file. The Default when not set by user is ?/conf/machbase-unix.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>?/conf/machbase-unix</td>
    </tr>
  </tbody>
</table>

## VOLATILE_TABLESPACE_MEMORY_MAX_SIZE
Sets the total amount of memory usage for all volatile and lookup tables in the system.

<table>
  <thead>
    <th> </th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>2 * 1024 * 1024 * 1024</td>
    </tr>
  </tbody>
</table>
