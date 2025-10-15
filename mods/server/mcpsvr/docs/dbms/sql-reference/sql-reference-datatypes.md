
# Index

* [Data Type Table](#data-type-table)
* [SQL DataType Table](#sql-datatype-table)

## Data Type Table

|Type Name|Description|Value Range|NULL Value|
|--|--|--|--|
|short|16-bit signed integer data type|-32767 ~ 32767|-32768|
|ushort|16-bit unsigned integer type data type|0 ~ 65534|65535|
|integer|32-bit signed integer data type|-2147483647 ~ 2147483647|-2147483648|
|uinteger|32-bit unsigned integer data type|0 ~ 4294967294|4294967295|
|long|64-bit signed integer data type|-9223372036854775807 ~ 9223372036854775807|-9223372036854775808|
|ulong|64-bit unsigned integer data type|0~18446744073709551614|18446744073709551615|
|float|32-bit floating point data type|-|-|
|double|64-bit floating point data type|-|-|
|datetime|Time and date|1970-01-01 00:00:00 000:000:000 ~ 2262-04-11 23:47:16.854:775:807|-|
|varchar|Variable-length character strings (UTF-8)|Length : 1 ~ 32768 (32K)|-|
|ipv4|Version 4 Internet address type (4 bytes)|"0.0.0.0" ~ "255.255.255.255"|-|
|ipv6|Version 6 Internet address type (16 bytes)|"0000:0000:0000:0000:0000:0000:0000:0000" ~ "FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF:FFFF"|-|
|text|Text data type (keyword index can be generated)|Length : 0 ~ 64M|-|
|binary|Binary data type  (index creation not possible)|Length: 0 ~ 64M|-|
|json|json data type|json data length : 1 ~ 32768 (32K)<br><br>json path length : 1 ~ 512|-|

### short

This is the same as the 16-bit signed integer data of the C language. For the minimum negative value, it is recognized as NULL. May be displayed as "int16".

### integer

This is the same as 32-bit signed integer data in C language. For the minimum negative value, it is recognized as NULL. May be displayed as "int32" or "int".

### long

This is the same as 64-bit signed integer data in C language. For the minimum negative value, it is recognized as NULL. May be displayed as "int64".

### float

This is equivalent to the C 32-bit floating-point data type float. For a positive maximum value, it is recognized as NULL.

### double

This is equivalent to the 64-bit floating-point data type double of C language. For a positive maximum value, it is recognized as NULL.

### datetime

In Machbase, this type maintains the nano value of the time elapsed since midnight January 1, 1970.

Thus, Machbase provides the ability to process values ​​up to nano units for all datetime type related functions.

### varchar

This is a variable string data type and can be generated up to 32K bytes in length.

This length criterion is based on one character in English, so it is different from the actual number of characters to be output in UTF-8 and should be set to an appropriate length.

### IPv4

This type is a type that can store addresses used in Internet Protocol version 4.

It is internally represented using 4 bytes, and can be expressed from "0.0.0.0" to "255.255.255.255".

### IPv6

This type is a type that can store addresses used in Internet Protocol version 6.

16 bytes are internally represented and can be expressed from "0000: 0000: 0000: 0000: 0000: 0000: 0000: 0000" to "FFFF: FFFF: FFFF: FFFF: FFFF: FFFF: FFFF: FFFF" .
Since the abbreviation type is also supported when inputting data, it can be expressed as follows using the symbol :.

* ":: FFFF: 1232": all leading with zeros 
* ":: FFFF: 192.168.0.3": Support for IPv4 type compatibility 
* ":: 192.168.3.1": Support for deprecated IPv4 type compatibility

### text

This type is a data type for storing text or documents beyond the size of a VARCHAR.

This data type can be searched through keyword indexes and can store up to 64 megabytes of text. 
This type is mainly used to store and retrieve large text files as separate columns.

### binary

This type is a supported type for storing unstructured data in columns.

It is used to store binary data such as image, video, or audio. Indexes can not be created for this type. 
The maximum data size for storing is up to 64 megabytes, the same as the TEXT type.

### json

This type is a data type for storing json data.

Json is a format to store data object, consisting of "Key-Value" pairs, into text format.

The maximum size of data is 32K bytes which is same as varchar type.

## SQL Datatype Table

The following table shows the SQL data types and C data types corresponding to the mark base data types.

|Machbase Datatype|Machbase CLI Datatype|SQL Datatype|C Datatype|Basic types for C|Description|
|--|--|--|--|--|--|
|short|SQL_SMALLINT|SQL_SMALLINT|SQL_C_SSHORT|int16_t (short)|16-bit signed integer data type|
|ushort|SQL_USMALLINT|SQL_SMALLINT|SQL_C_USHORT|uint16_t (unsigned short)|16-bit unsigned integer type data type|
|integer|SQL_INTEGER|SQL_INTEGER|SQL_C_SLONG|int32_t (int)|32-bit signed integer data type|
|uinteger|SQL_UINTEGER|SQL_INTEGER|SQL_C_ULONG|uint32_t (unsigned int)|32-bit unsigned integer data type|
|long|SQL_BIGINT|SQL_BIGINT|SQL_C_SBIGINT|int64_t (long long)|64-bit signed integer data type|
|ulong|SQL_UBIGINT|SQL_BIGINT|SQL_C_UBIGINT|uint64_t (unsigned long long)|64-bit unsigned integer data type|
|float|SQL_FLOAT|SQL_REAL|SQL_C_FLOAT|float|32-bit floating point data type|
|double|SQL_DOUBLE|SQL_FLOAT, SQL_DOUBLE|SQL_C_DOUBLE|double|64-bit floating point data type|
|datetime|SQL_TIMESTAMP<br><br>SQL_TIME|SQL_TYPE_TIMESTAMP<br><br>SQL_BIGINT<br><br>SQL_TYPE_TIME|SQL_C_TYPE_TIMESTAMP<br><br>SQL_C_UBIGINT<br><br>SQL_C_TIME|char * (YYYY-MM-DD HH24:MI:SS)<br><br>int64_t (timestamp: nano seconds)<br>struct tm|Time and date|
|varchar|SQL_VARCHAR|SQL_VARCHAR|SQL_C_CHAR|char *|String|
|ipv4|SQL_IPV4|SQL_VARCHAR|SQL_C_CHAR|char * (enter ip string)<br><br>unsigned char[4]|Version 4 Internet address type|
|ipv6|SQL_IPV6|SQL_VARCHAR|SQL_C_CHAR|char * (enter ip string)<br><br>unsigned char[16]|Version 6 Internet address type|
|text|SQL_TEXT|SQL_LONGVARCHAR|SQL_C_CHAR|char *|Text|
|binary|SQL_BINARY|SQL_BINARY|SQL_C_BINARY|char *|Binary data|
|json|SQL_JSON|SQL_JSON|SQL_C_CHAR|json_t|json data type|
