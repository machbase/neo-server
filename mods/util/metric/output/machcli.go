package output

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
	"github.com/machbase/neo-server/v8/mods/util/metric"
)

type MachCli struct {
	TableName string   // target table name; default: TAG
	Columns   []string // target columns (name, time, value); default: name, time, value
	Prefix    string   // prefix for metric name
	Host      string
	Port      int
	User      string
	Pass      string

	db *machcli.Database
}

var _ metric.Output = (*MachCli)(nil)

func (m *MachCli) openConn(ctx context.Context) (api.Conn, error) {
	if m.db == nil {
		if db, err := machcli.NewDatabase(&machcli.Config{
			Host:         m.Host,
			Port:         m.Port,
			TrustUsers:   map[string]string{m.User: m.Pass},
			MaxOpenConn:  -1,
			MaxOpenQuery: -1,
		}); err != nil {
			return nil, err
		} else {
			m.db = db
		}
	}
	//db.Connect(ctx context.Context, options ...api.ConnectOption)
	conn, err := m.db.Connect(ctx, api.WithTrustUser(m.User))
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (m *MachCli) Process(prd metric.Product) error {
	ctx := context.TODO()
	recs, err := m.convertProduct(prd)
	if err != nil {
		return err
	}

	tableName := m.TableName
	if tableName == "" {
		tableName = "TAG"
	}
	columns := m.Columns
	if len(columns) != 3 {
		columns = []string{"name", "time", "value"}
	}

	conn, err := m.openConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	for _, r := range recs {
		result := conn.Exec(
			ctx,
			"INSERT INTO %s (%s) VALUES (?, ?, ?)",
			tableName,
			strings.Join(columns, ", "),
			r.Name, r.Time.UnixNano(), r.Val)
		if err := result.Err(); err != nil {
			return fmt.Errorf("metrics insert: %w", err)
		}
	}
	return nil
}

type StatRec struct {
	Name string
	Time time.Time
	Val  float64
}

func (m *MachCli) convertProduct(pd metric.Product) (result []StatRec, err error) {
	prefix := ""
	if m.Prefix != "" {
		prefix = m.Prefix + ":"
	}

	switch p := pd.Value.(type) {
	case *metric.CounterValue:
		if p.Samples == 0 {
			return // Skip zero counters
		}
		result = []StatRec{{fmt.Sprintf("%s%s", prefix, pd.Name), pd.Time, p.Value}}
	case *metric.GaugeValue:
		if p.Samples == 0 {
			return // Skip zero gauges
		}
		result = []StatRec{{fmt.Sprintf("%s%s", prefix, pd.Name), pd.Time, p.Value}}
	case *metric.MeterValue:
		if p.Samples == 0 {
			return // Skip zero meters
		}
		result = []StatRec{
			{fmt.Sprintf("%s%s:avg", prefix, pd.Name), pd.Time, p.Sum / float64(p.Samples)},
			{fmt.Sprintf("%s%s:max", prefix, pd.Name), pd.Time, p.Max},
			{fmt.Sprintf("%s%s:min", prefix, pd.Name), pd.Time, p.Min},
		}
	case *metric.TimerValue:
		if p.Samples == 0 {
			return // Skip zero timers
		}
		result = []StatRec{
			{fmt.Sprintf("%s%s:avg", prefix, pd.Name), pd.Time, float64(int64(p.Sum) / p.Samples)},
			{fmt.Sprintf("%s%s:max", prefix, pd.Name), pd.Time, float64(p.Max)},
			{fmt.Sprintf("%s%s:min", prefix, pd.Name), pd.Time, float64(p.Min)},
		}
	case *metric.HistogramValue:
		if p.Samples == 0 {
			return // Skip zero histograms
		}
		for i, x := range p.P {
			pct := fmt.Sprintf("%d", int(x*1000))
			if pct[len(pct)-1] == '0' {
				pct = pct[:len(pct)-1]
			}
			result = append(result, StatRec{
				Name: fmt.Sprintf("%s%s:p%s", prefix, pd.Name, pct),
				Time: pd.Time,
				Val:  p.Values[i],
			})
		}
	case *metric.OdometerValue:
		if p.Samples == 0 {
			return // Skip zero odometers
		}
		result = []StatRec{{fmt.Sprintf("%s%s", prefix, pd.Name), pd.Time, p.Diff()}}
	default:
		err = fmt.Errorf("metrics unknown type: %T", p)
	}
	return
}
