
# Index

* [CREATE USER](#create-user)
* [DROP USER](#drop-user)
* [ALTER USER](#alter-user)
* [CONNECT](#connect)
* [GRANT/REVOKE](#grantrevoke)
* [Managing User Example](#managing-user-example)

## CREATE USER

**create_user_stmt:**

```sql
create_user_stmt ::= 'CREATE USER' user_name 'IDENTIFIED BY' password
```

The syntax for creating a user is:

```sql
-- Example
CREATE USER new_user IDENTIFIED BY password
```

## DROP USER

**drop_user_stmt:**

```sql
drop_user_stmt ::= 'DROP USER' user_name
```

The syntax for deleting a user is as follows. The SYS user can not be deleted, and if there is a table already created by the user to be deleted, an error is displayed.

```sql
-- Example
DROP USER old_user
```

## ALTER USER

**alter_user_pwd_stmt:**

```sql
alter_user_pwd_stmt ::= 'ALTER USER' user_name 'IDENTIFIED BY' password
```

The user can change the password through the following syntax.

```sql
-- Example
ALTER USER user1 IDENTIFIED BY password
```

## CONNECT

**user_connect_stmt:**

```sql
user_connect_stmt: 'CONNECT' user_name '/' password
```

The user can reconnect to another user via the following syntax without terminating the application.

```sql
-- Example
CONNECT user1/password;
```

## GRANT/REVOKE

Grants authority to the table to the user through the GRANT statement.

```sql
-- Grant user1 SELECT privileges on mytable
GRANT SELECT ON mytable TO user1;
 
-- Grant user1 all privileges on mytable
GRANT ALL ON mytable TO user1;
```

Revokes the privilege granted to a user through the REVOKE statement.

```sql
-- Revoke UPDATE privilege on mytable granted to user1
REVOKE UPDATE ON mytable FROM user1;
 
-- Revoke all privileges on mytable granted to user1
REVOKE ALL ON mytable FROM user1;
```

## Managing User Example

Here is an example of the above query and its results.

```
############################################
## Connect with SYS account
############################################
Mach> create user demo identified by 'demo';
Created successfully.
 
Mach> drop user demo;
Dropped successfully.
 
Mach> create user demo1 identified by 'demo1';
Created successfully.
 
Mach> create user demo2 identified by 'demo2';
Created successfully.
 
Mach> alter user demo2 identified by 'demo22';
Altered successfully.
 
Mach> create table demo1_table (id integer);
Created successfully.
 
Mach> create bitmap index demo1_table_index1 on demo1_table(id);
Created successfully.
 
Mach> insert into demo1_table values(99991);
1 row(s) inserted.
 
Mach> insert into demo1_table values(99992);
1 row(s) inserted.
 
Mach> insert into demo1_table values(99993);
1 row(s) inserted.
 
Mach> select * from demo1_table;
ID
--------------
99993
99992
99991
[3] row(s) selected.
 
#Error: Can't drop the user connected.
Mach> drop user SYS;
[ERR-02083 : Drop user error. You cannot drop yourself(SYS).]
 
############################################
## Connect DEMO1
############################################
Mach> connect demo1/demo1;
Connected successfully.
 
#Error: can't alter other's account password
Mach> alter user demo2 identified by 'demo22';
[ERR-02085 : ALTER user error. The user(DEMO2) does not have ALTER privileges.]
 
Mach> alter user demo1 identified by demo11;
Altered successfully.
 
#Error: wrong password
Mach> connect demo1/demo11234;
[ERR-02081 : User authentication error. Invalid password (DEMO11234).]
 
## Correct password
Mach> connect demo1/demo11;
Connected successfully.
 
Mach> create table demo1_table (id integer);
Created successfully.
 
Mach> create bitmap index demo1_table_index1 on demo1_table(id);
Created successfully.
 
Mach> insert into demo1_table values(1);
1 row(s) inserted.
 
Mach> insert into demo1_table values(2);
1 row(s) inserted.
 
Mach> insert into demo1_table values(3);
1 row(s) inserted.
 
Mach> select * from demo1_table;
ID
--------------
3
2
1
[3] row(s) selected.
 
Mach> select * from demo1.demo1_table;
ID
--------------
3
2
1
[3] row(s) selected.
 
############################################
## Connect SYS again
############################################
Mach> connect SYS/MANAGER;
Connected successfully.
 
Mach> select * from demo1_table;
ID
--------------
99993
99992
99991
[3] row(s) selected.
 
Mach> select * from demo1.demo1_table;
ID
--------------
3
2
1
[3] row(s) selected.
 
Mach> drop user demo1;
[ERR-02084 : DROP user error. The user's tables still exist. Drop those tables first.]
 
Mach> connect demo1/demo11;
Connected successfully.
 
Mach> drop table demo1_table;
Dropped successfully.
 
Mach> connect SYS/MANAGER;
Connected successfully.
 
Mach> drop user demo1;
Dropped successfully.
```
