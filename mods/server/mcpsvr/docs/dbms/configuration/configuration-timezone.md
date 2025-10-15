# Timezone

## Index

* [Timezone of Machbase](#timezone-of-machbase)
* [Timezone Format in Machbase](#timezone-format-in-machbase)
    * [machsql](#machsql)
    * [machloader](#machloader)
    * [SDK](#sdk)
    * [Rest API](#rest-api)

## Timezone of Machbase

Machbase assumes that each client's Timezone is valid only in each session.

In general, a time zone is specified as a string representing a specific time.

```
"YYYY-MM-DD HH24:MI:SS ZZZ(Timezone String)"

Example) 
"12:06:56.568+01:00"  
"2006.07.10 at 15:08:56 -05:00"
"09  AM, GMT+09:00"
```

However, the above method not only has the inconvenience of having to designate a specific time based on the time zone every time, but also has a problem in that the amount of data transmission increases linearly when the time zone value is included for a large amount of data.

Therefore, Machbase supports the method of specifying the time zone property for the session in which the client and server are connected.

The following is a step-by-step explanation of the time zone operation provided by Machbase.

* The server operates based on the default time zone provided by the operating system in which the server is installed.<br>
    In other words, if no setting is made, Machbase reads and uses the time zone in which the OS operates.

* If the client program connects to the server without setting the time zone, the client's time zone is set to the server's time zone.<br>
    That is, if the TIMEZONE set in the server is KST, it means that the client also operates in KST.

* If the time zone is explicitly set in the client program, the corresponding session of the server operates in the time zone designated by the client.<br>
    That is, even if the TIMEZONE set in the server is KST, if the client sets the time zone as EDT when connecting, the session operates as EDT.

## Timezone Format in Machbase

Machbase provides only one format consisting of 5 characters to increase ease of use and remove complexity.

That is, the first character is a + or - sign indicating the sign of time, and the following two characters have a value between 00 and 23. And, it is assumed that the last two characters have a time from 00 to 59.

The following shows the format of TIMEZONE supported by Machbase.

```
ex)
TIMEZONE=+0900
TIMEZONE=-0900
```

### machsql
---

When machsql is started, you can set the time zone to operate through the following options.

```
-z, --timezone=+-HHMM
```

You can check the currently set time zone through the SHOW TIMEZONE command.

```
SHOW TIMEZONE;

Mach> show timezone;
Timezone : +0900
```

### machloader
---

When running machloader, you can set the time zone to operate through the following options.

```
-z, --timezone=+-HHMM
```

It connects to the designated time zone and time calculation operates based on the corresponding time zone.

### SDK
---

TIMEZONE has been added to the connection string, and the time zone for the session can be specified.

If TIMEZONE is not specified in the connection string, it operates based on the time zone of the server.

This is the same for CLI, ODBC, JDBC, and DOTNET.

Connection String Example

```
SERVER=127.0.0.1;UID=SYS;PWD=MANAGER;CONNTYPE=1;NLS_USE=UTF8;PORT_NO=5656;TIMEZONE=+0300
```

### Rest API
---

Rest API operates based on the time zone specified in the HTTP protocol HEADER when requesting an operation.

The header is named The-Timezone-Machbase, and the usage is as follows.

```
Authorization: Basic XXXXXXXXXXXXXXXXXXX
...................
The-Timezone-Machbase: +0900
...............
```

As described above, you can specify the desired Timezone string.

If the Timezone is not specified, it operates as the Timezone of the server.

Request example: set to UTC

```
curl -H "The-Timezone-Machbase: +0000" -G "http://127.0.0.1:${ITF_HTTP_PORT}/machbase" --data-urlencode 'q=select * from test_table order by c4 asc';
{
  "error_code": 0,
  "error_message": "",
  "columns": [
    {
      "name": "C1",
      "type": 4,
      "length": 6
    },
    {
      "name": "C2",
      "type": 8,
      "length": 11
    },
    {
      "name": "C3",
      "type": 5,
      "length": 20
    },
    {
      "name": "C4",
      "type": 6,
      "length": 31
    },
    {
      "name": "C5",
      "type": 32,
      "length": 15
    }
  ],
  "timezone": "+0000",
  "data": [
    {
      "C1": 1,
      "C2": 2,
      "C3": "test1",
      "C4": "1999-09-09 00:09:09 000:000:000",
      "C5": "127.0.0.1"
    },
  ]
}
```

The time zone value set in the "timezone" item is returned to the resulting JSON.

