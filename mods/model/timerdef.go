package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// TimerDefinition is a custom cron-style timer schedule persisted in
// _NEO_TIMER_DEF. Like bridge, timer has no reserved definitions, so its
// Id is exposed as int64 directly instead of a string.
type TimerDefinition struct {
	Id         int64           `json:"id"`
	Name       string          `json:"name"`
	ExecUser   string          `json:"execUser"`
	AutoStart  bool            `json:"autoStart"`
	Task       string          `json:"task"`
	Schedule   string          `json:"schedule"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

func (s *Provider) ensureTimerTable(ctx context.Context, conn *sql.Conn) error {
	s.timerTableMu.Lock()
	defer s.timerTableMu.Unlock()
	if s.timerTableReady {
		return nil
	}
	_, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _NEO_TIMER_DEF (
ID LONG PRIMARY KEY AUTO_INCREMENT,
NAME VARCHAR(40) NOT NULL,
EXEC_USER VARCHAR(64) NOT NULL,
AUTO_START SHORT NOT NULL,
TASK VARCHAR(4096) NOT NULL,
SCHEDULE VARCHAR(128) NOT NULL,
ATTRIBUTES JSON
)`)
	if err != nil {
		return err
	}
	s.timerTableReady = true
	return nil
}

func scanTimerDefinition(scanner scheduleRowScanner) (*TimerDefinition, error) {
	var id int64
	var autoStart int64
	var name, execUser, task, schedule sql.NullString
	var attrs sql.NullString
	if err := scanner.Scan(&id, &name, &execUser, &autoStart, &task, &schedule, &attrs); err != nil {
		return nil, err
	}
	def := &TimerDefinition{
		Id:        id,
		Name:      name.String,
		ExecUser:  execUser.String,
		AutoStart: autoStart != 0,
		Task:      task.String,
		Schedule:  schedule.String,
	}
	if attrs.Valid && strings.TrimSpace(attrs.String) != "" {
		def.Attributes = json.RawMessage(attrs.String)
	}
	return def, nil
}

// LoadAllTimers returns every timer definition.
func (s *Provider) LoadAllTimers(ctx context.Context) ([]*TimerDefinition, error) {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, NAME, EXEC_USER, AUTO_START, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF ORDER BY ID`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ret := []*TimerDefinition{}
	for rows.Next() {
		def, err := scanTimerDefinition(rows)
		if err != nil {
			return nil, err
		}
		ret = append(ret, def)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

// LoadTimer loads a single timer definition by name.
//
// Deprecated: kept for backward compatibility with the file-based storage
// era. Prefer LoadTimerByID for new call sites.
func (s *Provider) LoadTimer(ctx context.Context, name string) (*TimerDefinition, error) {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	name = strings.ToUpper(strings.TrimSpace(name))
	row := conn.QueryRowContext(ctx, `SELECT ID, NAME, EXEC_USER, AUTO_START, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF WHERE NAME = ?`, name)
	def, err := scanTimerDefinition(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("timer '%s' not found", name)
	}
	if err != nil {
		return nil, err
	}
	return def, nil
}

// LoadTimerByID loads a single timer definition by its auto increment ID.
func (s *Provider) LoadTimerByID(ctx context.Context, id int64) (*TimerDefinition, error) {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	row := conn.QueryRowContext(ctx, `SELECT ID, NAME, EXEC_USER, AUTO_START, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF WHERE ID = ?`, id)
	def, err := scanTimerDefinition(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("timer id '%d' not found", id)
	}
	if err != nil {
		return nil, err
	}
	return def, nil
}

// SaveTimer inserts or updates a timer definition. When def.Id == 0 a new
// row is inserted with ExecUser recorded as the creator; otherwise the
// existing row is updated and ExecUser is left unchanged (creator is
// immutable).
func (s *Provider) SaveTimer(ctx context.Context, def *TimerDefinition) error {
	if def == nil {
		return errors.New("timer definition not specified")
	}
	if len(def.Name) == 0 {
		return errors.New("timer name not specified")
	}
	if len(def.ExecUser) == 0 {
		return errors.New("timer exec user not specified")
	}
	if len(def.Schedule) == 0 {
		return errors.New("timer schedule not specified")
	}
	if len(def.Task) == 0 {
		return errors.New("timer task not specified")
	}
	name := strings.ToUpper(strings.TrimSpace(def.Name))
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if def.Id == 0 {
		exists, err := s.scheduleNameExists(ctx, conn, name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("schedule name '%s' already exists", name)
		}
		result, err := conn.ExecContext(ctx, `INSERT INTO _NEO_TIMER_DEF (NAME, EXEC_USER, AUTO_START, TASK, SCHEDULE, ATTRIBUTES) VALUES (?, ?, ?, ?, ?, ?)`,
			name, def.ExecUser, boolToShort(def.AutoStart), def.Task, def.Schedule, attributesToDB(def.Attributes))
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		def.Id = id
		def.Name = name
		return nil
	}

	result, err := conn.ExecContext(ctx, `UPDATE _NEO_TIMER_DEF SET NAME = ?, AUTO_START = ?, TASK = ?, SCHEDULE = ?, ATTRIBUTES = ? WHERE ID = ?`,
		name, boolToShort(def.AutoStart), def.Task, def.Schedule, attributesToDB(def.Attributes), def.Id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return os.ErrNotExist
	}
	def.Name = name
	return nil
}

// RemoveTimer deletes a timer definition by name.
//
// Deprecated: kept for backward compatibility with the file-based storage
// era. Prefer RemoveTimerByID for new call sites.
func (s *Provider) RemoveTimer(ctx context.Context, name string) error {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	name = strings.ToUpper(strings.TrimSpace(name))
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_TIMER_DEF WHERE NAME = ?`, name)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return os.ErrNotExist
	}
	return nil
}

// RemoveTimerByID deletes a timer definition by its auto increment ID.
func (s *Provider) RemoveTimerByID(ctx context.Context, id int64) error {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_TIMER_DEF WHERE ID = ?`, id)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return os.ErrNotExist
	}
	return nil
}
