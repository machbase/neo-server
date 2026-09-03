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

// SubscriberDefinition is a custom mqtt/nats subscriber schedule persisted
// in _NEO_SUBSCRIBER_DEF. Like bridge, subscriber has no reserved
// definitions. USER_NAME controls definition management, EXEC_USER is the
// runtime identity, Disabled is the desired start state, and LastError holds
// runtime diagnostics. Its Id is exposed as int64 directly instead of a string.
type SubscriberDefinition struct {
	Id          int64     `json:"id"`
	UserName    string    `json:"userName"`
	Name        string    `json:"name"`
	ExecUser    string    `json:"execUser"`
	AutoStart   bool      `json:"autoStart"`
	Disabled    bool      `json:"disabled"`
	LastError   string    `json:"lastError,omitempty"`
	LastErrorAt time.Time `json:"lastErrorAt,omitempty"`
	Task        string    `json:"task"`
	Bridge      string    `json:"bridge"`
	Topic       string    `json:"topic"`

	QoS        int    `json:"qos,omitempty"`    // mqtt only
	QueueName  string `json:"queue,omitempty"`  // nats only
	StreamName string `json:"stream,omitempty"` // nats only

	Attributes json.RawMessage `json:"attributes,omitempty"`
}

func (s *Provider) ensureSubscriberTable(ctx context.Context, conn *sql.Conn) error {
	s.subscriberTableMu.Lock()
	defer s.subscriberTableMu.Unlock()
	if s.subscriberTableReady {
		return nil
	}
	if err := ensureScheduleTable(ctx, conn, "_NEO_SUBSCRIBER_DEF", `CREATE TABLE IF NOT EXISTS _NEO_SUBSCRIBER_DEF (
ID LONG PRIMARY KEY AUTO_INCREMENT,
USER_NAME VARCHAR(64) NOT NULL,
NAME VARCHAR(40) NOT NULL,
EXEC_USER VARCHAR(64) NOT NULL,
AUTO_START SHORT NOT NULL,
DISABLED SHORT NOT NULL,
LAST_ERROR VARCHAR(1024),
LAST_ERROR_AT DATETIME,
TASK VARCHAR(4096) NOT NULL,
BRIDGE VARCHAR(128) NOT NULL,
TOPIC VARCHAR(512) NOT NULL,
QOS INTEGER,
QUEUE_NAME VARCHAR(128),
STREAM_NAME VARCHAR(128),
ATTRIBUTES JSON
)`); err != nil {
		return err
	}
	s.subscriberTableReady = true
	return nil
}

func scanSubscriberDefinition(scanner scheduleRowScanner) (*SubscriberDefinition, error) {
	var id int64
	var autoStart, disabled int64
	var qos sql.NullInt64
	var userName, name, execUser, task, brdg, topic, queueName, streamName sql.NullString
	var lastError, attrs sql.NullString
	var lastErrorAt sql.NullTime
	if err := scanner.Scan(&id, &userName, &name, &execUser, &autoStart, &disabled, &lastError, &lastErrorAt, &task, &brdg, &topic, &qos, &queueName, &streamName, &attrs); err != nil {
		return nil, err
	}
	def := &SubscriberDefinition{
		Id:          id,
		UserName:    userName.String,
		Name:        name.String,
		ExecUser:    execUser.String,
		AutoStart:   autoStart != 0,
		Disabled:    disabled != 0,
		LastError:   lastError.String,
		LastErrorAt: lastErrorAt.Time,
		Task:        task.String,
		Bridge:      brdg.String,
		Topic:       topic.String,
		QoS:         int(qos.Int64),
		QueueName:   queueName.String,
		StreamName:  streamName.String,
	}
	if attrs.Valid && strings.TrimSpace(attrs.String) != "" {
		def.Attributes = json.RawMessage(attrs.String)
	}
	return def, nil
}

// LoadAllSubscribers returns every subscriber definition.
func (s *Provider) LoadAllSubscribers(ctx context.Context) ([]*SubscriberDefinition, error) {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, BRIDGE, TOPIC, QOS, QUEUE_NAME, STREAM_NAME, ATTRIBUTES FROM _NEO_SUBSCRIBER_DEF ORDER BY ID`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ret := []*SubscriberDefinition{}
	for rows.Next() {
		def, err := scanSubscriberDefinition(rows)
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

// LoadSubscriber loads a single subscriber definition by name.
//
// Deprecated: kept for backward compatibility with the file-based storage
// era. Prefer LoadSubscriberByID for new call sites.
func (s *Provider) LoadSubscriber(ctx context.Context, name string) (*SubscriberDefinition, error) {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	name = strings.ToUpper(strings.TrimSpace(name))
	row := conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, BRIDGE, TOPIC, QOS, QUEUE_NAME, STREAM_NAME, ATTRIBUTES FROM _NEO_SUBSCRIBER_DEF WHERE NAME = ?`, name)
	def, err := scanSubscriberDefinition(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("subscriber '%s' not found", name)
	}
	if err != nil {
		return nil, err
	}
	return def, nil
}

// LoadSubscriberByID loads a single subscriber definition by its auto
// increment ID.
func (s *Provider) LoadSubscriberByID(ctx context.Context, id int64) (*SubscriberDefinition, error) {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	row := conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, BRIDGE, TOPIC, QOS, QUEUE_NAME, STREAM_NAME, ATTRIBUTES FROM _NEO_SUBSCRIBER_DEF WHERE ID = ?`, id)
	def, err := scanSubscriberDefinition(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("subscriber id '%d' not found", id)
	}
	if err != nil {
		return nil, err
	}
	return def, nil
}

// SaveSubscriber inserts or updates a subscriber definition. When
// def.Id == 0 a new row is inserted with ExecUser recorded as the creator;
// otherwise the existing row is updated and ExecUser is left unchanged
// (creator is immutable).
func (s *Provider) SaveSubscriber(ctx context.Context, def *SubscriberDefinition) error {
	if def == nil {
		return errors.New("subscriber definition not specified")
	}
	if len(def.Name) == 0 {
		return errors.New("subscriber name not specified")
	}
	if len(def.ExecUser) == 0 {
		return errors.New("subscriber exec user not specified")
	}
	if len(def.Bridge) == 0 {
		return errors.New("subscriber bridge not specified")
	}
	if len(def.Topic) == 0 {
		return errors.New("subscriber topic not specified")
	}
	if len(def.Task) == 0 {
		return errors.New("subscriber task not specified")
	}
	name := strings.ToUpper(strings.TrimSpace(def.Name))
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if def.Id == 0 {
		result, err := conn.ExecContext(ctx, `INSERT INTO _NEO_SUBSCRIBER_DEF (USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, BRIDGE, TOPIC, QOS, QUEUE_NAME, STREAM_NAME, ATTRIBUTES) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			def.UserName, name, def.ExecUser, boolToShort(def.AutoStart), boolToShort(def.Disabled), nullIfEmpty(def.LastError), nullTime(def.LastErrorAt), def.Task, def.Bridge, def.Topic, def.QoS, nullIfEmpty(def.QueueName), nullIfEmpty(def.StreamName), attributesToDB(def.Attributes))
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

	result, err := conn.ExecContext(ctx, `UPDATE _NEO_SUBSCRIBER_DEF SET NAME = ?, EXEC_USER = ?, AUTO_START = ?, DISABLED = ?, LAST_ERROR = ?, LAST_ERROR_AT = ?, TASK = ?, BRIDGE = ?, TOPIC = ?, QOS = ?, QUEUE_NAME = ?, STREAM_NAME = ?, ATTRIBUTES = ? WHERE ID = ? AND USER_NAME = ?`,
		name, def.ExecUser, boolToShort(def.AutoStart), boolToShort(def.Disabled), nullIfEmpty(def.LastError), nullTime(def.LastErrorAt), def.Task, def.Bridge, def.Topic, def.QoS, nullIfEmpty(def.QueueName), nullIfEmpty(def.StreamName), attributesToDB(def.Attributes), def.Id, def.UserName)
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

// SetSubscriberRuntimeError records scheduler diagnostics without applying a
// user ownership scope. An empty error clears the diagnostic.
func (s *Provider) SetSubscriberRuntimeError(ctx context.Context, id int64, lastError string) error {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var lastErrorAt any
	if lastError != "" {
		lastErrorAt = time.Now()
	}
	_, err = conn.ExecContext(ctx, `UPDATE _NEO_SUBSCRIBER_DEF SET LAST_ERROR = ?, LAST_ERROR_AT = ? WHERE ID = ?`, nullIfEmpty(lastError), lastErrorAt, id)
	return err
}

// RemoveSubscriber deletes a subscriber definition by name.
//
// Deprecated: kept for backward compatibility with the file-based storage
// era. Prefer RemoveSubscriberByID for new call sites.
func (s *Provider) RemoveSubscriber(ctx context.Context, name string) error {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	name = strings.ToUpper(strings.TrimSpace(name))
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_SUBSCRIBER_DEF WHERE NAME = ?`, name)
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

// RemoveSubscriberByID deletes a subscriber definition by its auto
// increment ID.
func (s *Provider) RemoveSubscriberByID(ctx context.Context, id int64) error {
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_SUBSCRIBER_DEF WHERE ID = ?`, id)
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

// LoadSubscribers returns only definitions owned by scope.User.
func (s *Provider) LoadSubscribers(ctx context.Context, scope UserScope) ([]*SubscriberDefinition, error) {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, BRIDGE, TOPIC, QOS, QUEUE_NAME, STREAM_NAME, ATTRIBUTES FROM _NEO_SUBSCRIBER_DEF WHERE USER_NAME = ? ORDER BY ID`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	definitions := []*SubscriberDefinition{}
	for rows.Next() {
		definition, err := scanSubscriberDefinition(rows)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, rows.Err()
}

// LoadSubscriberForUser loads a subscriber only when owned by scope.User.
func (s *Provider) LoadSubscriberForUser(ctx context.Context, scope UserScope, id int64) (*SubscriberDefinition, error) {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	definition, err := scanSubscriberDefinition(conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, BRIDGE, TOPIC, QOS, QUEUE_NAME, STREAM_NAME, ATTRIBUTES FROM _NEO_SUBSCRIBER_DEF WHERE ID = ? AND USER_NAME = ?`, id, user))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("subscriber id '%d' not found", id)
	}
	return definition, err
}

// LoadSubscriberByNameForUser is retained only while schedule.* accepts names.
func (s *Provider) LoadSubscriberByNameForUser(ctx context.Context, scope UserScope, name string) (*SubscriberDefinition, error) {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	definition, err := scanSubscriberDefinition(conn.QueryRowContext(ctx, `SELECT ID, USER_NAME, NAME, EXEC_USER, AUTO_START, DISABLED, LAST_ERROR, LAST_ERROR_AT, TASK, BRIDGE, TOPIC, QOS, QUEUE_NAME, STREAM_NAME, ATTRIBUTES FROM _NEO_SUBSCRIBER_DEF WHERE NAME = ? AND USER_NAME = ?`, name, user))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("subscriber '%s' not found", name)
	}
	return definition, err
}

// SaveSubscriberForUser forces USER_NAME to scope.User before writing.
func (s *Provider) SaveSubscriberForUser(ctx context.Context, scope UserScope, definition *SubscriberDefinition) error {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	definition.UserName = user
	return s.SaveSubscriber(ctx, definition)
}

// RemoveSubscriberForUser deletes a subscriber only when owned by scope.User.
func (s *Provider) RemoveSubscriberForUser(ctx context.Context, scope UserScope, id int64) error {
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return err
	}
	conn, err := s.scheduleConn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_SUBSCRIBER_DEF WHERE ID = ? AND USER_NAME = ?`, id, user)
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
