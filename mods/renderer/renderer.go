package renderer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

type ChartQuery struct {
	Table        string
	Tag          string
	Field        string
	RangeFunc    func(db spi.Database) (time.Time, time.Time)
	Label        string
	TimeLocation *time.Location
	TimeRange    time.Duration
}

func (dq *ChartQuery) Query(db spi.Database) (*spi.RenderingData, error) {
	if strings.ToUpper(dq.Field) == "VALUE" {
		dq.Label = strings.ToLower(dq.Tag)
	} else {
		dq.Label = strings.ToLower(fmt.Sprintf("%s-%s", dq.Tag, dq.Field))
	}
	rangeFrom, rangeTo := dq.RangeFunc(db)

	lastSql := fmt.Sprintf(`select TIME, %s from %s where NAME = ? AND TIME between ? AND ? order by time`, dq.Field, dq.Table)

	rows, err := db.Query(lastSql, dq.Tag, rangeFrom, rangeTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := make([]float64, 0)
	labels := make([]string, 0)
	idx := 0
	var timeOffset time.Time
	var firstTime = false

	for rows.Next() {
		var ts time.Time
		var value float64
		err = rows.Scan(&ts, &value)
		if err != nil {
			fmt.Println(err.Error())
			return nil, err
		}
		if !firstTime {
			timeOffset = ts
			firstTime = true
		}

		label := ""
		if dq.TimeRange < 10*time.Second {
			d := ts.Sub(timeOffset)
			label = fmt.Sprintf("%d.%09d", d/time.Second, d%time.Second)
		} else if dq.TimeRange < time.Hour {
			label = ts.In(dq.TimeLocation).Format("04:05.000")
		} else {
			label = ts.In(dq.TimeLocation).Format("15:04:05")
		}
		values = append(values, value)
		labels = append(labels, label)
		idx++
	}
	return &spi.RenderingData{Name: dq.Label, Values: values, Labels: labels}, nil
}

func BuildChartQueries(tagPaths []string, cmdTimestamp string, cmdRange time.Duration, timeformat string, tz *time.Location) ([]*ChartQuery, error) {
	timeformat = util.GetTimeformat(timeformat)
	queries := make([]*ChartQuery, len(tagPaths))
	for i, path := range tagPaths {
		// path syntax: <table>/<tag>#<column>
		toks := strings.SplitN(path, "/", 2)
		if len(toks) == 2 {
			queries[i] = &ChartQuery{}
			queries[i].Table = strings.ToUpper(toks[0])
		} else {
			return nil, fmt.Errorf("table name not found in '%s'", path)
		}
		toks = strings.SplitN(toks[1], "#", 2)
		if len(toks) == 2 {
			queries[i].Tag = toks[0]
			queries[i].Field = toks[1]
		} else {
			queries[i].Tag = toks[0]
			queries[i].Field = "VALUE"
		}

		queries[i].TimeLocation = tz
		queries[i].TimeRange = cmdRange

		queries[i].RangeFunc = func(db spi.Database) (time.Time, time.Time) {
			var timestamp time.Time
			var epoch int64
			var err error
			if cmdTimestamp == "now" || cmdTimestamp == "" {
				timestamp = time.Now()
			} else if cmdTimestamp == "last" {
				row := db.QueryRow(fmt.Sprintf("select max_time from V$%s_STAT where name = ?", queries[i].Table), queries[i].Tag)
				if err := row.Scan(&timestamp); err != nil {
					timestamp = time.Now()
				}
			} else {
				switch timeformat {
				case "ns":
					epoch, err = strconv.ParseInt(cmdTimestamp, 10, 64)
					timestamp = time.Unix(0, epoch)
				case "us":
					epoch, err = strconv.ParseInt(cmdTimestamp, 10, 64)
					timestamp = time.Unix(0, epoch*int64(time.Microsecond))
				case "ms":
					epoch, err = strconv.ParseInt(cmdTimestamp, 10, 64)
					timestamp = time.Unix(epoch, epoch*int64(time.Millisecond))
				case "s":
					epoch, err = strconv.ParseInt(cmdTimestamp, 10, 64)
					timestamp = time.Unix(epoch, 0)
				default:
					timestamp, err = time.ParseInLocation(timeformat, cmdTimestamp, tz)
				}
				if err == nil {
					timestamp = timestamp.UTC()
				} else {
					fmt.Println("BuildChartQueries", err.Error())
					timestamp = time.Now()
				}
			}
			return timestamp.Add(-1 * cmdRange), timestamp
		}
	}
	return queries, nil
}
