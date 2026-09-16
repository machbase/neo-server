/******************************************************************************
* Copyright of this product 2013-2023,
 * MACHBASE Corporation(or Inc.) or its subsidiaries.
 * All Rights reserved.
 ******************************************************************************/

#ifndef _MMI_ENGINE_H_
#define _MMI_ENGINE_H_

#define MACH_DATA_TYPE_INT16    0
#define MACH_DATA_TYPE_INT32    1
#define MACH_DATA_TYPE_INT64    2
#define MACH_DATA_TYPE_DATETIME 3
#define MACH_DATA_TYPE_FLOAT    4
#define MACH_DATA_TYPE_DOUBLE   5
#define MACH_DATA_TYPE_IPV4     6
#define MACH_DATA_TYPE_IPV6     7
#define MACH_DATA_TYPE_STRING   8
#define MACH_DATA_TYPE_BINARY   9
#define MACH_DATA_TYPE_UINT16   10
#define MACH_DATA_TYPE_UINT32   11
#define MACH_DATA_TYPE_UINT64   12
#define MACH_DATA_TYPE_TEXT     13
#define MACH_DATA_TYPE_JSON     14
#define MACH_DATA_TYPE_MAX      15

typedef struct MachEngineAppendVarStruct
{
    unsigned int    mLength;
    void*           mData;
} MachEngineAppendVarStruct;

/*
 * mLength: length of IP value
 * if mLength is MACH_ENGINE_APPEND_IP_IPV4 or MACH_ENGINE_APPEND_IP_IPV6, mAddr is used
 */
typedef struct MachEngineAppendIPStruct
{
    unsigned char   mLength;
    unsigned char   mAddr[16];
    char*           mAddrString;
} MachEngineAppendIPStruct;
#define MACH_ENGINE_APPEND_IP_NULL 0        /* null */
#define MACH_ENGINE_APPEND_IP_IPV4 4        /* IPV4 value */
#define MACH_ENGINE_APPEND_IP_IPV6 6        /* IPV6 value */
#define MACH_ENGINE_APPEND_IP_STRING 255    /* string value */

/*
 * mTime: nano timestamp value or MACH_ENGINE_APPEND_DATETIME_STRING
 * if mTime is MACH_ENGINE_APPEND_DATETIME_STRING, mDateStr and mFormatStr are used 
 */
typedef struct MachEngineAppendDateTimeStruct
{
    long long   mTime;
    char*       mDateStr;
    char*       mFormatStr;
} MachEngineAppendDateTimeStruct;
#define MACH_ENGINE_APPEND_DATETIME_DEFAULT 0
#define MACH_ENGINE_APPEND_DATETIME_NOW -1       /* arrival time will be set as current time at server */
#define MACH_ENGINE_APPEND_DATETIME_STRING -2    /* string value */

typedef union MachEngineAppendParamData
{
    short                           mShort;
    unsigned short                  mUShort;
    int                             mInteger;
    unsigned int                    mUInteger;
    long long                       mLong;
    unsigned long long              mULong;
    float                           mFloat;
    double                          mDouble;
    MachEngineAppendIPStruct        mIP;
    MachEngineAppendVarStruct       mVar;
    MachEngineAppendVarStruct       mVarchar;
    MachEngineAppendVarStruct       mText;
    MachEngineAppendVarStruct       mJson;
    MachEngineAppendVarStruct       mBinary;
    MachEngineAppendVarStruct       mBlob;
    MachEngineAppendVarStruct       mClob;
    MachEngineAppendDateTimeStruct  mDateTime;
} MachEngineAppendParamData;

typedef struct MachEngineAppendParam
{
    int                         mIsNull; /* 1: NULL, 0: NOT NULL*/
    MachEngineAppendParamData   mData;
} MachEngineAppendParam;

#define MACH_ENGINE_COLUMN_NAME_MAX_LENGTH 100
typedef struct MachEngineColumnInfo
{
    char    mColumnName[MACH_ENGINE_COLUMN_NAME_MAX_LENGTH+1];  /* column name */
    int     mColumnType;                                        /* column type */
    int     mColumnSize;                                        /* column size */
    int     mColumnLength;                                      /* length of fetched column value */
} MachEngineColumnInfo;

#define MACH_OPT_NONE            (0)
#define MACH_OPT_SIGHANDLER_MASK (3)
#define MACH_OPT_SIGHANDLER_ON   (0)
#define MACH_OPT_SIGHANDLER_OFF  (1)
#define MACH_OPT_SIGINT_OFF      (2)

/**
 * @brief Initialize DB Handle
 * @param [in] aHomePath the path of machbase home
 * @param [in] aPortNo the port number of machbase server, it takes effect only when aPortNo > 0
 * @param [in] aOpt MACH_OPT_XXXX options can be bitwise-ored)
 * @param [out] aEnvHandle to be allocated
 */
int MachInitialize(char* aHomePath, int aPortNo, int aOpt, void** aEnvHandle);

/**
 * Finalize DB Handle
 * aEnvHandle will be freed
 */
void MachFinalize(void* aEnvHandle);

/**
 * @brief create and destroy machbase database
 */
int MachCreateDB(void* aEnvHandle);
int MachDestroyDB(void* aEnvHandle);

/**
 * return 1 if DB is create, otherwise return 0
 */
int MachIsDBCreated(void* aEnvHandle);

int MachRestoreDB(void* aEnvHandle, char* aDBPath);

/**
 * @brief startup machbase DB
 */
int MachStartupDB(void* aEnvHandle);

/**
 * @brief shutdown machbase DB
 */
int MachShutdownDB(void* aEnvHandle);

/**
 * @brief connect machbase DB
 */
int MachConnect(void*  aEnvHandle,
                char*  aUserName,
                char*  aPassword,
                void** aConHandle);

/**
 * connect w/o password
 */
int MachConnectNoAuth(void*  aEnvHandle,
                      char*  aUserName,
                      void** aConHandle);

/**
 * @brief disconnect machbase DB
 */
int MachDisconnect(void* aConHandle);

/**
 * @brief Cancel Session execution 
 */
int MachCancel(void* aConHandle);


/*
 * DB user authentification
 * return value is 0 or error code,
 * 0: id and password are correct
 * 2080: user does not exist
 * 2081: password is not correct
 */
int MachUserAuth(void* aEnvHandle,
                 char* aUserName,
                 char* aPassword);

/**
 * These functions retrieve the error code and error message after error occurs.
 * It must be called after MachInitialize() success.
 * @param [in] env handle or stmt handle
 * @return error code / error msg
 */
int MachErrorCode(void* aHandle);
char* MachErrorMsg(void* aHandle);

/**
 * @brief get current connected handle count
 */
int MachGetConnectionCount(void* aEnvHandle);

/*************************SQL Function*********************************/

/**
 * @brief allocate and free statement
 */
int MachAllocStmt(void* aConHandle, void** aMachStmt);
int MachFreeStmt(void* aMachStmt);

/**
 * @brief prepare statement
 * @param [in] aMachStmt statement handle
 * @param [in] aSQL SQL string to prepare
 */
int MachPrepare(void* aMachStmt, char* aSQL);

/**
 * @brief execute statement 
 * @param [in] aMachStmt statement handle
 */
int MachExecute(void* aMachStmt);
int MachExecuteClean(void* aMachStmt);

/**
 * @brief direct execute
 * @param [in] aSQL SQL string
 */
int MachDirectExecute(void* aMachStmt, char* aSQL);

/**
 * It must be called after the statement is PREPARED or APPEND_OPEN
 * @param [out] aStmtType stmt type to be stored
 *
 * DDL: 1-255
 * ALTER SYSTEM: 256-511
 * SELECT: 512
 * INSERT: 513
 * DELETE: 514-517
 * INSERT_SELECT: 518
 * UPDATE: 519
 * EXEC_ROLLUP: 1000-1002
 */
int MachStmtType(void* aMachStmt, int* aStmtType);

/**
 * @brief retrieve the number of result rows
 * @param [in] aMachStmt statement handle
 * @param [out] aEffectRows number of results rows to be stored
 */
int MachEffectRows(void* aMachStmt, unsigned long long* aEffectRows);

/**
 * @brief fetch the result from executed SELECT query
 * @param [in] aMachStmt statement handle
 * @param [out] aFetchEnd 0 if record exists, otherwise 1
 */
int MachFetch(void* aMachStmt, int* aFetchEnd);

/**
 * retrieve the number of columns
 * @param [in] aMachStmt statement handle
 * @param [out] aColumnCount the result to be stored
 */
int MachColumnCount(void* aMachStmt, int* aColumnCount);

/**
 * @brief retrieve the column information from fetched row
 * @param [in] aMachStmt statement handle
 * @param [in] aColumnIndex column index (start at 0)
 * @param [out] aColumnName column name to be stored
 * @param [in] aBufSize size of aColumName
 * @param [out] aType column type
 * @param [out] aSize column size 
 * @param [out] aColumnLength length of column value
 * @param [out] aColumnInfo data structure for column information
 */
int MachColumnName(void* aMachStmt, int aColumnIndex, char* aColumnName, int aColumnNameBufSize);
int MachColumnType(void* aMachStmt, int aColumnIndex, int* aColumnType, int* aColumnSize);
int MachColumnLength(void* aMachStmt, int aColumnIndex, int* aColumnLength);
int MachColumnInfo(void* aMachStmt, int aColumnIndex, MachEngineColumnInfo* aColumnInfo);

/**
 * @brief retrieve the column value from fetched row
 * @param [in] aMachStmt statement handle
 * @param [in] aColumnIndex column index
 * @param [out] aDest column value to be stored
 * @param [out] aIsNull 1 if the column value is NULL, otherwise 0
 */
int MachColumnData(void* aMachStmt, int aColumnIndex, void* aDest, int aBufSize, char* aIsNull);

/**
 * @brief retrieve the column value by column type
 */
int MachColumnDataInt16(void* aMachStmt, int aColumnIndex, short* aDest, char* aIsNull);
int MachColumnDataInt32(void* aMachStmt, int aColumnIndex, int* aDest, char* aIsNull);
int MachColumnDataInt64(void* aMachStmt, int aColumnIndex, long long* aDest, char* aIsNull);
int MachColumnDataUInt16(void* aMachStmt, int aColumnIndex, unsigned short* aDest, char* aIsNull);
int MachColumnDataUInt32(void* aMachStmt, int aColumnIndex, unsigned int* aDest, char* aIsNull);
int MachColumnDataUInt64(void* aMachStmt, int aColumnIndex, unsigned long long* aDest, char* aIsNull);
int MachColumnDataDateTime(void* aMachStmt, int aColumnIndex, long long* aDest, char* aIsNull);
int MachColumnDataFloat(void* aMachStmt, int aColumnIndex, float* aDest, char* aIsNull);
int MachColumnDataDouble(void* aMachStmt, int aColumnIndex, double* aDest, char* aIsNull);
int MachColumnDataIPV4(void* aMachStmt, int aColumnIndex, void* aDest, char* aIsNull);
int MachColumnDataIPV6(void* aMachStmt, int aColumnIndex, void* aDest, char* aIsNull);
int MachColumnDataString(void* aMachStmt, int aColumnIndex, char* aDest, int aBufSize, char* aIsNull);
int MachColumnDataBinary(void* aMachStmt, int aColumnIndex, void* aDest, int aBufSize, char* aIsNull);

/**
 * @brief bind value on bind variable in prepared statement
 * @param [in] aMachStmt statement handle
 * @param [in] aParamNo bind variable index
 * @param [in] aValue bind value
 */
int MachBindInt32(void* aMachStmt, int aParamNo, int aValue);
int MachBindInt64(void* aMachStmt, int aParamNo, long long aValue);
int MachBindDouble(void* aMachStmt, int aParamNo, double aValue);
int MachBindString(void* aMachStmt, int aParamNo, char* aValue, int aLength);
int MachBindBinary(void* aMachStmt, int aParamNo, void* aValue, int aLength);
int MachBindNull(void* aMachStmt, int aParamNo);

/**
 * @brief EXPLAIN query
 * @param [in] aMachStmt statement handle
 * @param [out] aBuffer EXPLAIN result will be set
 * @param [in] aBufSize sepcifies the size of aBuffer
 * @param [in] aExplainMode (0: explain only, 1: explain full)
 */
int MachExplain(void* aMachStmt, char* aBuffer, int aBufSize, int aExplainMode);

/*************************Append Function*********************************/

/**
 * Append functions are used to to insert data in fast manner.
 * MachAppendOpen should be called before Append data,
 * MachAppendClose should be called when Append is finished.
 * @param [in] aMachStmt statement handle
 * @param [in] aTableName target table
 * @param [out] aAppendSuccessCount success count
 * @param [out] aAppendFailureCount failure count
 */
int MachAppendOpen(void* aMachStmt, char* aTableName);
int MachAppendClose(void* aMachStmt,
                    unsigned long long* aAppendSuccessCount,
                    unsigned long long* aAppendFailureCount);

/**
 * @brief Append data
 * @details the value of _ARRIVAL_TIME must be set in append buffer to append on LOG tables.
 * @param [in] aMachStmt statement handle
 * @param [in] aAppendParamArr append buffer which contains data to append
 */
int MachAppendData(void* aMachStmt, MachEngineAppendParam* aAppendParamArr);

unsigned long long MachSessionID(void* aConHandle);

#endif
