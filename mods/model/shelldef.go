package model

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util"
)

type customShellIdError struct {
	id string
}

func (e customShellIdError) Error() string {
	return fmt.Sprintf("invalid shell id '%s'", e.id)
}

const (
	SHELL_TERM = "term"
)

const (
	SHELLID_SQL   = "SQL"
	SHELLID_TQL   = "TQL"
	SHELLID_WRK   = "WRK"
	SHELLID_TAZ   = "TAZ"
	SHELLID_DSH   = "DSH"
	SHELLID_SHELL = "SHELL"
	SHELLID_JSH   = "JSH"
)

var reservedShellNames = []string{"SQL", "TQL", "WORKSHEET", "TAG ANALYZER", "SHELL", "JSH",
	/*and more for future uses*/
	"WORKBOOK", "SCRIPT", "RUN", "CMD", "COMMAND", "CONSOLE", "MONITOR", "CHART",
	"DASHBOARD", "LOG", "HOME", "PLAYGROUND", "GRAPH", "FLOW", "DIAGRAM", "PLOT"}

var reservedWebShellDef = map[string]*ShellDefinition{
	SHELLID_SQL: {Type: "sql", Label: "SQL", Icon: "file-document-outline", Id: SHELLID_SQL},
	SHELLID_TQL: {Type: "tql", Label: "TQL", Icon: "chart-scatter-plot", Id: SHELLID_TQL},
	SHELLID_WRK: {Type: "wrk", Label: "WORKSHEET", Icon: "clipboard-text-play-outline", Id: SHELLID_WRK},
	SHELLID_TAZ: {Type: "taz", Label: "TAG ANALYZER", Icon: "chart-line", Id: SHELLID_TAZ},
	SHELLID_DSH: {Type: "dsh", Label: "DASHBOARD", Icon: "dashboard", Id: SHELLID_DSH},
	SHELLID_JSH: {Type: SHELL_TERM, Label: "JSH", Icon: "fish", Id: SHELLID_JSH,
		Attributes: &ShellAttributes{},
	},
	SHELLID_SHELL: {Type: SHELL_TERM, Label: "SHELL", Icon: "console", Id: SHELLID_SHELL,
		Attributes: &ShellAttributes{Cloneable: true},
	},
}

type ShellDefinition struct {
	Id         string           `json:"id"`
	Type       string           `json:"type"`
	Icon       string           `json:"icon,omitempty"`
	Label      string           `json:"label"`
	Theme      string           `json:"theme,omitempty"`
	Command    string           `json:"command,omitempty"`
	Attributes *ShellAttributes `json:"attributes,omitempty"`
}

func (def *ShellDefinition) Clone() *ShellDefinition {
	ret := &ShellDefinition{}
	ret.Id = def.Id
	ret.Type = def.Type
	ret.Icon = def.Icon
	ret.Label = def.Label
	ret.Theme = def.Theme
	ret.Command = def.Command
	if def.Attributes != nil {
		ret.Attributes = &ShellAttributes{}
		ret.Attributes.Cloneable = def.Attributes.Cloneable
		ret.Attributes.Removable = def.Attributes.Removable
		ret.Attributes.Editable = def.Attributes.Editable
	}
	return ret
}

type ShellAttributes struct {
	Removable bool `json:"removable"`
	Cloneable bool `json:"cloneable"`
	Editable  bool `json:"editable"`
}

func (att *ShellAttributes) MarshalJSON() ([]byte, error) {
	itm := []string{}
	if att.Removable {
		itm = append(itm, `{"removable":true}`)
	}
	if att.Cloneable {
		itm = append(itm, `{"cloneable":true}`)
	}
	if att.Editable {
		itm = append(itm, `{"editable":true}`)
	}
	b := bytes.Buffer{}
	b.WriteString("[")
	b.WriteString(strings.Join(itm, ","))
	b.WriteString("]")
	return b.Bytes(), nil
}

func (att *ShellAttributes) UnmarshalJSON(data []byte) error {
	obj := map[string]any{}
	if err := json.Unmarshal(data, &obj); err == nil {
		att.applyMap(obj)
		return nil
	}
	maps := []map[string]any{}
	err := json.Unmarshal(data, &maps)
	if err != nil {
		return err
	}
	for _, m := range maps {
		att.applyMap(m)
	}
	return nil
}

func (att *ShellAttributes) applyMap(m map[string]any) {
	toBool := func(v any) bool {
		switch vv := v.(type) {
		case bool:
			return vv
		case string:
			if b, err := strconv.ParseBool(vv); err != nil {
				return false
			} else {
				return b
			}
		default:
			return false
		}
	}
	if v, ok := m["removable"]; ok {
		att.Removable = toBool(v)
	}
	if v, ok := m["cloneable"]; ok {
		att.Cloneable = toBool(v)
	}
	if v, ok := m["editable"]; ok {
		att.Editable = toBool(v)
	}
}

type shellAttributesObject ShellAttributes

func marshalShellAttributesForDB(att *ShellAttributes) (string, error) {
	if att == nil {
		return "{}", nil
	}
	data, err := json.Marshal((*shellAttributesObject)(att))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalShellAttributesFromDB(data string) (*ShellAttributes, error) {
	if strings.TrimSpace(data) == "" {
		return nil, nil
	}
	obj := shellAttributesObject{}
	if err := json.Unmarshal([]byte(data), &obj); err == nil {
		att := ShellAttributes(obj)
		return &att, nil
	}
	att := &ShellAttributes{}
	if err := json.Unmarshal([]byte(data), att); err != nil {
		return nil, err
	}
	return att, nil
}

func (s *Provider) SetDefaultShellCommand(cmd string) {
	reservedWebShellDef[SHELLID_SHELL].Command = cmd
}

func (s *Provider) SetDefaultJshCommand(cmd string) {
	reservedWebShellDef[SHELLID_JSH].Command = cmd
}

func (s *Provider) GetShell(ctx context.Context, scope UserScope, id string) (*ShellDefinition, error) {
	id = strings.ToUpper(id)
	ret := reservedWebShellDef[id]
	if ret != nil {
		return ret, nil
	}
	shellId, err := parseCustomShellId(id)
	if err != nil {
		return nil, err
	}
	return s.loadShellDef(ctx, scope, shellId)
}

func (s *Provider) GetAllShells(ctx context.Context, scope UserScope, includesWebShells bool) ([]*ShellDefinition, error) {
	var ret []*ShellDefinition
	if includesWebShells {
		ret = append(ret, reservedWebShellDef[SHELLID_SQL])
		ret = append(ret, reservedWebShellDef[SHELLID_TQL])
		ret = append(ret, reservedWebShellDef[SHELLID_TAZ])
		ret = append(ret, reservedWebShellDef[SHELLID_DSH])
		ret = append(ret, reservedWebShellDef[SHELLID_WRK])
		ret = append(ret, reservedWebShellDef[SHELLID_JSH])
		ret = append(ret, reservedWebShellDef[SHELLID_SHELL])
	}
	err := s.iterateShellDefs(ctx, scope, func(def *ShellDefinition) bool {
		ret = append(ret, def)
		return true
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (s *Provider) CopyShell(ctx context.Context, scope UserScope, id string) (*ShellDefinition, error) {
	id = strings.ToUpper(id)
	var ret *ShellDefinition
	if _, ok := reservedWebShellDef[id]; ok {
		ret = &ShellDefinition{}
		ret.Type = SHELL_TERM
		ret.Attributes = &ShellAttributes{Removable: true, Editable: true, Cloneable: true}
		if exename, err := os.Executable(); err != nil {
			ret.Command = fmt.Sprintf(`"%s" shell`, os.Args[0])
		} else {
			ret.Command = fmt.Sprintf(`"%s" shell`, exename)
		}
	} else {
		d, err := s.GetShell(ctx, scope, id)
		if err != nil {
			return nil, err
		}
		if d == nil {
			s.log.Warnf("shell def not found '%s'", id)
			return nil, fmt.Errorf("shell definition not found '%s'", id)
		}
		ret = d.Clone()
	}
	if ret == nil {
		s.log.Warnf("shell def not found '%s'", id)
		return nil, fmt.Errorf("shell definition not found '%s'", id)
	}
	ret.Id = ""
	ret.Label = "CUSTOM SHELL"
	if err := s.SaveShell(ctx, scope, ret); err != nil {
		s.log.Warn("shell def not saved,", err.Error())
		return nil, err
	}
	return ret, nil
}

func (s *Provider) RemoveShell(ctx context.Context, scope UserScope, id string) error {
	shellId, err := parseCustomShellId(id)
	if err != nil {
		return err
	}
	return s.deleteShellDef(ctx, scope, shellId)
}

func (s *Provider) SaveShell(ctx context.Context, scope UserScope, def *ShellDefinition) error {
	if def == nil {
		return errors.New("shell definition not specified")
	}
	label := strings.ToUpper(strings.TrimSpace(def.Label))
	for _, n := range reservedShellNames {
		if label == n {
			return fmt.Errorf("'%s' is not allowed for the custom shell name", def.Label)
		}
	}
	if def.Id != "" {
		if _, err := parseCustomShellId(def.Id); err != nil {
			return err
		}
	}
	if len(def.Command) == 0 {
		return errors.New("invalid command for the custom shell")
	}
	args := util.SplitFields(def.Command, true)
	if len(args) == 0 {
		return errors.New("invalid command for the custom shell")
	}
	binpath := args[0]
	if fi, err := os.Stat(binpath); err != nil {
		return fmt.Errorf("'%s' is not accessible, %s", binpath, err.Error())
	} else {
		if fi.IsDir() {
			return fmt.Errorf("'%s' is not executable", binpath)
		}
		if runtime.GOOS == "windows" {
			if !strings.HasSuffix(strings.ToLower(binpath), ".exe") && !strings.HasSuffix(strings.ToLower(binpath), ".com") {
				return fmt.Errorf("'%s' is not executable", binpath)
			}
		} else {
			if fi.Mode().Perm()&0111 == 0 {
				return fmt.Errorf("'%s' is not executable", binpath)
			}
		}
	}
	return s.saveShellDef(ctx, scope, def)
}

func (s *Provider) normalizeContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	return nil
}

func (s *Provider) normalizeUserScope(scope UserScope) (string, error) {
	user := strings.ToUpper(strings.TrimSpace(scope.User))
	if user == "" {
		return "", errors.New("user scope is empty")
	}
	return user, nil
}

func (s *Provider) shellConn(ctx context.Context, scope UserScope) (*sql.Conn, string, error) {
	if err := s.normalizeContext(ctx); err != nil {
		return nil, "", err
	}
	user, err := s.normalizeUserScope(scope)
	if err != nil {
		return nil, "", err
	}
	if s.connect == nil {
		return nil, "", errors.New("database connect function is not configured")
	}
	conn, err := s.connect(ctx, "sys")
	if err != nil {
		return nil, "", err
	}
	if err := s.ensureShellTable(ctx, conn); err != nil {
		conn.Close()
		return nil, "", err
	}
	return conn, user, nil
}

func (s *Provider) ensureShellTable(ctx context.Context, conn *sql.Conn) error {
	s.shellTableMu.Lock()
	defer s.shellTableMu.Unlock()
	if s.shellTableReady {
		return nil
	}
	_, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _NEO_SHELL_DEF (
ID LONG PRIMARY KEY AUTO_INCREMENT,
USER_NAME VARCHAR(64) NOT NULL,
TYPE VARCHAR(32) NOT NULL,
ICON VARCHAR(128),
LABEL VARCHAR(256) NOT NULL,
THEME VARCHAR(128),
COMMAND VARCHAR(4096) NOT NULL,
ATTRIBUTES JSON
)`)
	if err != nil {
		return err
	}
	s.shellTableReady = true
	return nil
}

func parseCustomShellId(id string) (int64, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, customShellIdError{id: id}
	}
	if _, ok := reservedWebShellDef[strings.ToUpper(id)]; ok {
		return 0, customShellIdError{id: id}
	}
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, customShellIdError{id: id}
	}
	return parsed, nil
}

func formatCustomShellId(id int64) string {
	return strconv.FormatInt(id, 10)
}

func (s *Provider) loadShellDef(ctx context.Context, scope UserScope, id int64) (*ShellDefinition, error) {
	conn, user, err := s.shellConn(ctx, scope)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	row := conn.QueryRowContext(ctx, `SELECT ID, TYPE, ICON, LABEL, THEME, COMMAND, ATTRIBUTES FROM _NEO_SHELL_DEF WHERE USER_NAME = ? AND ID = ?`, user, id)
	def, err := scanShellDefinition(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return normalizeShellDefinition(def), nil
}

type shellScanner interface {
	Scan(dest ...any) error
}

func scanShellDefinition(scanner shellScanner) (*ShellDefinition, error) {
	var id int64
	var typ, icon, label, theme, command sql.NullString
	var attrs sql.NullString
	if err := scanner.Scan(&id, &typ, &icon, &label, &theme, &command, &attrs); err != nil {
		return nil, err
	}
	def := &ShellDefinition{
		Id:      formatCustomShellId(id),
		Type:    typ.String,
		Icon:    icon.String,
		Label:   label.String,
		Theme:   theme.String,
		Command: command.String,
	}
	if attrs.Valid && strings.TrimSpace(attrs.String) != "" {
		attrs, err := unmarshalShellAttributesFromDB(attrs.String)
		if err != nil {
			return nil, err
		}
		def.Attributes = attrs
	}
	return def, nil
}

func (s *Provider) saveShellDef(ctx context.Context, scope UserScope, def *ShellDefinition) error {
	conn, user, err := s.shellConn(ctx, scope)
	if err != nil {
		return err
	}
	defer conn.Close()
	attrs, err := marshalShellAttributesForDB(def.Attributes)
	if err != nil {
		return err
	}
	if def.Id == "" {
		result, err := conn.ExecContext(ctx, `INSERT INTO _NEO_SHELL_DEF (USER_NAME, TYPE, ICON, LABEL, THEME, COMMAND, ATTRIBUTES) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			user, def.Type, def.Icon, def.Label, def.Theme, def.Command, attrs)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		def.Id = formatCustomShellId(id)
		return nil
	}
	id, err := parseCustomShellId(def.Id)
	if err != nil {
		return err
	}
	result, err := conn.ExecContext(ctx, `UPDATE _NEO_SHELL_DEF SET TYPE = ?, ICON = ?, LABEL = ?, THEME = ?, COMMAND = ?, ATTRIBUTES = ? WHERE USER_NAME = ? AND ID = ?`,
		def.Type, def.Icon, def.Label, def.Theme, def.Command, attrs, user, id)
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

func (s *Provider) deleteShellDef(ctx context.Context, scope UserScope, id int64) error {
	conn, user, err := s.shellConn(ctx, scope)
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := conn.ExecContext(ctx, `DELETE FROM _NEO_SHELL_DEF WHERE USER_NAME = ? AND ID = ?`, user, id)
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

func (s *Provider) iterateShellDefs(ctx context.Context, scope UserScope, cb func(*ShellDefinition) bool) error {
	if cb == nil {
		return nil
	}
	conn, user, err := s.shellConn(ctx, scope)
	if err != nil {
		return err
	}
	defer conn.Close()
	rows, err := conn.QueryContext(ctx, `SELECT ID, TYPE, ICON, LABEL, THEME, COMMAND, ATTRIBUTES FROM _NEO_SHELL_DEF WHERE USER_NAME = ? ORDER BY ID`, user)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		def, err := scanShellDefinition(rows)
		if err != nil {
			return err
		}
		def = normalizeShellDefinition(def)
		shouldContinue := cb(def)
		if !shouldContinue {
			break
		}
	}
	return rows.Err()
}

func normalizeShellDefinition(def *ShellDefinition) *ShellDefinition {
	if def.Type == "" {
		def.Type = SHELL_TERM
	}
	if def.Attributes == nil {
		def.Attributes = &ShellAttributes{
			Cloneable: true, Removable: true, Editable: true,
		}
	}
	if def.Icon == "" {
		def.Icon = "console-network-outline"
	}
	if def.Label == "" {
		def.Label = "CUSTOM SHELL"
	}
	return def
}
