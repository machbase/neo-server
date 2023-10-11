package do

import (
	"context"
	"fmt"
	"strings"

	spi "github.com/machbase/neo-spi"
)

// Describe retrieves the result of 'desc table'.
//
// If includeHiddenColumns is true, the result includes hidden columns those name start with '_'
// such as "_RID" and "_ARRIVAL_TIME".
func Describe(ctx context.Context, conn spi.Conn, name string, includeHiddenColumns bool) (Description, error) {
	tableName := strings.ToUpper(name)
	if strings.HasPrefix(tableName, "V$") {
		return describe_mv(ctx, conn, name, includeHiddenColumns)
	} else if strings.HasPrefix(tableName, "M$") {
		return describe_mv(ctx, conn, name, includeHiddenColumns)
	} else {
		return describe(ctx, conn, name, includeHiddenColumns)
	}
}

func describe(ctx context.Context, conn spi.Conn, name string, includeHiddenColumns bool) (Description, error) {
	d := &TableDescription{}
	var tableType int
	var colCount int
	var colType int

	tableName := strings.ToUpper(name)
	userName := "SYS"
	dbName := "MACHBASEDB"
	dbId := -1
	toks := strings.Split(tableName, ".")
	if len(toks) == 2 {
		userName = toks[0]
		tableName = toks[1]
	} else if len(toks) == 3 {
		dbName = toks[0]
		userName = toks[1]
		tableName = toks[2]
	}

	if dbName != "" && dbName != "MACHBASEDB" {
		row := conn.QueryRow(ctx, "select BACKUP_TBSID from V$STORAGE_MOUNT_DATABASES where MOUNTDB = ?", dbName)
		if err := row.Scan(&dbId); err != nil {
			return nil, err
		}
	}
	sqlText := `SELECT
			j.ID as TABLE_ID,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG,
			j.COLCOUNT as TABLE_COLCOUNT
		from
			M$SYS_USERS u,
			M$SYS_TABLES j
		where
			u.NAME = ?
		and j.USER_ID = u.USER_ID
		and j.DATABASE_ID = ?
		and j.NAME = ?`

	r := conn.QueryRow(ctx, sqlText, userName, dbId, tableName)
	if r.Err() != nil {
		return nil, r.Err()
	}
	if err := r.Scan(&d.Id, &tableType, &d.Flag, &colCount); err != nil {
		return nil, err
	}
	d.Type = spi.TableType(tableType)
	d.Database = dbName
	d.User = userName
	d.Name = tableName

	rows, err := conn.Query(ctx, "select name, type, length, id from M$SYS_COLUMNS where table_id = ? order by id", d.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		col := &ColumnDescription{}
		err = rows.Scan(&col.Name, &colType, &col.Length, &col.Id)
		if err != nil {
			return nil, err
		}
		if !includeHiddenColumns && strings.HasPrefix(col.Name, "_") {
			continue
		}
		col.Type = spi.ColumnType(colType)
		d.Columns = append(d.Columns, col)
	}
	return d, nil
}

func describe_mv(ctx context.Context, conn spi.Conn, name string, includeHiddenColumns bool) (Description, error) {
	d := &TableDescription{}
	var tableType int
	var colCount int
	var colType int
	tableName := strings.ToUpper(name)
	tablesTable := "M$SYS_TABLES"
	columnsTable := "M$SYS_COLUMNS"
	if strings.HasPrefix(tableName, "V$") {
		tablesTable = "V$TABLES"
		columnsTable = "V$COLUMNS"
	} else if strings.HasPrefix(tableName, "M$") {
		tablesTable = "M$TABLES"
		columnsTable = "M$COLUMNS"
	}
	r := conn.QueryRow(ctx, fmt.Sprintf("select name, type, flag, id, colcount from %s where name = ?", tablesTable), tableName)
	if err := r.Scan(&d.Name, &tableType, &d.Flag, &d.Id, &colCount); err != nil {
		return nil, err
	}
	d.Type = spi.TableType(tableType)
	d.Database = "MACHBASEDB"
	d.User = "SYS"
	d.Name = tableName

	rows, err := conn.Query(ctx, fmt.Sprintf(`select name, type, length, id from %s where table_id = ? order by id`, columnsTable), d.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		col := &ColumnDescription{}
		err = rows.Scan(&col.Name, &colType, &col.Length, &col.Id)
		if err != nil {
			return nil, err
		}
		if !includeHiddenColumns && strings.HasPrefix(col.Name, "_") {
			continue
		}
		col.Type = spi.ColumnType(colType)
		d.Columns = append(d.Columns, col)
	}
	return d, nil
}

type Description interface {
	description()
}

func (td *TableDescription) description()  {}
func (cd *ColumnDescription) description() {}

// TableDescription is represents data that comes as a result of 'desc <table>'
type TableDescription struct {
	Database string             `json:"database"`
	User     string             `json:"user"`
	Name     string             `json:"name"`
	Type     spi.TableType      `json:"type"`
	Flag     int                `json:"flag"`
	Id       int                `json:"id"`
	Columns  ColumnDescriptions `json:"columns"`
}

// TypeString returns string representation of table type.
func (td *TableDescription) TypeString() string {
	return TableTypeDescription(td.Type, td.Flag)
}

// TableTypeDescription converts the given TableType and flag into string representation.
func TableTypeDescription(typ spi.TableType, flag int) string {
	desc := "undef"
	switch typ {
	case spi.LogTableType:
		desc = "Log Table"
	case spi.FixedTableType:
		desc = "Fixed Table"
	case spi.VolatileTableType:
		desc = "Volatile Table"
	case spi.LookupTableType:
		desc = "Lookup Table"
	case spi.KeyValueTableType:
		desc = "KeyValue Table"
	case spi.TagTableType:
		desc = "Tag Table"
	}
	switch flag {
	case 1:
		desc += " (data)"
	case 2:
		desc += " (rollup)"
	case 4:
		desc += " (meta)"
	case 8:
		desc += " (stat)"
	}
	return desc
}

type ColumnDescriptions []*ColumnDescription

func (cds ColumnDescriptions) Columns() spi.Columns {
	cols := make([]*spi.Column, len(cds))
	for i, cd := range cds {
		col := &spi.Column{
			Name:   cd.Name,
			Type:   spi.ColumnBufferType(cd.Type),
			Size:   cd.Length,
			Length: cd.Length,
		}
		cols[i] = col
	}
	return cols
}

// columnDescription represents information of a column info.
type ColumnDescription struct {
	Id     uint64         `json:"id"`
	Name   string         `json:"name"`
	Type   spi.ColumnType `json:"type"`
	Length int            `json:"length"`
}

// TypeString returns string representation of column type.
func (cd *ColumnDescription) TypeString() string {
	return spi.ColumnTypeString(cd.Type)
}
