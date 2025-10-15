# .NET Connector

## Index

* [Overview](#overview)
* [Install](#install)
* [Install Connector via NuGet Package Manager](#install-connector-via-nuget-package-manager)
* [Connection String Reference](#connection-string-reference)
* [API Reference](#api-reference)
* [Usage and Examples](#usage-and-examples)
* [Full Provider APIs (Protocol 4.0-full)](#full-provider-apis-protocol-40-full)

## Overview

Machbase ships a universal ADO.NET provider, **UniMachNetConnector**, that wraps every supported Machbase wire protocol (2.1 through 4.0). Beginning with Machbase 8.0.50, this universal connector is bundled with the server packages, and the version number appended to the DLL name matches the Machbase release you built or installed. The connector automatically chooses the correct protocol at runtime based on the connection string.

## Install

The Machbase server and client installers include the universal .NET provider under `$MACHBASE_HOME/lib/`. After installation you will see:

- **UniMachNetConnector** – the framework-neutral entry point. The files are named `UniMachNetConnector-net{50|60|70|80}-<version>.dll` so you can pick the build that matches your target framework.
- **Legacy protocol connectors** – optional protocol-specific assemblies that the universal loader can activate on demand, such as `machNetConnector-XX-net{50|60|70|80}-<version>.dll`.

Reference the DLL that matches your application (for example `UniMachNetConnector-net80-8.0.50.dll`) or copy it next to your binaries when you deploy.

## Install Connector via NuGet Package Manager

> **Note**: .NET Connector 5.0 of Machbase has already enrolled to NuGet package! This 5.0 package is the legacy standalone distribution that predates the unified UniMachNetConnector bundled with Machbase 8.0.50.

If you use Visual Studio, you'll easily get and use .NET Connector from NuGet repository. Below procedure is about how to get machNetConnector5.0 from NuGet.

1. In Visual Studio, create a new C# .NET project.
2. When the project is created, activate context menu above project name at Solution Explorer and select "Manage NuGet Packages".
3. When NuGet Package Manager window is activated, select "Browse" tab on the upper left and search "machNet".
4. When the result is displayed on the left pane, select "machNetConnector5.0" and select "Install".
5. If Preview Changes window is activated, just select "OK" to continue to install.
6. When the package was installed successfully, you can confirm it at "Dependencies - Packages" on Solution Explorer.
7. Now, you can use machNetConnector by "using Mach.Data.MachClient" at Program.cs.

## Connection String Reference

Connection-string segments are separated by semicolons (`;`). Keywords listed in the same row are aliases.

| Keyword                                                         | Description                                                                                              | Example                                         | Default |
|-----------------------------------------------------------------|----------------------------------------------------------------------------------------------------------|-------------------------------------------------|---------|
| `DSN`, `SERVER`, `HOST`                                         | Hostname or IP address                                                                                    | `SERVER=127.0.0.1`                              | _(none)_|
| `PORT`, `PORT_NO`                                               | Listener port                                                                                             | `PORT=55656`                                    | `5656`  |
| `USERID`, `USERNAME`, `USER`, `UID`                             | Username                                                                                                  | `UID=SYS`                                       | `SYS`   |
| `PASSWORD`, `PWD`                                               | Password                                                                                                  | `PWD=manager`                                   | _(none)_|
| `CONNECT_TIMEOUT`, `ConnectionTimeout`, `connectTimeout`        | Connection timeout in milliseconds                                                                        | `CONNECT_TIMEOUT=10000`                        | `60000` |
| `COMMAND_TIMEOUT`, `CommandTimeout`, `commandTimeout`           | Per-command timeout in milliseconds                                                                       | `COMMAND_TIMEOUT=50000`                        | `60000` |
| `PROTOCOL`, `ProtocolVersion`, `MachProtocol`                   | Preferred wire protocol (for example `2.1`, `3.0`, `4.0`, `4.0-full`). UniMachNetConnector now defaults to `4.0` when omitted. | `PROTOCOL=4.0-full`                            | `4.0`   |

Example:

```csharp
var connectionString = string.Format(
    "SERVER={0};PORT_NO={1};UID=SYS;PWD=MANAGER;COMMAND_TIMEOUT=50000;PROTOCOL=4.0-full",
    host,
    port);
```

## API Reference

Features not listed below may not be implemented yet or may not work correctly.<br>
If you call a method or field that is not a named instance, it generates NotImplementedException or a NotSupportedException.

### MachConnection

```cs
public sealed class MachConnection : DbConnection
```

This class is responsible for linking with Machbase.

Because it inherits IDisposable like DbConnection, it supports disassociation through Dispose () or automatic disposition of object using using () statement.

#### Constructor
```
MachConnection(string aConnectionString)
```

Creates a MachConnection with a Connection String as input.

#### Open

```cs
void Open()
```

Attempts to connect to the connection string.

#### Close

```cs
void Close()
```

Closes the connection when connecting.

#### SetConnectAppendFlush

```cs
void SetConnectAppendFlush(bool activeFlush)
```

Set flush to be performed automatically during append.

#### Field

| Name | Description |
|--|--|
|State|Represents a System.Data.ConnectionState value.|
|StatusString|Indicates the state to be performed by the connected MachCommand.<br>This is used internally to decorate the Error Message and it is not appropriate to check the status of the query with this value because it indicates the state in which the operation started.|

### MachCommand

```cs
public sealed class MachCommand : DbCommand
```

A class that performs **SQL commands or APPEND** using MachConnection.

Since it inherits IDisposable like DbCommand, it supports object disposal through Dispose () or automatic disposal of object using using () statement

#### Constructor

```cs
MachCommand(string aQueryString, MachConnection)
```

Creates by typing the query to be executed along with the MachConnection object to be connected.

```cs
MachCommand(MachConnection)
```

Creates a MachConnection object to connect to. Use only if there is no query to perform (eg APPEND).

#### CreateParameter

```cs
MachParameter CreateParameter()
```

Creates a new MachParameter.

#### AppendOpen

```cs
MachAppendWriter AppendOpen(aTableName, aErrorCheckCount = 0, MachAppendOption = None)
```

Starts APPEND. Returns a MachAppendWriter object.

* aTableName: Target table name
* aErrorCheckCount: Each time the cumulative number of records entered by APPEND-DATA matches, it is checked whether it is sent to the server or not.<br>
  In other words, you are setting the automatic APPEND-FLUSH point.<br>
* MachAppendOption: Currently only one option is provided.
  * MachAppendOption.None: No options are attached.
  * MachAppendOption.MicroSecTruncated: When inputting the value of a DateTime object, enter the value expressed only up to microsecond.
  (The Ticks value of a DateTime object is expressed up to 100 nanoseconds.)

#### AppendData

```cs
void AppendData(MachAppendWriter aWriter, List<object> aDataList)
```

Through the MachAppendWriter object, it takes a list containing the data and enters it into the database.
- In the order of the data in the List, each datatype must match the datatype of the column represented in the table.
- If the data in the List is insufficient or overflows, an error occurs.

> **Note**: When representing a time value with a ulong object, simply do not enter the Tick value of the DateTime object. In that value, you must enter a value that excludes the DateTime Tick value that represents 1970-01-01.

```cs
void AppendDataWithTime(MachAppendWriter aWriter, List<object> aDataList, DateTime aArrivalTime)
```

Method that explicitly puts an _arrival_time value into a DateTime object in AppendData().

```cs
void AppendDataWithTime(MachAppendWriter aWriter, List<object> aDataList, ulong aArrivalTimeLong)
```

Method that can explicitly put _arrival_time value into a ulong object in AppendData(). Refer to AppendData() above for problems that may occur when typing a ulong value as an _arrival_time value.

#### AppendFlush

```cs
void AppendFlush(MachAppendWriter aWriter)
```

The data entered by AppendData() is immediately sent to the server to force data insert.<br>
The more frequently the call is made, the lower the data loss rate due to the system error and the faster the error check, although the performance is lowered.<br>
The less frequently the call is made, the more likely the data loss will occur and the error checking will be delayed, but the performance will increase significantly.

#### AppendClose

```cs
void AppendClose(MachAppendWriter aWriter)
```

Closes APPEND. Internally, after calling AppendFlush(), the actual protocol is internally finished.

#### ExecuteNonQuery

```cs
int ExecuteNonQuery()
```

Performs the input query. Returns the number of records affected by the query. It is usually used when performing queries except SELECT.

#### ExecuteScalar

```cs
object ExecuteScalar()
```

Performs the input query. Returns the first value of the query targetlist as an object. It is usually used when you want to perform a SELECT query, especially a SELECT (Scalar Query) with only one result, and get the result without a DbDataReader.

#### ExecuteDbDataReader

```cs
DbDataReader ExecuteDbDataReader(CommandBehavior aBehavior)
```

Executes the input query, generates a DbDataReader that can read the result of the query, and returns it.

#### Field

| Name | Description|
|--|--|
| Connection / DbConnection                   | Connected MachConnection.|
| ParameterCollection / DbParameterCollection | The MachParameterCollection to use for the Binding purpose.|
| CommandText                                 | Query string.|
| CommandTimeout                              | The amount of time it takes to perform a particular task, waiting for a response from the server.<br>It follows the values ​​set in MachConnection, where you can only reference values.|
| FetchSize                                   | The number of records to fetch from the server at one time . The default value is 3000.|
| IsAppendOpened                              | Determines if Append is already open when APPEND is at work|

### MachDataReader

```cs
public sealed class MachDataReader : DbDataReader
```

This is a class that reads fetch results. Only objects created with MachCommand.ExecuteDbDataReader () that can not be explicitly created are available.

#### GetName

```cs
string GetName(int ordinal)
```

Returns the ordinal column name.

#### GetDataTypeName

```cs
string GetDataTypeName(int ordinal)
```

Returns the datatype name of the ordinal column.

#### GetFieldType

```cs
Type GetFieldType(int ordinal)
```

Returns the datatype of the ordinal column.

#### GetOrdinal

```cs
int GetOrdinal(string name)
```

Returns the index at which the column name is located.

#### GetValue

```cs
object GetValue(int ordinal)
```

Returns the ordinal value of the current record.

#### IsDBNull

```cs
bool IsDBNull(int ordinal)
```

Returns whether the ordinal value of the current record is NULL.

#### GetValues

```cs
int GetValues(object[] values)
```

Sets all the values ​​of the current record and returns the number.

#### Get*xxxx*

```cs
bool GetBoolean(int ordinal)
byte GetByte(int ordinal)
char GetChar(int ordinal)
short GetInt16(int ordinal)
int GetInt32(int ordinal)
long GetInt64(int ordinal)
DateTime GetDateTime(int ordinal)
string GetString(int ordinal)
decimal GetDecimal(int ordinal)
double GetDouble(int ordinal)
float GetFloat(int ordinal)
```

Returns the ordinal column value according to the datatype.

#### Read

```cs
bool Read()
```

Reads the next record. Returns False if the result does not exist.

#### Field

| Name | Description|
|--|--|
| FetchSize         |The number of records to fetch from the server at one time. The default is 3000, which can not be modified here.|
| FieldCount        |Number of result columns.|
| this[int ordinal] |Equivalent to object GetValue (int ordinal).|
| this[string name] |Equivalent to object GetValue(GetOrdinal(name).|
| HasRows           |Indicates whether the result is present.|
| RecordsAffected   |Unlike MachCommand, here, it represents Fetch Count.|

### MachParameterCollection

```cs
public sealed class MachParameterCollection : DbParameterCollection, IEnumerable<MachParameter>
```

This is a class that binds parameters needed by MachCommand.

If you do this after binding, the values ​​are done together.

> Since the concept of Prepared Statement is not implemented, execution performance after Binding is the same as the performance performed first.

#### Add

```cs
MachParameter Add(string parameterName, DbType dbType)
```

Adds the MachParameter, specifying the parameter name and type. Returns the added MachParameter object.

```cs
int Add(object value)
```

Adds a value. Returns the index added.

```cs
void AddRange(Array values)
```

Adds an array of simple values.

```cs
MachParameter AddWithValue(string parameterName, object value)
```

Adds the parameter name and its value. Returns the added MachParameter object.|

#### Contains

```cs
bool Contains(object value)
```

Determines whether or not the corresponding value is added.

```cs
bool Contains(string value)
```

Determines whether or not the corresponding parameter name is added.

#### Clear

```cs
void Clear()
```

Deletes all parameters.

#### IndexOf

```cs
int IndexOf(object value)
```

Returns the index of the corresponding value.

```cs
int IndexOf(string parameterName)
```

Returns the index of the corresponding parameter name.

#### Insert

```cs
void Insert(int index, object value)
```

Adds the value to a specific index.

#### Remove

```cs
void Remove(object value)
```

Deletes the parameter including the value.

```cs
void RemoveAt(int index)
```

Deletes the parameter located at the index.

```cs
void RemoveAt(string parameterName)
```

Deletes the parameter with that name.

#### Field

| Name                 | Description                 |
| ----------------- | --------------------------------------- |
|Count              | Number of parameters|
|this[int index]    | Indicates the MachParameter at index.|
|this[string name]  | Indicates the MachParameter of the order in which the parameter names match.|

### MachParameter

```cs
public sealed class MachParameter : DbParameter
```

This is a class that contains the information that binds the necessary parameters to each MachCommand.

No special methods are supported.

#### Field

| Name            | Description                                                                          |
| ------------- | --------------------------------------------------------------------------------- |
|ParameterName|Parameter name|
|Value|Value|
|Size|Value size|
|Direction|ParameterDirection (Input / Output / InputOutput / ReturnValue)<br>The default value is Input.|
|DbType|DB Type|
|MachDbType|MACHBASE DB Type<br>May differ from DB Type.|
|IsNullable|Whether nullable|
|HasSetDbType|Whether DB Type is specified|

### MachException

```cs
public class MachException : DbException
```

This is a class that displays errors that appear in Machbase.

An error message is set, and all error messages  can be found in  MachErrorMsg .

#### Field

| Name| Description|
|--|--|
|int MachErrorCode|Error code provided by MACHBASE|

### MachAppendWriter

```cs
public sealed class MachAppendWriter
```

APPEND is supported as a separate class using MachCommand.
This is a class to support MACHBASE Append Protocol, not ADO.NET standard.

It is created with MachCommand's AppendOpen () without a separate constructor.

#### SetErrorDelegator

```cs
void SetErrorDelegator(ErrorDelegateFuncType aFunc)

void ErrorDelegateFuncType(MachAppendException e);
```

Specifies the ErrorDelegateFunc to call when an error occurs.

#### Field

| Name | Description |
|--|--|
|SuccessCount|Number of successful records. Is set after AppendClose().|
|FailureCount|The number of records that failed input. Set after AppendClose ().|
|Option|MachAppendOption received input during AppendOpen()|

### MachAppendException

```cs
public sealed class MachAppendException : MachException
```

Same as MachException, except that:

* An error message is received from the server side.
* A data buffer in which an error has occurred can be obtained. (comma-separated) can be used to process and re-append or record data.

The exception is only available within the ErrorDelegateFunc.

#### GetRowBuffer

```cs
string GetRowBuffer()
```

A data buffer in which an error has occurred can be obtained.

## Usage and Examples

### Connection

You can create a MachConnection and use Open () - Close ().
```c#
String sConnString = String.Format("DSN={0};PORT_NO={1};UID=SYS;PWD=MANAGER;", SERVER_HOST, SERVER_PORT);
MachConnection sConn = new MachConnection(sConnString);
sConn.Open();
//... do something
sConn.Close();
```

If you use the using statement, you do not need to call Close (), which is a connection closing task.
```c#
String sConnString = String.Format("DSN={0};PORT_NO={1};UID=SYS;PWD=MANAGER;", SERVER_HOST, SERVER_PORT);
using (MachConnection sConn = new MachConnection(sConnString))
{
    sConn.Open();
    //... do something
} // you don't need to call sConn.Close();
```

### Executing Queries

Create a MachCommand and perform the query.

```c#
String sConnString = String.Format("DSN={0};PORT_NO={1};UID=SYS;PWD=MANAGER;", SERVER_HOST, SERVER_PORT);
using (MachConnection sConn = new MachConnection(sConnString))
{
    sConn.Open();

    String sQueryString = "CREATE TABLE tab1 ( col1 INTEGER, col2 VARCHAR(20) )";
    MachCommand sCommand = new MachCommand(sQueryString , sConn)
    try
    {
        sCommand.ExecuteNonQuery();
    }
    catch (MachException me)
    {
        throw me;
    }
}
```

Again, using the using statement, MachCommand release can be done immediately.
```c#
String sConnString = String.Format("DSN={0};PORT_NO={1};UID=SYS;PWD=MANAGER;", SERVER_HOST, SERVER_PORT);
using (MachConnection sConn = new MachConnection(sConnString))
{
    sConn.Open();

    String sQueryString = "CREATE TABLE tab1 ( col1 INTEGER, col2 VARCHAR(20) )";
    using(MachCommand sCommand = new MachCommand(sQueryString , sConn))
    {
        try
        {
            sCommand.ExecuteNonQuery();
        }
        catch (MachException me)
        {
            throw me;
        }
    }
}
```

### Executing SELECT
You can get a MachDataReader by executing a MachCommand with a SELECT query.

You can fetch the records one by one through the MachDataReader.
```c#
String sConnString = String.Format("DSN={0};PORT_NO={1};UID=SYS;PWD=MANAGER;", SERVER_HOST, SERVER_PORT);
using (MachConnection sConn = new MachConnection(sConnString))
{
    sConn.Open();

    String sQueryString = "SELECT * FROM tab1;";
    using(MachCommand sCommand = new MachCommand(sQueryString , sConn))
    {
        try
        {
            MachDataReader sDataReader = sCommand.ExecuteReader();
            while (sDataReader.Read())
            {
                for (int i = 0; i < sDataReader.FieldCount; i++)
                {
                    Console.WriteLine(String.Format("{0} : {1}",
                                                    sDataReader.GetName(i),
                                                    sDataReader.GetValue(i)));
                }
            }
        }
        catch (MachException me)
        {
            throw me;
        }
    }
}
```

### Parameter Binding
You can create a MachParameterCollection and then link it to a MachCommand.
```c#
String sConnString = String.Format("DSN={0};PORT_NO={1};UID=SYS;PWD=MANAGER;", SERVER_HOST, SERVER_PORT);
using (MachConnection sConn = new MachConnection(sConnString))
{
    sConn.Open();

    string sSelectQuery = @"SELECT *
        FROM tab2
        WHERE CreatedDateTime < @CurrentTime
        AND CreatedDateTime >= @PastTime";

    using (MachCommand sCommand = new MachCommand(sSelectQuery, sConn))
    {
        DateTime sCurrtime = DateTime.Now;
        DateTime sPastTime = sCurrtime.AddMinutes(-1);

        try
        {
            sCommand.ParameterCollection.Add(new MachParameter { ParameterName = "@CurrentTime", Value = sCurrtime });
            sCommand.ParameterCollection.Add(new MachParameter { ParameterName = "@PastTime", Value = sPastTime });

            MachDataReader sDataReader = sCommand.ExecuteReader();

            while (sDataReader.Read())
            {
                for (int i = 0; i < sDataReader.FieldCount; i++)
                {
                    Console.WriteLine(String.Format("{0} : {1}",
                                                    sDataReader.GetName(i),
                                                    sDataReader.GetValue(i)));
                }
            }
        }
        catch (MachException me)
        {
            throw me;
        }
    }
}
```

### APPEND
When you run AppendOpen () on a MachCommand, you get a MachAppendWriter object.

Using this object and MachCommand, you can get a list of one input record and perform an AppendData().
AppendFlush() will reflect the input of all records, and AppendClose () will end the entire Append process.
```c#
String sConnString = String.Format("DSN={0};PORT_NO={1};UID=SYS;PWD=MANAGER;", SERVER_HOST, SERVER_PORT);
using (MachConnection sConn = new MachConnection(sConnString))
{
    sConn.Open();

    using (MachCommand sAppendCommand = new MachCommand(sConn))
    {
        MachAppendWriter sWriter = sAppendCommand.AppendOpen("tab2");
        sWriter.SetErrorDelegator(AppendErrorDelegator);

        var sList = new List<object>();
        for (int i = 1; i <= 100000; i++)
        {
            sList.Add(i);
            sList.Add(String.Format("NAME_{0}", i % 100));

            sAppendCommand.AppendData(sWriter, sList);

            sList.Clear();

            if (i % 1000 == 0)
            {
                sAppendCommand.AppendFlush();
            }
        }

        sAppendCommand.AppendClose(sWriter);
        Console.WriteLine(String.Format("Success Count : {0}", sWriter.SuccessCount));
        Console.WriteLine(String.Format("Failure Count : {0}", sWriter.FailureCount));
    }
}
```
```c#
private static void AppendErrorDelegator(MachAppendException e)
{
    Console.WriteLine("{0}", e.Message);
    Console.WriteLine("{0}", e.GetRowBuffer());
}
```

### Set Error Delegator

In MachAppendWriter, you can specify a function to detect errors occurring on the MACHBASE server side during APPEND.

In .NET, this function type is specified as a Delegator Function.
```c#
public static void ErrorCallbackFunc(MachAppendException e)
{
    Console.WriteLine("====================");
    Console.WriteLine("Error occured");
    Console.WriteLine(e.Message);
    Console.WriteLine(e.StackTrace);
    Console.WriteLine("====================");
}

public static void DoAppend()
{
    MachCommand com = new MachCommand(conn);
    MachAppendWriter writer = com.AppendOpen("tag", errorCheckCount);
    writer.SetErrorDelegator(ErrorCallbackFunc);
    //... do append
}
```

### Set Auto AppendFlush

If you set `Set Connect Append Flush` to true in the connection, flush is automatically performed during append.

```cs
private static string connString = $"SERVER={HOST};PORT_NO={port};USER={USER};PWD={PWD}";

public static void Main(string[] args)
{
    MachConnection conn = new MachConnection(connString);
    conn.Open();
    conn.SetConnectAppendFlush(true);
}
```

If set to false, the function is disabled.

```cs
conn.SetConnectAppendFlush(false);
```

## Full Provider APIs (Protocol 4.0-full)

The `4.0-full` handshake unlocks the full ADO.NET surface that ships with Machbase 8.0.50 and later. Load one of the assemblies below when you need these provider features:

- `UniMachNetConnector-net80-8.0.50.dll` – bundled with Machbase 8.0.50+.
- `machNetConnector-40-net80-3.2.0.dll` – the standalone connector that exposes the same surface.

### Key types introduced by 4.0-full
- `MachDbProviderFactory` (`Instance`, `Register()`, and the standard `Create*` methods) so frameworks can resolve the connector by invariant name `Mach.Data`.
- `MachConnectionStringBuilder` for strongly typed connection-string edits without remembering every keyword.
- `MachDataAdapter` plus the `MachRowUpdating`/`MachRowUpdated` events for DataTable/DataSet workflows.
- `MachCommandBuilder` to auto-generate INSERT/DELETE (and UPDATE for tables that support it—never for log/tag tables) commands from a SELECT statement.

### Enable the full provider stack
```csharp
var connString = "SERVER=127.0.0.1;PORT_NO=55656;UID=SYS;PWD=MANAGER;PROTOCOL=4.0-full";
using var connection = new MachConnection(connString);
connection.Open();

if (!connection.SupportsFullApi)
{
    throw new InvalidOperationException("Full provider API negotiation failed.");
}
```

Use lookup or volatile tables when you need INSERT/DELETE/UPDATE semantics. Log and tag tables do not accept UPDATE statements, so keep those workloads append-only.

### Build connection strings fluently
```csharp
var builder = new MachConnectionStringBuilder
{
    Server = "127.0.0.1",
    Port = 55656,
    UserID = "SYS",
    Password = "MANAGER"
};

// Protocol stays a string key so older connectors understand the value.
builder["PROTOCOL"] = "4.0-full";

using var connection = new MachConnection(builder.ConnectionString);
connection.Open();
```

### Sample: append rows with MachDataAdapter
This example downloads a lookup table into a `DataTable`, appends a new row, and pushes the change back. The `MachCommandBuilder` auto-generates the INSERT statement. (If the table does not exist yet, create it once: `CREATE LOOKUP TABLE dotnet_lookup_demo(id LONG PRIMARY KEY, name VARCHAR(64));`)

```csharp
using Mach.Data.MachClient;
using System.Data;

var connString = "SERVER=127.0.0.1;PORT_NO=55656;UID=SYS;PWD=MANAGER;PROTOCOL=4.0-full";
using var connection = new MachConnection(connString);
connection.Open();

var adapter = new MachDataAdapter(
    "SELECT id, name FROM dotnet_lookup_demo ORDER BY id",
    connection);
var builder = new MachCommandBuilder(adapter);

var table = new DataTable();
adapter.Fill(table);

var newRow = table.NewRow();
newRow["id"] = 2001;
newRow["name"] = "Inserted from MachDataAdapter";
table.Rows.Add(newRow);

adapter.Update(table);
```

> **Tip**: When you need to inspect or veto outgoing commands, subscribe to `MachDataAdapter.MachRowUpdating` / `MachRowUpdated`.

```csharp
adapter.MachRowUpdating += (sender, args) =>
{
    Console.WriteLine($"About to run {args.StatementType} with SQL: {args.Command?.CommandText}");
};
```

### Sample: work through DbProviderFactory
`MachDbProviderFactory.Instance` lets you plug Machbase into provider-agnostic infrastructure such as `DbProviderFactories`, Dapper, or your own DI container.

```csharp
using System.Data.Common;
using Mach.Data.MachClient;

DbProviderFactory factory = MachDbProviderFactory.Instance;

using DbConnection connection = factory.CreateConnection()!;
connection.ConnectionString = "SERVER=127.0.0.1;PORT_NO=55656;UID=SYS;PWD=MANAGER;PROTOCOL=4.0-full";
connection.Open();

using DbCommand command = connection.CreateCommand();
command.CommandText = "SELECT COUNT(*) FROM dotnet_lookup_demo";
var count = (long)command.ExecuteScalar();

Console.WriteLine($"Lookup rows: {count}");
```

Need to make the factory visible to configuration-driven apps? Call `MachDbProviderFactory.Register()` once during startup so `DbProviderFactories.GetFactory("Mach.Data")` returns the same instance.

Remember that `4.0-full` is only available when you connect to Machbase 7.x or later servers; fall back to `Protocol=4.0` (limited surface) or the 2.x/3.x protocols for older clusters.
