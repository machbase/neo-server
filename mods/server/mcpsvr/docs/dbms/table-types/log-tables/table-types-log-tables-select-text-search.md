# Text Search

This document deals with text search using keyword indexes.

Text search is faster than comparable DBMS LIKE search because it searches a special kind of index called "reverse index" to search the desired string pattern. Keyword indexes can only be created for varchar and text type columns, which are variable-length character columns. However, the search target string must match exactly. Machbase does not perform keywords based on special characters or morphological analysis.

## SEARCH

```sql
SELECT  column_name(s)
FROM    table_name
WHERE   column_name
SEARCH  pattern;
```

```sql
Mach> CREATE TABLE search_table (id INTEGER, name VARCHAR(20));
Created successfully.
 
Mach> CREATE INDEX idx_SEARCH ON SEARCH_table (name) INDEX_TYPE KEYWORD;
Created successfully.
 
Mach> INSERT INTO search_table VALUES(1, 'time flys');
1 row(s) inserted.
 
Mach> INSERT INTO search_table VALUES(1, 'time runs');
1 row(s) inserted.
 
Mach> SELECT * FROM search_table WHERE name SEARCH 'time' OR name SEARCH 'runs2' ;
ID          NAME
-------------------------------------
1           time runs
1           time flys
[2] row(s) selected.
 
Mach> SELECT * FROM search_table WHERE name SEARCH 'time' AND name SEARCH 'runs2' ;
ID          NAME
-------------------------------------
[0] row(s) selected.
 
Mach> SELECT * FROM search_table WHERE name SEARCH 'flys' OR name SEARCH 'runs2' ;
ID          NAME
-------------------------------------
1           time flys
[1] row(s) selected.
```

## Multilingual Search

Machbase can search variable-length strings of various kinds of languages ​​stored in ASCII and UTF-8. In order to search only part of a sentence in a language such as Korean or Japanese, a 2-gram technique is used.

```sql
SELECT  column_name(s)
FROM    table_name
WHERE   column_name
SEARCH  pattern;
```

```sql
Mach> CREATE TABLE multi_table (message varchar(100));
Created successfully.
 
Mach> CREATE INDEX idx_multi ON multi_table(message)INDEX_TYPE KEYWORD;
Created successfully.
 
Mach> INSERT INTO multi_table VALUES("Machbase is the combination of ideal solutions");
1 row(s) inserted.
 
Mach> INSERT INTO multi_table VALUES("Machbase is a columnar DBMS");
1 row(s) inserted.
 
Mach> INSERT INTO multi_table VALUES("Machbaseは理想的なソリューションの組み合わせです");
1 row(s) inserted.
 
Mach> INSERT INTO multi_table VALUES("Machbaseは円柱状のDBMSです");
1 row(s) inserted.
 
Mach>  SELECT * from multi_table WHERE message SEARCH 'Machbase DBMS';
MESSAGE
------------------------------------------------------------------------------------
Machbaseは円柱状のDBMSです
Machbase is a columnar DBMS
[2] row(s) selected.
 
Mach> SELECT * from multi_table WHERE message SEARCH 'DBMS is';
MESSAGE
------------------------------------------------------------------------------------
Machbase is a columnar DBMS
[1] row(s) selected.
 
Mach> SELECT * from multi_table WHERE message SEARCH 'DBMS' OR message SEARCH 'ideal';
MESSAGE
------------------------------------------------------------------------------------
Machbaseは円柱状のDBMSです
Machbase is a columnar DBMS
Machbase is the combination of ideal solutions
[3] row(s) selected.
 
Mach> SELECT * from multi_table WHERE message SEARCH '組み合わせ';
MESSAGE
------------------------------------------------------------------------------------
Machbaseは理想的なソリューションの組み合わせです
[1] row(s) selected.
Elapsed time: 0.001
Mach> SELECT * from multi_table WHERE message SEARCH '円柱';
MESSAGE
------------------------------------------------------------------------------------
Machbaseは円柱状のDBMSです
[1] row(s) selected.
```

When the input data is "대한민국", three words of "대한," "한민," and "민국" are recorded in the index. Therefore, you can search for "대한민국" with the keywords "대한" or "민국".

Basically, the keywords entered in the search statement are searched by the AND condition, so even if you enter only three words, the result is displayed very accurately. For example, if the search target keyword is a "computer utilization guide", the three words "computer", "utilization", and "guide" are set as AND conditions.

## ESEARCH

The ESEARCH operator is used to expand the search target keyword. The search target keyword must be ASCII. Search keywords can be set using the % character. Using a keyword that begins with the % character, such as the LIKE conditional, searches all records, but searches for this condition on words in the keyword index, which makes searching faster than LIKE. This feature is useful for quickly searching for alphabet strings (such as error statements or code).

```sql
SELECT  column_name(s)
FROM    table_name
WHERE   column_name
ESEARCH pattern;
```

```sql
Mach> CREATE TABLE esearch_table(id INTEGER, name VARCHAR(20), data VARCHAR(40));
Created successfully.
 
Mach> CREATE INDEX idx1 ON esearch_table(name) INDEX_TYPE KEYWORD;
Created successfully.
 
Mach> CREATE INDEX idx2 ON esearch_table(data) INDEX_TYPE KEYWORD;
Created successfully.
 
Mach> INSERT INTO esearch_table VALUES(1, 'machbase', 'Real-time search technology');
1 row(s) inserted.
 
Mach> INSERT INTO esearch_table VALUES(2, 'mach2flux', 'Real-time data compression');
1 row(s) inserted.
 
Mach> INSERT INTO esearch_table VALUES(3, 'DB MS', 'Memory cache technology');
1 row(s) inserted.
 
Mach> INSERT INTO esearch_table VALUES(4, 'ファ ッションアドバイザー、', 'errors');
1 row(s) inserted.
 
Mach> INSERT INTO esearch_table VALUES(5, '인피 니 플럭스', 'socket232');
1 row(s) inserted.
 
Mach> SELECT * FROM esearch_table where name ESEARCH '%mach';
ID          NAME                  DATA
--------------------------------------------------------------------------------
1           machbase            Real-time search technology
[1] row(s) selected.
Elapsed time: 0.001
Mach> SELECT * FROM esearch_table where data ESEARCH '%echn%';
ID          NAME                  DATA
--------------------------------------------------------------------------------
3           DB MS                 Memory cache technology
1           machbase            Real-time search technology
[2] row(s) selected.
 
Mach> SELECT * FROM esearch_table where name ESEARCH '%피니%럭스';
ID          NAME                  DATA
--------------------------------------------------------------------------------
[0] row(s) selected.
 
Mach> SELECT * FROM esearch_table where data ESEARCH '%232';
ID          NAME                  DATA
--------------------------------------------------------------------------------
5           인피 니 플럭스  socket232
[1] row(s) selected.
```

## REGEXP

The REGEXP operator is used to perform a text search on data through a regular expression. The REGEXP operator is executed by performing a regular expression on the target column, and because the index is not available, the search performance may be degraded. Therefore, it is a good idea to add another search condition that can use the index as an AND operator to improve the search speed.

Applying a SEARCH or ESEARCH operator that can use an index before searching for a particular regular expression pattern is a good way to improve search performance by first reducing the result set and then using REGEXP.

```sql
Mach> CREATE TABLE regexp_table(id INTEGER, name VARCHAR(20), data VARCHAR(40));
Created successfully.
 
Mach> INSERT INTO regexp_table VALUES(1, 'machbase', 'Real-time search technology');
1 row(s) inserted.
 
Mach> INSERT INTO regexp_table VALUES(2, 'mach2base', 'Real-time data compression');
1 row(s) inserted.
 
Mach> INSERT INTO regexp_table VALUES(3, 'DBMS', 'Memory cache technology');
1 row(s) inserted.
 
Mach> INSERT INTO regexp_table VALUES(4, 'ファ ッショ', 'errors');
1 row(s) inserted.
 
Mach> INSERT INTO regexp_table VALUES(5, '인피니플럭스', 'socket232');
1 row(s) inserted.
 
Mach> SELECT * FROM regexp_table WHERE name REGEXP 'mach';
ID          NAME                  DATA
--------------------------------------------------------------------------------
2           mach2base           Real-time data compression
1           machbase            Real-time search technology
[2] row(s) selected.
 
Mach> SELECT * FROM regexp_table WHERE data REGEXP 'mach[1]';
ID          NAME                  DATA
--------------------------------------------------------------------------------
[0] row(s) selected.
 
Mach> SELECT * FROM regexp_table WHERE data REGEXP '[A-Za-z]';
ID          NAME                  DATA
--------------------------------------------------------------------------------
5           인피니플럭스  socket232
4           ファ ッショ      errors
3           DBMS                  Memory cache technology
2           mach2base           Real-time data compression
1           machbase            Real-time search technology
[5] row(s) selected.
```

## LIKE

Machbase also supports the SQL standard LIKE operator. The LIKE operator is available in Korean, Japanese, and Chinese.

```sql
SELECT  column_name(s)
FROM    table_name
WHERE   column_name
LIKE    pattern;
```

Example:

```sql
Mach> CREATE TABLE like_table (id INTEGER, name VARCHAR(20), data VARCHAR(40));
Created successfully.
 
Mach> INSERT INTO like_table VALUES(1, 'machbase', 'Real-time search technology');
1 row(s) inserted.
 
Mach> INSERT INTO like_table VALUES(2, 'mach2base', 'Real-time data compression');
1 row(s) inserted.
 
Mach> INSERT INTO like_table VALUES(3, 'DBMS', 'Memory cache technology');
1 row(s) inserted.
 
Mach> INSERT INTO like_table VALUES(4, 'ファ ッションアドバイザー、', 'errors');
1 row(s) inserted.
 
Mach> INSERT INTO like_table VALUES(5, '인피 니 플럭스', 'socket232');
1 row(s) inserted.
 
Mach> SELECT * FROM like_table WHERE name LIKE 'mach%';
ID          NAME                  DATA
--------------------------------------------------------------------------------
2           mach2base           Real-time data compression
1           machbase            Real-time search technology
[2] row(s) selected.
 
Mach> SELECT * FROM like_table WHERE name LIKE '%니%';
ID          NAME                  DATA
--------------------------------------------------------------------------------
5           인피 니 플럭스  socket232
[1] row(s) selected.
 
Mach> SELECT * FROM like_table WHERE data LIKE '%technology';
ID          NAME                  DATA
--------------------------------------------------------------------------------
3           DBMS                  Memory cache technology
1           machbase            Real-time search technology
[2] row(s) selected.
```
