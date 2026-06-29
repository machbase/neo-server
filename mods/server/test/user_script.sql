create user user_a identified by 'password';

connect user_a/password;

create table neo_scope_t1_ua (id integer);
insert into neo_scope_t1_ua values (1);
insert into neo_scope_t1_ua values (2);
insert into neo_scope_t1_ua values (3);

select * from neo_scope_t1_ua;

connect sys/manager;

sql --format csv select * from neo_scope_t1_ua;

insert into user_a.neo_scope_t1_ua values (4);

sql --format csv select * from user_a.neo_scope_t1_ua;

drop table user_a.neo_scope_t1_ua;
drop user user_a;