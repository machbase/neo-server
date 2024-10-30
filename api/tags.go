package api

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func Tags(ctx context.Context, conn Conn, table string, callback func(string, int64, error) bool) {
	var sqlText string
	if strings.Contains(table, ".") {
		idx := strings.LastIndex(table, ".")
		prefix := table[0:idx]
		table = table[idx+1:]
		sqlText = fmt.Sprintf("select _ID, NAME from %s._%s_META", strings.ToUpper(prefix), strings.ToUpper(table))
	} else {
		sqlText = fmt.Sprintf("select _ID, NAME from _%s_META", strings.ToUpper(table))
	}
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		callback("", 0, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64 = -1
		var name string
		err := rows.Scan(&id, &name)
		if err != nil {
			callback("", id, err)
			return
		}
		if !callback(name, id, nil) {
			return
		}
	}
}

type TagStatInfo struct {
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

func TagStat(ctx context.Context, conn Conn, table string, tag string) (*TagStatInfo, error) {
	sqlText := fmt.Sprintf(`select
			name, row_count, min_time, max_time,
			min_value, min_value_time,
			max_value, max_value_time,
			recent_row_time
		from V$%s_STAT
		where name = ?`, strings.ToUpper(table))
	sqlText = SqlTidy(sqlText)
	nfo := &TagStatInfo{}
	row := conn.QueryRow(ctx, sqlText, tag)
	if err := row.Err(); err != nil {
		return nil, row.Err()
	}
	err := row.Scan(&nfo.Name, &nfo.RowCount, &nfo.MinTime, &nfo.MaxTime,
		&nfo.MinValue, &nfo.MinValueTime, &nfo.MaxValue, &nfo.MaxValueTime,
		&nfo.RecentRowTime)
	if err != nil {
		return nil, err
	}

	return nfo, nil
}
