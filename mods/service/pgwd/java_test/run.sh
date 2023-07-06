javac -classpath .:./postgresql-42.6.0.jar PostgreSQL.java \
&& \
java \
    -classpath .:./postgresql-42.6.0.jar \
    -Djdbc.drivers=org.postgresql.Driver \
    -Djava.util.logging.config.file=./logging.properties \
    PostgreSQL