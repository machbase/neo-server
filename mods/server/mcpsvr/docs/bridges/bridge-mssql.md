# Machbase Neo Bridge - MSSQL

## Register a bridge to MSSQL

Register a bridge that connects to the MSSQL database.

The connection string is according to the MSSQL specification.

```
bridge add -t mssql  ms server=127.0.0.1:1433 user=sa pass=changeme database=master encrypt=disable
```

**Connect options**

| Option               | Aliases                |Description                            | example                 |
| :-----------         | :-----------           |:---------------------------------     | :-------------          |
| `server`             |                        | MSSQL Server address                  | `server=127.0.0.1:1433` |
| `database`           |                        | Database name                         | `database=master`       |
| `user id`            | `user`, `user-id`      | User                                  | `user=sa`               |
| `password`           | `pass`                 | The user's password                   | `password=changeme`     |
| `connection timeout` | `connection-timeout`   | DB connection timeout in seconds      | `connection-timeout=5`  |
| `dial timeout`       | `dial-timeout`         | TCP handshake in seconds              | `dial-timeout=3`        |
| `app name`           | `app-name`             | App name (default is `neo-bridge`)    |                         |
| `encrypt`            |                        | Encryption Mode (`disable`, `true`, `false`)  | (see below)  |

- `encrypt`
  - `disable` Data send between client and server is not encrypted.
  - `false` Data sent between client and server is not encrypted beyond the login packet. 
  - `true` Data sent between client and server is encrypted.

```
machbase-neo» bridge list;
╭────────┬──────────┬───────────────────────────────────────────────────────────╮
│ NAME   │ TYPE     │ CONNECTION                                                │
├────────┼──────────┼───────────────────────────────────────────────────────────┤
│ ms     │ mssql    │ server=127.0.0.1:1433 user=SA pass=secret database=master │
╰────────┴──────────┴───────────────────────────────────────────────────────────╯
```

Test connectivity

```
machbase-neo» bridge test ms;
Test bridge ms connectivity... success 3.042458ms
```

## Create table

Open machbase-neo shell and execute the command below which creates a `ms_example` table via the `ms` bridge.

```sh
bridge exec ms CREATE TABLE ms_example(
    id         INT NOT NULL PRIMARY KEY,
    company    VARCHAR(50) UNIQUE NOT NULL,
    employee   INT,
    discount   REAL,
    pricePlan  NUMERIC(7,2),
    code       BINARY,
    valid      SMALLINT,
    memo       TEXT,
    created_on DATETIME NOT NULL,
    UNIQUE(company)
);
```

```
machbase-neo» bridge query ms select * from ms_example;
╭────┬─────────┬──────────┬──────────┬───────────┬──────┬───────┬──────┬────────────╮
│ ID │ COMPANY │ EMPLOYEE │ DISCOUNT │ PRICEPLAN │ CODE │ VALID │ MEMO │ CREATED_ON │
├────┼─────────┼──────────┼──────────┼───────────┼──────┼───────┼──────┼────────────┤
╰────┴─────────┴──────────┴──────────┴───────────┴──────┴───────┴──────┴────────────╯
```

## *TQL* writing on the MSSQL

```js
BYTES(payload() ?? `{
  "id":1,
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
  ctx.yield(msg.id, msg.company, msg.employee, ts)
})
INSERT(bridge("ms"), table("ms_example"), "id", "company", "employee", "created_on")
```

```
machbase-neo» bridge query ms select id, company, employee, created_on from ms_example;
╭────┬─────────┬──────────┬───────────────────────────────────╮
│ ID │ COMPANY │ EMPLOYEE │ CREATED_ON                        │
├────┼─────────┼──────────┼───────────────────────────────────┤
│  1 │ acme    │       10 │ 2023-08-11 20:55:49.527 +0900 KST │
╰────┴─────────┴──────────┴───────────────────────────────────╯
```

## *TQL* reading from the MSSQL

```js
SQL(bridge('ms'), "select * from ms_example")
CSV()
```