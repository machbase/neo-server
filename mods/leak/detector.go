package leak

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/leak/lru"
	"github.com/machbase/neo-server/mods/logging"
	cmap "github.com/orcaman/concurrent-map"
)

type Detector struct {
	log         logging.Log
	stopCh      chan bool
	stopWg      sync.WaitGroup
	runCh       chan time.Time
	runLock     sync.Mutex
	running     bool
	runTick     time.Duration
	inflights   cmap.ConcurrentMap
	history     *lru.Cache
	historyLock sync.Mutex
}

type Option func(*Detector)

func Timer(dur time.Duration) Option {
	return func(det *Detector) {
		det.runTick = dur
	}
}

func HistoryCapacity(count int) Option {
	return func(det *Detector) {
		det.history = lru.New(count)
	}
}

func NewDetector(opts ...Option) *Detector {
	ret := &Detector{
		log:       logging.GetLog("leakdetector"),
		stopCh:    make(chan bool),
		runCh:     make(chan time.Time),
		runTick:   10 * time.Second,
		inflights: cmap.New(),
	}
	for _, op := range opts {
		op(ret)
	}
	if ret.history == nil {
		ret.history = lru.New(10)
	}
	return ret
}

func (det *Detector) Start() {
	det.stopWg.Add(1)
	go func() {
		ticker := time.NewTicker(det.runTick)
	loop:
		for {
			select {
			case <-det.stopCh:
				break loop
			case <-det.runCh:
				det.runDetection()
			case <-ticker.C:
				det.runDetection()
			}
		}
		ticker.Stop()
		det.stopWg.Done()
	}()
}

func (det *Detector) Stop() {
	det.stopCh <- true
	det.stopWg.Wait()
	close(det.stopCh)
	close(det.runCh)
}

func (det *Detector) Detect() {
	det.runCh <- time.Now()
}

func (det *Detector) runDetection() {
	if det.running {
		return
	}
	det.runLock.Lock()
	det.running = true
	defer func() {
		det.running = false
		det.runLock.Unlock()
	}()

	det.inflights.IterCb(func(key string, value interface{}) {
		switch value.(type) {
		case *RowsParole:
			//fmt.Println(key, val.String())
		case *AppenderParole:
			//fmt.Println(key, val.String())
		}
	})
}

var idSerial int64

type RowsParole struct {
	Rows        api.Rows
	id          string
	release     func()
	releaseOnce sync.Once
	sqlText     string

	createTime     time.Time
	lastAccessTime time.Time
	releaseTime    time.Time
}

func (rp *RowsParole) String() string {
	return fmt.Sprintf("ROWS %s %s %s", rp.id, time.Since(rp.createTime).String(), rp.sqlText)
}

func (rp *RowsParole) Id() string {
	return rp.id
}

func (rp *RowsParole) Release() {
	if rp.release != nil {
		rp.releaseOnce.Do(rp.release)
	}
}

func (det *Detector) EnlistDetective(obj any, sqlTextOrTableName string) {
	key := fmt.Sprintf("%p#0", obj)
	if rows, ok := obj.(api.Rows); ok {
		det.detainRows(key, rows, sqlTextOrTableName)
	} else if appender, ok := obj.(api.Appender); ok {
		det.detainAppender(key, appender, sqlTextOrTableName)
	}
}

func (det *Detector) DelistDetective(obj any) {
	key := fmt.Sprintf("%p#0", obj)
	det.inflights.RemoveCb(key, func(key string, v any, exists bool) bool {
		if exists {
			switch val := v.(type) {
			case *RowsParole:
				val.releaseTime = time.Now()
				det.addHistoryRows(val)
			case *AppenderParole:
				val.releaseTime = time.Now()
				det.addHistoryAppender(val)
			}
		}
		return true
	})
}

func (det *Detector) UpdateDetective(obj any) {
	key := fmt.Sprintf("%p", obj)
	if value, ok := det.inflights.Get(key); ok {
		switch val := value.(type) {
		case *RowsParole:
			val.lastAccessTime = time.Now()
		}
	}
}

func (det *Detector) DetainRows(rows api.Rows, sqlText string) *RowsParole {
	ser := atomic.AddInt64(&idSerial, 1)
	key := fmt.Sprintf("%p#%d", rows, ser)
	return det.detainRows(key, rows, sqlText)
}

func (det *Detector) detainRows(key string, rows api.Rows, sqlText string) *RowsParole {
	ret := &RowsParole{
		Rows:       rows,
		id:         key,
		sqlText:    sqlText,
		createTime: time.Now(),
	}
	ret.lastAccessTime = ret.createTime
	ret.release = func() {
		det.inflights.RemoveCb(ret.id, func(key string, v any, exists bool) bool {
			if ret.Rows != nil {
				err := ret.Rows.Close()
				if err != nil {
					det.log.Warnf("error on rows.close; %s, statement: %s", err.Error(), ret.String())
				}
				ret.releaseTime = time.Now()
				det.addHistoryRows(ret)
			}
			return true
		})
	}
	det.inflights.Set(key, ret)
	return ret
}

func (det *Detector) Rows(id string) (*RowsParole, error) {
	value, exists := det.inflights.Get(id)
	if !exists {
		return nil, fmt.Errorf("handle '%s' not found", id)
	}
	ret, ok := value.(*RowsParole)
	if !ok {
		return nil, fmt.Errorf("handle '%s' is not valid", id)
	}
	ret.lastAccessTime = time.Now()
	return ret, nil
}

type AppenderParole struct {
	Appender    api.Appender
	id          string
	release     func()
	releaseOnce sync.Once
	tableName   string
	createTime  time.Time
	releaseTime time.Time
}

func (ap *AppenderParole) String() string {
	return fmt.Sprintf("APPEND %s %s %s", ap.id, time.Since(ap.createTime), ap.tableName)
}

func (ap *AppenderParole) Id() string {
	return ap.id
}

func (ap *AppenderParole) Release() {
	if ap.release != nil {
		ap.releaseOnce.Do(ap.release)
	}
}

func (det *Detector) DetainAppender(appender api.Appender, tableName string) *AppenderParole {
	ser := atomic.AddInt64(&idSerial, 1)
	key := fmt.Sprintf("%p#%d", appender, ser)
	return det.detainAppender(key, appender, tableName)
}

func (det *Detector) detainAppender(key string, appender api.Appender, tableName string) *AppenderParole {
	ret := &AppenderParole{
		Appender:   appender,
		id:         key,
		tableName:  tableName,
		createTime: time.Now(),
	}
	ret.release = func() {
		det.inflights.RemoveCb(ret.id, func(key string, v any, exists bool) bool {
			if ret.Appender != nil {
				succ, fail, err := ret.Appender.Close()
				if err != nil {
					det.log.Warnf("close APND %v success:%d fail:%d error:%s", ret.id, succ, fail, err.Error())
				} else {
					if fail == 0 {
						det.log.Tracef("close APND %v success:%d", ret.id, succ)
					} else {
						det.log.Tracef("close APND %v success:%d fail:%d", ret.id, succ, fail)
					}
				}
				ret.releaseTime = time.Now()
				det.addHistoryAppender(ret)
			}
			return true
		})
	}
	det.inflights.Set(key, ret)
	return ret
}

func (det *Detector) Appender(id string) (*AppenderParole, error) {
	value, exists := det.inflights.Get(id)
	if !exists {
		return nil, fmt.Errorf("handle '%s' not found", id)
	}
	ret, ok := value.(*AppenderParole)
	if !ok {
		return nil, fmt.Errorf("handle '%s' is not valid", id)
	}
	return ret, nil
}

type RowsStat struct {
	lock     sync.Mutex
	sqlText  string
	ageTotal time.Duration
	count    int64
}

func (rs *RowsStat) String() string {
	ageAverage := time.Duration(int64(rs.ageTotal) / rs.count)
	return fmt.Sprintf("count:%d total:%s avg:%s %s", rs.count, rs.ageTotal, ageAverage, rs.sqlText)
}

func (det *Detector) addHistoryRows(rp *RowsParole) {
	if rp == nil || rp.sqlText == "" {
		return
	}
	text := rp.sqlText
	age := rp.releaseTime.Sub(rp.createTime)

	det.historyLock.Lock()
	obj, ok := det.history.Get(lru.Key(text))
	if !ok {
		obj = &RowsStat{sqlText: text}
		det.history.Add(lru.Key(text), obj)
	}
	det.historyLock.Unlock()

	if rowsStat, ok := obj.(*RowsStat); ok {
		rowsStat.lock.Lock()
		rowsStat.count += 1
		rowsStat.ageTotal += age
		rowsStat.lock.Unlock()
	}
}

func (det *Detector) addHistoryAppender(ap *AppenderParole) {
}
