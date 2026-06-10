create user user_a identified by 'password';

connect user_a/password;

create table table1 (id integer);
insert into table1 values (1);
insert into table1 values (2);
insert into table1 values (3);

select * from table1;

connect sys/manager;

sql --format csv select * from user_a.table1;

drop table user_a.table1;
drop user user_a;