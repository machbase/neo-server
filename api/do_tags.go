package api

import (
	"context"
	"fmt"

	"github.com/machbase/neo-client/api"
)

func ListTagsSql(fullTable string, tagNameColumn string) string {
	database, user, table := TableName(fullTable).Split()
	return fmt.Sprintf(`SELECT _ID, %s FROM %s.%s._%s_META`, tagNameColumn, database, user, table)
}

func ListTags(ctx context.Context, conn api.Conn, fullTable string, tagNameColumn string) ([]*TagInfo, error) {
	var tags []*TagInfo
	database, user, table := TableName(fullTable).Split()
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

	database, userName, tableName := TableName(table).Split()
	for rows.Next() {
		nfo := &TagInfo{Database: database, User: userName, Table: tableName}
		nfo.Err = rows.Scan(&nfo.Id, &nfo.Name)
		if !callback(nfo) {
			return
		}
	}
}

func TagStatSql(fullTable string) string {
	database, user, table := TableName(fullTable).Split()
	return api.SqlTidy(`SELECT`,
		`NAME, ROW_COUNT,`,
		`MIN_TIME, MAX_TIME,`,
		`MIN_VALUE, MIN_VALUE_TIME, MAX_VALUE, MAX_VALUE_TIME,`,
		`RECENT_ROW_TIME`,
		`FROM`,
		fmt.Sprintf("%s.%s.V$%s_STAT", database, user, table),
		`WHERE NAME = ?`)
}

func TagStat(ctx context.Context, conn api.Conn, table string, tag string) (*TagStatInfo, error) {
	database, user, table := TableName(table).Split()
	sqlText := TagStatSql(table)
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
