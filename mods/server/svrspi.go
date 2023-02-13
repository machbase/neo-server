package server

import (
	"context"
	"os"
	"runtime"
	"time"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-grpc/spi"
	"github.com/machbase/neo-server/mods"
)

var startupTime time.Time = time.Now()

type DB struct {
	m *mach.Database
}

type ROWS struct {
	m *mach.Rows
}

type ROW struct {
}

func (db *DB) GetServerInfo() (*spi.ServerInfo, error) {
	rsp := &spi.ServerInfo{}
	v := mods.GetVersion()
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)

	rsp.Version = spi.Version{
		Major: int32(v.Major), Minor: int32(v.Minor), Patch: int32(v.Patch),
		GitSHA:         v.GitSHA,
		BuildTimestamp: mods.BuildTimestamp(),
		BuildCompiler:  mods.BuildCompiler(),
		Engine:         mods.EngineInfoString(),
	}

	rsp.Runtime = spi.Runtime{
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Pid:            int32(os.Getpid()),
		UptimeInSecond: int64(time.Since(startupTime).Seconds()),
		Processes:      int32(runtime.GOMAXPROCS(-1)),
		Goroutines:     int32(runtime.NumGoroutine()),
		MemSys:         mem.Sys,
		MemHeapSys:     mem.HeapSys,
		MemHeapAlloc:   mem.HeapAlloc,
		MemHeapInUse:   mem.HeapInuse,
		MemStackSys:    mem.StackSys,
		MemStackInUse:  mem.StackInuse,
	}
	return rsp, nil
}

func (db *DB) Explain(sqlText string) (string, error) {
	return db.m.Explain(sqlText)
}

func (db *DB) Exec(sqlText string, params ...any) error {
	result := db.m.Exec(sqlText, params...)
	// TODO returns result itself
	return result.Err()
}

func (db *DB) ExecContext(ctx context.Context, sqlText string, params ...any) error {
	// TODO returns result and apply context
	result := db.m.Exec(sqlText, params...)
	return result.Err()
}

func (db *DB) Query(sqlText string, params ...any) (spi.Rows, error) {
	result, err := db.m.Query(sqlText, params...)
	if err != nil {
		return nil, err
	}
	return &ROWS{m: result}, nil
}

func (db *DB) QueryContext(ctx context.Context, sqlText string, params ...any) (spi.Rows, error) {
	// TODO apply context
	result, err := db.m.Query(sqlText, params)
	if err != nil {
		return nil, err
	}
	return &ROWS{m: result}, nil
}

func (db *DB) QueryRow(sqlText string, params ...any) spi.Row {
	result := db.m.QueryRow(sqlText, params...)
	return result
}

func (db *DB) QueryRowContext(ctx context.Context, sqlText string, params ...any) spi.Row {
	// TODO apply context
	result := db.m.QueryRow(sqlText, params...)
	return result
}

func (db *DB) Appender(table string) (spi.Appender, error) {
	apd, err := db.m.Appender(table)
	if err != nil {
		return nil, err
	}
	return apd, nil
}

func (rows *ROWS) Next() bool {
	return rows.m.Next()
}
func (rows *ROWS) Scan(cols ...any) error {
	return rows.m.Scan(cols...)
}
func (rows *ROWS) Close() error {
	return rows.m.Close()
}
func (rows *ROWS) Message() string {
	// TODO implement
	return ""
}
func (rows *ROWS) IsFetchable() bool {
	return rows.m.IsFetchable()
}
func (rows *ROWS) Columns() (spi.Columns, error) {
	cols, err := rows.m.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]*spi.Column, len(cols))
	for i := range cols {
		result[i] = &spi.Column{
			Name:   cols[i].Name,
			Type:   cols[i].Type,
			Size:   cols[i].Size,
			Length: cols[i].Len,
		}
	}
	return result, nil
}
