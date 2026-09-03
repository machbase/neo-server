package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// TimerDefinition is a custom cron-style timer schedule persisted in
// _NEO_TIMER_DEF. USER_NAME controls definition management, EXEC_USER is the
// runtime identity, Disabled is the desired start state, and LastError holds
// runtime diagnostics. Like bridge, timer has no reserved definitions, so its
// Id is exposed as int64 directly instead of a string.
type TimerDefinition struct {
	Id          int64           `json:"id"`
	UserName    string          `json:"userName"`
	Name        string          `json:"name"`
	ExecUser    string          `json:"execUser"`
	AutoStart   bool            `json:"autoStart"`
	Disabled    bool            `json:"disabled"`
	LastError   string          `json:"lastError,omitempty"`
	LastErrorAt time.Time       `json:"lastErrorAt,omitempty"`
	Task        string          `json:"task"`
	Schedule    string          `json:"schedule"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`
}

func (s *Provider) ensureTimerTable(ctx context.Context, conn *sql.Conn) error {
	s.timerTableMu.Lock()
	defer s.timerTableMu.Unlock()
	if s.timerTableReady {
		return nil
	}
	if err := ensureScheduleTable(ctx, conn, "_NEO_TIMER_DEF", `CREATE TABLE IF NOT EXISTS _NEO_TIMER_DEF (
ID LONG PRIMARY KEY AUTO_INCREMENT,
USER_NAME VARCHAR(64) NOT NULL,
NAME VARCHAR(40) NOT NULL,
EXEC_USER VARCHAR(64) NOT NULL,
AUTO_START SHORT NOT NULL,
DISABLED SHORT NOT NULL,
LAST_ERROR VARCHAR(1024),
LAST_ERROR_AT DATETIME,
TASK VARCHAR(4096) NOT NULL,
SCHEDULE VARCHAR(128) NOT NULL,
ATTRIBUTES JSON
	)`); err != nil {
		return err
	}
	s.timerTableReady = true
	return nil
}

func scanTimerDefinition(scanner scheduleRowScanner) (*TimerDefinition, error) {
	var id int64
	var autoStart, disabled int64
	var userName, name, execUser, task, schedule sql.NullString
	var lastError, attrs sql.NullString
	var lastErrorAt sql.NullTime
	if err := scanner.Scan(&id, &userName, &name, &execUser, &autoStart, &disabled, &lastError, &lastErrorAt, &task, &schedule, &attrs); err != nil {
		return nil, err
	}
	def := &TimerDefinition{
		Id:          id,
		UserName:    userName.String,
		Name:        name.String,
		ExecUser:    execUser.String,
		AutoStart:   autoStart != 0,
		Disabled:    disabled != 0,
		LastError:   lastError.String,
		Task:        task.String,
		Schedule:    schedule.String,
		LastErrorAt: lastErrorAt.Time,
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
	rows, err := conn.QueryContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF ORDER BY ID`)
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
	row := conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF WHERE NAME = ?`, name)
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
	row := conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF WHERE ID = ?`, id)
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
		result, err := conn.ExecContext(ctx, `INSERT INTO _NEO_TIMER_DEF (USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, SCHEDULE, ATTRIBUTES) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			def.UserName, name, def.ExecUser, boolToShort(def.AutoStart), boolToShort(def.Disabled), nullIfEmpty(def.LastError), nullTime(def.LastErrorAt), def.Task, def.Schedule, attributesToDB(def.Attributes))
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

	result, err := conn.ExecContext(ctx, `UPDATE _NEO_TIMER_DEF SET NAME = ?, EXEC_USER = ?, AUTO_START = ?, DISABLED = ?, LAST_ERROR = ?, LAST_ERROR_AT = ?, TASK = ?, SCHEDULE = ?, ATTRIBUTES = ? WHERE ID = ? AND USER_NAME = ?`,
		name, def.ExecUser, boolToShort(def.AutoStart), boolToShort(def.Disabled), nullIfEmpty(def.LastError), nullTime(def.LastErrorAt), def.Task, def.Schedule, attributesToDB(def.Attributes), def.Id, def.UserName)
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

// SetTimerRuntimeError records scheduler diagnostics without applying a user
// ownership scope. An empty error clears the diagnostic.
func (s *Provider) SetTimerRuntimeError(ctx context.Context, id int64, lastError string) error {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var lastErrorAt any
	if lastError != "" {
		lastErrorAt = time.Now()
	}
	_, err = conn.ExecContext(ctx, `UPDATE _NEO_TIMER_DEF SET LAST_ERROR = ?, LAST_ERROR_AT = ? WHERE ID = ?`, nullIfEmpty(lastError), lastErrorAt, id)
	return err
}

func ensureScheduleTable(ctx context.Context, conn *sql.Conn, table, ddl string) error {
	if _, err := conn.ExecContext(ctx, ddl); err != nil {
		return err
	}
	rows, err := conn.QueryContext(ctx, "SELECT USER_NAME FROM "+table+" WHERE 1=0")
	if err == nil {
		return rows.Close()
	}
	if !isMissingUserNameColumn(err) {
		return err
	}
	if _, err := conn.ExecContext(ctx, "DROP TABLE "+table); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, ddl)
	return err
}

func isMissingUserNameColumn(err error) bool {
	message := strings.ToUpper(err.Error())
	return strings.Contains(message, "USER_NAME") &&
		(strings.Contains(message, "COLUMN") || strings.Contains(message, "UNKNOWN") || strings.Contains(message, "INVALID"))
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
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

// LoadTimers returns only definitions owned by scope.User. EXEC_USER is not
// an access grant: it controls runtime execution, while USER_NAME controls
// who may manage the definition.
func (s *Provider) LoadTimers(ctx context.Context, scope UserScope) ([]*TimerDefinition, error) {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF WHERE USER_NAME = ? ORDER BY ID`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	definitions := []*TimerDefinition{}
	for rows.Next() {
		definition, err := scanTimerDefinition(rows)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, rows.Err()
}

// LoadTimerForUser loads a timer only when its USER_NAME matches scope.User.
func (s *Provider) LoadTimerForUser(ctx context.Context, scope UserScope, id int64) (*TimerDefinition, error) {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	definition, err := scanTimerDefinition(conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF WHERE ID = ? AND USER_NAME = ?`, id, user))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("timer id '%d' not found", id)
	}
	return definition, err
}

// LoadTimerByNameForUser is retained only while schedule.* accepts names.
func (s *Provider) LoadTimerByNameForUser(ctx context.Context, scope UserScope, name string) (*TimerDefinition, error) {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	definition, err := scanTimerDefinition(conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, SCHEDULE, ATTRIBUTES FROM _NEO_TIMER_DEF WHERE NAME = ? AND USER_NAME = ?`, name, user))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("timer '%s' not found", name)
	}
	return definition, err
}

// SaveTimerForUser forces USER_NAME to scope.User before writing. Callers
// cannot claim another user's definition by supplying UserName in the payload.
func (s *Provider) SaveTimerForUser(ctx context.Context, scope UserScope, definition *TimerDefinition) error {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	definition.UserName = user
	return s.SaveTimer(ctx, definition)
}

// RemoveTimerForUser deletes a timer only when it is owned by scope.User.
func (s *Provider) RemoveTimerForUser(ctx context.Context, scope UserScope, id int64) error {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_TIMER_DEF WHERE ID = ? AND USER_NAME = ?`, id, user)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
