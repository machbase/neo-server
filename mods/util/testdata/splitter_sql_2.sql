SELECT 1; SELECT 2 FROM T WHERE name = '--abc';
-- comment

SELECT *  -- start of statement
FROM
	table 
WHERE
	name = 'a;b--c'; -- end of statement

-- env: bridge_bad=sqlite
SELECT 4;
-- env: reset

-- env: bridge=postgres
SELECT 5 FROM T WHERE id = 1;
-- env: bridge=mysql
SELECT 6 FROM T WHERE id = 2;
-- env: reset

-- env: bridge=ms-sql
SELECT 7
FROM T WHERE id = 3;
-- env: reset

-- env: db=mydb
SELECT 8 FROM T WHERE id = 4;
-- env: bridge=sqlite db=otherdb
SELECT 9 FROM T WHERE id = 5;
-- env: reset

wrong statement
