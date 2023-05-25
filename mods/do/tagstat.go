package do

import (
	"fmt"
	"strings"
	"time"

	spi "github.com/machbase/neo-spi"
)

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

func TagStat(db spi.Database, table string, tag string) (*TagStatInfo, error) {
	sqlText := fmt.Sprintf(`select
			name, row_count, min_time, max_time,
			min_value, min_value_time,
			max_value, max_value_time,
			recent_row_time
		from V$%s_STAT
		where name = ?`, strings.ToUpper(table))

	nfo := &TagStatInfo{}
	row := db.QueryRow(sqlText, tag)
	err := row.Scan(&nfo.Name, &nfo.RowCount, &nfo.MinTime, &nfo.MaxTime,
		&nfo.MinValue, &nfo.MinValueTime, &nfo.MaxValue, &nfo.MaxValueTime,
		&nfo.RecentRowTime)
	if err != nil {
		return nil, err
	}

	// if nfo.MinValueTime.IsZero() && nfo.MinValue == 0 {

	// }

	return nfo, nil
}
