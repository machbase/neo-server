package spi

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-client/api"
)

type TagInfo struct {
	Database   string       `json:"database"`
	User       string       `json:"user"`
	Table      string       `json:"table"`
	Name       string       `json:"name"`
	Id         int64        `json:"id"`
	Err        error        `json:"-"`
	Summarized bool         `json:"summarized"`
	Stat       *TagStatInfo `json:"stat,omitempty"`
}

func (ti *TagInfo) Values() []interface{} {
	return []interface{}{
		ti.Database, ti.User, ti.Table, ti.Name, ti.Id, ti.Summarized,
	}
}

type TagStatInfo struct {
	Database      string    `json:"database"`
	User          string    `json:"user"`
	Table         string    `json:"table"`
	Name          string    `json:"name"`
	RowCount      int64     `json:"row_count"`
	MinTime       time.Time `json:"min_time"`
	MaxTime       time.Time `json:"max_time"`
	MinValue      float64   `json:"min_value"`
	MinValueTime  time.Time `json:"min_value_time"`
	MaxValue      float64   `json:"max_value"`
	MaxValueTime  time.Time `json:"max_value_time"`
	RecentRowTime time.Time `json:"recent_row_time"`
}

func (tsi *TagStatInfo) Values() []interface{} {
	return []interface{}{
		tsi.Database, tsi.User, tsi.Table, tsi.Name, tsi.RowCount,
		tsi.MinTime, tsi.MaxTime, tsi.MinValue, tsi.MinValueTime,
		tsi.MaxValue, tsi.MaxValueTime, tsi.RecentRowTime,
	}
}

func ListTagsSql(fullTable string, tagNameColumn string) string {
	database, user, table := api.TableName(fullTable).Split()
	return fmt.Sprintf(`SELECT _ID, %s FROM %s.%s._%s_META`, tagNameColumn, database, user, table)
}

func ListTags(ctx context.Context, conn api.Conn, fullTable string, tagNameColumn string) ([]*TagInfo, error) {
	var tags []*TagInfo
	database, user, table := api.TableName(fullTable).Split()
	rows, err := conn.Query(ctx, ListTagsSql(fullTable, tagNameColumn))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		nfo := &TagInfo{Database: database, User: user, Table: table}
		err := rows.Scan(&nfo.Id, &nfo.Name)
		if err != nil {
			return nil, err
		}
		tags = append(tags, nfo)
	}
	return tags, nil
}

func ListTagsWalk(ctx context.Context, conn api.Conn, table string, tagNameColumn string, callback func(*TagInfo) bool) {
	rows, err := conn.Query(ctx, ListTagsSql(table, tagNameColumn))
	if err != nil {
		callback(&TagInfo{Err: err})
		return
	}
	defer rows.Close()

	database, userName, tableName := api.TableName(table).Split()
	for rows.Next() {
		nfo := &TagInfo{Database: database, User: userName, Table: tableName}
		nfo.Err = rows.Scan(&nfo.Id, &nfo.Name)
		if !callback(nfo) {
			return
		}
	}
}

func TagStat(ctx context.Context, conn api.Conn, table string, tag string) (*TagStatInfo, error) {
	database, user, table := api.TableName(table).Split()
	sqlText := SqlTidy(`SELECT`,
		`NAME, ROW_COUNT,`,
		`MIN_TIME, MAX_TIME,`,
		`MIN_VALUE, MIN_VALUE_TIME, MAX_VALUE, MAX_VALUE_TIME,`,
		`RECENT_ROW_TIME`,
		`FROM`,
		fmt.Sprintf("%s.%s.V$%s_STAT", database, user, table),
		`WHERE NAME = ?`)
	nfo := &TagStatInfo{
		Database: database,
		User:     user,
		Table:    table,
	}
	row := conn.QueryRow(ctx, sqlText, tag)
	if err := row.Err(); err != nil {
		return nil, row.Err()
	}
	err := row.Scan(
		&nfo.Name, &nfo.RowCount,
		&nfo.MinTime, &nfo.MaxTime,
		&nfo.MinValue, &nfo.MinValueTime, &nfo.MaxValue, &nfo.MaxValueTime,
		&nfo.RecentRowTime)
	return nfo, err
}
