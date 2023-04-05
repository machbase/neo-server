javac -classpath .:./postgresql-42.5.4.jar PostgreSQL.java \
&& \
java \
    -classpath .:./postgresql-42.5.4.jar \
    -Djdbc.drivers=org.postgresql.Driver \
    -Djava.util.logging.config.file=./logging.properties \
    PostgreSQL