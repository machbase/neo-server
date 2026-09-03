package model

import (
	"context"
	"database/sql"
	"fmt"
)

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
