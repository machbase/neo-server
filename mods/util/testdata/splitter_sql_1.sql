CREATE TABLE abs_table (c1 INTEGER, c2 DOUBLE, c3 VARCHAR(10));

INSERT INTO abs_table VALUES(1, 1.0, '');

INSERT INTO abs_table VALUES(2, 2.0, 'sqltest');

INSERT INTO abs_table VALUES(3, 3.0, 'sqltest');

SELECT count(*) - 1 FROM example;
SELECT count(*) -2 FROM example; -- comment
SELECT count(*) / 3 FROM example;
// double slash comment
SELECT count(*) / 4 FROM example; // comment 2
SELECT * FROM example where name = 'contains // slash';
SELECT * FROM example where name = 'contains -- dash';