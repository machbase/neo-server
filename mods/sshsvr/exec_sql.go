package shell

import (
	"fmt"
	"strings"
	"time"
)

func (sess *Session) exec_sql(line string) {
	sess.log.Debugf("SQL: %s", line)

	rows, err := sess.db.Query(line)
	if err != nil {
		sess.WriteStr(err.Error() + "\r\n")
		return
	}
	defer rows.Close()

	if !rows.IsFetchable() {
		nrows, err := rows.AffectedRows()
		if err != nil {
			sess.log.Errorf("fail to get affected rows %s", err.Error())
			return
		}
		sess.Println(rows.ResultString(nrows))
		return
	}
	chunk := &ResultChunk{}
	chunk.heading = true

	chunk.cols, err = rows.ColumnNames()
	if err != nil {
		sess.WriteStr(err.Error() + "\r\n")
		return
	}
	nrows := 0
	height := sess.window.Height - 1
	if chunk.heading {
		height--
	}
	for {
		rec, next, err := rows.Fetch()
		if err != nil {
			sess.Println(err.Error() + "\r\n")
			return
		}
		if !next {
			if len(chunk.rows) > 0 {
				sess.display(chunk)
			}
			sess.Println(rows.ResultString(int64(nrows)))
			return
		}
		nrows++
		cols := make([]string, len(rec))
		for i, r := range rec {
			if r == nil {
				cols[i] = "NULL"
				continue
			}
			switch v := r.(type) {
			case *string:
				cols[i] = *v
			case *time.Time:
				cols[i] = v.Format("2006-01-02 15:04:05.000000")
			case *float64:
				cols[i] = fmt.Sprintf("%f", *v)
			case *int:
				cols[i] = fmt.Sprintf("%d", *v)
			case *int32:
				cols[i] = fmt.Sprintf("%d", *v)
			case *int64:
				cols[i] = fmt.Sprintf("%d", *v)
			default:
				cols[i] = fmt.Sprintf("%T", r)
			}
		}
		chunk.rows = append(chunk.rows, cols)

		if nrows%height == 0 {
			chunk = sess.display(chunk)
			sess.WriteStr(":")
			sess.Flush()
			c := [3]byte{}
			n, err := sess.ss.Read(c[:])
			// ':' prompt를 삭제한다.
			sess.EraseLine()
			sess.CursorBackward(1)

			if n > 0 && err == nil {
				switch c[0] {
				case 'q', 'Q':
					return
				default:
				}
			}
		}
	}
}

type ResultChunk struct {
	heading bool
	width   []int
	cols    []string
	rows    [][]string
}

func (sess *Session) display(chunk *ResultChunk) *ResultChunk {
	if len(chunk.width) == 0 {
		chunk.width = make([]int, len(chunk.cols))
		// 각 컬럼의 폭을 계산한다.
		for c := range chunk.cols {
			// 컬럼 명의 길이를 최소 폭으로 한다.
			max := len(chunk.cols[c])
			// 각 rows를 순회하며 해당 column 값의 폭 중에서 가장 긴 값을 찾는다.
			for r := range chunk.rows {
				v := chunk.rows[r][c]
				if len(v) > max {
					max = len(v)
				}
			}
			chunk.width[c] = max
		}
		for c := range chunk.cols {
			chunk.cols[c] = fmt.Sprintf("%-*s", chunk.width[c], chunk.cols[c])
		}
	}

	if chunk.heading {
		line := strings.Join(chunk.cols, " | ")
		if len(line) > sess.window.Width {
			line = line[0 : sess.window.Width-4]
			line = line + "..."
		}
		sess.WriteStr(line + "\r\n")
	}
	for r, row := range chunk.rows {
		for c := range chunk.cols {
			chunk.rows[r][c] = fmt.Sprintf("%-*s", chunk.width[c], row[c])
		}
		line := strings.Join(row, "   ")
		if len(line) > sess.window.Width {
			line = line[0 : sess.window.Width-4]
			line = line + "..."
		}
		sess.WriteStr(line + "\r\n")
	}

	return &ResultChunk{
		heading: chunk.heading,
		width:   chunk.width,
		cols:    chunk.cols,
	}
}
