# Machbase Neo HTTP Create, Drop Table

The HTTP "Query" API doesn't accept only "SELECT" SQL but also DDL. So it is possible to create and drop tables via HTTP API

## Create table

Please refer to the docs for understanding what is the [Tag Tables](/dbms/feature-table/tag/)

**Request**

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=create tag table EXAMPLE (name varchar(40) primary key, time datetime basetime, value double)
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode \
    "q=create tag table EXAMPLE (name varchar(40) primary key, time datetime basetime, value double)"
```

**Response**

```json
{"success":true,"reason":"Created successfully.","elapse":"92.489922ms"}
```

### IF NOT EXISTS

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=create tag table if not exists EXAMPLE (name varchar(40) primary key, time datetime basetime, value double)
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode \
    "q=create tag table if not exists EXAMPLE (name varchar(40) primary key, time datetime basetime, value double)"
```

### TAG STATISTICS

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    q=create tag table EXAMPLE (name varchar(40) primary key, time datetime basetime, value double summarized)
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode \
    "q=create tag table EXAMPLE (name varchar(40) primary key, time datetime basetime, value double summarized)"
```

**Note** The keyword "summarized" refers to the automatic generation of statistics on the internal tag data structure when data is written into the corresponding tag table. For more detailed information, please refer to the link below. [Tag Statistics](/dbms/feature-table/tag/manipulate/extract/#display-statistical-information-by-specific-tag-id)

## Drop table

**Request**

**HTTP:**
~~~
```http
GET http://127.0.0.1:5654/db/query
    ?q=drop table EXAMPLE
```
~~~

**cURL:**
```sh
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=drop table EXAMPLE"
```

**Response**

```json
{"success":true,"reason":"Dropped successfully.","elapse":"185.37292ms"}
```