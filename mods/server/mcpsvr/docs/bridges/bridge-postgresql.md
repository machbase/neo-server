# Machbase Neo Bridge - PostgreSQL

## Register a bridge to postgresql

Register a bridge that connects to the postgreSQL database.

```
bridge add -t postgres pg host=127.0.0.1 port=5432 user=dbuser dbname=postgres sslmode=disable;
```

Connect options

| Option            | Description                            | example         |
| :-----------      | :---------------------------------     | :-------------  |
| `dbname`          | The name of the database to connect to |                 |
| `user`            | The user to sign in as                 |                 |
| `password`        | The user's password                    |                 |
| `host`            | The host to connect to. Values that start with / are for unix domain sockets. default is localhost | `host=127.0.0.1` |
| `port`            | The port to bind to. default is `5432` |     |
| `sslmode`         | Whether or not to use SSL (default is `require`)  | (see below) |
| `connect_timeout` | Maximum wait for connection, in seconds. Zero or not specified means wait indefinitely. |  |
| `sslcert`         | Cert file location. The file must contain PEM encoded data.   |  |
| `sslkey`          | Key file location. The file must contain PEM encoded data.    |  |
| `sslrootcert`     | The location of the root certificate file. The file must contain PEM encoded data. |  |

Valid values for `sslmode` are:

| sslmode       |  Description                      |
|:------------  | :---------------------------------|
| `disable`     | No SSL                            |
| `require`     | Always SSL (skip verification)    |
| `verify-ca`   | Always SSL (verify that the certificate presented by the server was signed by a trusted CA) |
| `verify-full` | Always SSL (verify that the certification presented by the server was signed by a trusted CA and the server host name matches the one in the certificate)|

## Create table

Open machbase-neo shell and execute the command below which creates a `pg_example` table via the `pg` bridge.

```sh
bridge exec pg CREATE TABLE IF NOT EXISTS pg_example(
    id         SERIAL PRIMARY KEY,
    company    VARCHAR(50) UNIQUE NOT NULL,
    employee   INT,
    discount   REAL,
    plan       FLOAT(8),
    code       UUID,
    valid      BOOL,
    memo       TEXT,
    created_on TIMESTAMP NOT NULL
);
```

Can make sure the table has been created with `psql` command line tool

```
postgres=# \d pg_example;
                                        Table "public.pg_example"
   Column   |            Type             | Collation | Nullable |                Default                 
------------+-----------------------------+-----------+----------+----------------------------------------
 id         | integer                     |           | not null | nextval('pg_example_id_seq'::regclass)
 company    | character varying(50)       |           | not null | 
 employee   | integer                     |           |          | 
 discount   | real                        |           |          | 
 plan       | real                        |           |          | 
 code       | uuid                        |           |          | 
 valid      | boolean                     |           |          | 
 memo       | text                        |           |          | 
 created_on | timestamp without time zone |           | not null | 
Indexes:
    "pg_example_pkey" PRIMARY KEY, btree (id)
    "pg_example_company_key" UNIQUE CONSTRAINT, btree (company)

```

## *TQL* writing on the PostgreSQL

```js
BYTES(payload() ?? `{
  "company": "acme",
  "employee": 10
}`)
SCRIPT("tengo", {
  // get current time
  times := import("times")
  ts := times.now()
  // get tql context
  ctx := import("context")
  val := ctx.value()
  // parse json
  json := import("json")
  msg := json.decode(val[0])
  ctx.yield(msg.company, msg.employee, ts)
})
INSERT(bridge("pg"), table("pg_example"), "company", "employee", "created_on")
```

```
postgres=# select * from pg_example;
 id | company | employee | discount | plan | code | valid | memo |         created_on         
----+---------+----------+----------+------+------+-------+------+----------------------------
  1 | acme    |       10 |          |      |      |       |      | 2023-08-09 11:05:30.039961
(1 row)
```

## *TQL* reading from the PostgreSQL

```js
SQL(bridge('pg'), "select * from pg_example")
CSV()
```