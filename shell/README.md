# Install

## Go

```sh
go install github.com/machbase/neo-server/v8/shell@latest
```

# Run

## Interactive

```sh
neo-shell \
    --user sys \
    --password manager \
    --server 127.0.0.1:5654 \
    -v /tmp=./workdir \
```

## Run

```sh
neo-shell \
    --user sys \
    --password manager \
    --server 127.0.0.1:5654 \
    -v /tmp=./workdir \
sql --format csv \
    --output /tmp/output.csv.gz \
    --compress gzip \
    --no-pause \
    --no-footer \
    "select * from example limit 100"
```