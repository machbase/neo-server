# Machbase Neo Bridge - MySQL

## Register a bridge to MySQL

Register a bridge that connects to the MySQL database.

The connection string is according to the MySQL specification.

```
bridge add -t mysql my root:password@tcp(127.0.0.1:3306)/mydb?parseTime=true;
```

**Warning**: For handling TIMESTAMP typed column properly, option parameter `parseTime=true` is required.

```
machbase-neo» bridge list;
╭────────┬──────────┬─────────────────────────────────────────────────────────╮
│ NAME   │ TYPE     │ CONNECTION                                              │
├────────┼──────────┼─────────────────────────────────────────────────────────┤
│ my     │ mysql    │ root:password@tcp(127.0.0.1:3306)/mydb?parseTime=true   │
╰────────┴──────────┴─────────────────────────────────────────────────────────╯
```

## Create table

Open machbase-neo shell and execute the command below which creates a `my_example` table via the `my` bridge.

```sh
bridge exec my CREATE TABLE IF NOT EXISTS my_example(
    id         INT NOT NULL AUTO_INCREMENT,
    company    VARCHAR(50) UNIQUE NOT NULL,
    employee   INT,
    discount   REAL,
    plan       FLOAT,
    code       CHAR(64),
    valid      SMALLINT,
    memo       TEXT,
    created_on TIMESTAMP NOT NULL,
    PRIMARY KEY(id)
);
```

```
mysql> desc my_example;
+------------+-------------+------+-----+---------+----------------+
| Field      | Type        | Null | Key | Default | Extra          |
+------------+-------------+------+-----+---------+----------------+
| id         | int         | NO   | PRI | NULL    | auto_increment |
| company    | varchar(50) | NO   | UNI | NULL    |                |
| employee   | int         | YES  |     | NULL    |                |
| discount   | double      | YES  |     | NULL    |                |
| plan       | float       | YES  |     | NULL    |                |
| code       | char(64)    | YES  |     | NULL    |                |
| valid      | smallint    | YES  |     | NULL    |                |
| memo       | text        | YES  |     | NULL    |                |
| created_on | timestamp   | NO   |     | NULL    |                |
+------------+-------------+------+-----+---------+----------------+
9 rows in set (0.01 sec)
```

## *TQL* writing on the MySQL

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
INSERT(bridge("my"), table("my_example"), "company", "employee", "created_on")
```

```
mysql> update my_example set discount=1.234, plan=2.3456, code='0c275c5e-776f-457e-910e-0a95587d60c7', valid=1, memo='This is mysql bridge test';
Query OK, 1 row affected (0.00 sec)

mysql> select id, company, employee, plan, created_on from my_example;
+----+------------+----------+----------+--------+---------------------+
| id | company    | employee | discount | plan   | created_on          |
+----+------------+----------+----------+--------+---------------------+
|  1 | acme       |       10 |    1.234 | 2.3456 | 2023-08-09 05:20:00 |
+----+------------+----------+----------+--------+---------------------+
1 row in set (0.00 sec)
```

```
machbase-neo» bridge query my select id, company, employee, plan, created_on from my_example;
╭────┬────────────┬──────────┬────────┬─────────────────────────╮
│ ID │ COMPANY    │ EMPLOYEE │ PLAN   │ CREATED_ON              │
├────┼────────────┼──────────┼────────┼─────────────────────────┤
│  1 │ acme       │       10 │ 2.3456 │ 2023-08-09 14:20:00 UTC │
╰────┴────────────┴──────────┴────────┴─────────────────────────╯
```

## *TQL* reading from the MySQL

```js
SQL(bridge('my'), "select * from my_example")
CSV()
```