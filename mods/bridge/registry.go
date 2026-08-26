package bridge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/mods/bridge/connector"
	"github.com/machbase/neo-server/v8/mods/model"
)

// registry is a pure reference holder for live Bridge instances, keyed by
// the stable _NEO_BRIDGE_DEF.ID. It does not know about owners or scope;
// "who can see this bridge by name" is decided by BridgeDefProvider.
var registry = map[int64]Bridge{}
var registryLock sync.RWMutex

var defProvider BridgeProvider

// SetBridgeProvider wires the bridge definition provider used by scope-aware
// lookups (GetBridge, GetSqlBridge, GetMqttBridge, ResolveSqlDB). It should
// be called once during server bootstrap.
func SetBridgeProvider(p BridgeProvider) {
	defProvider = p
}

func RegisterByID(def *model.BridgeDefinition) (err error) {
	registryLock.Lock()
	defer registryLock.Unlock()

	var br Bridge
	switch def.Type {
	case model.BRIDGE_SQLITE:
		br = connector.NewSqliteBridge(def.Name, def.Path)
	case model.BRIDGE_POSTGRES:
		br = connector.NewPostgresBridge(def.Name, def.Path)
	case model.BRIDGE_MYSQL:
		br = connector.NewMySQLBridge(def.Name, def.Path)
	case model.BRIDGE_MSSQL:
		br = connector.NewMSSQLBridge(def.Name, def.Path)
	case model.BRIDGE_MQTT:
		br = NewMqttBridge(def.Name, def.Path)
	case model.BRIDGE_NATS:
		br = NewNatsBridge(def.Name, def.Path)
	default:
		return fmt.Errorf("undefined bridge type %s, unable to register", def.Type)
	}

	if err = br.BeforeRegister(); err != nil {
		return err
	}
	registry[def.Id] = br
	if sqlBridge, ok := br.(SqlBridge); ok {
		connector.SetDatabaseByID(def.Id, sqlBridge.DB())
	}
	return nil
}

func UnregisterByID(id int64) {
	registryLock.Lock()
	defer registryLock.Unlock()

	if c, ok := registry[id]; ok {
		delete(registry, id)
		if _, ok := c.(SqlBridge); ok {
			connector.UnsetDatabaseByID(id)
		}
		c.AfterUnregister()
	}
}

func UnregisterAll() {
	registryLock.RLock()
	ids := make([]int64, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	registryLock.RUnlock()

	for _, id := range ids {
		UnregisterByID(id)
	}
}

func GetBridgeByID(id int64) (Bridge, error) {
	registryLock.RLock()
	defer registryLock.RUnlock()

	if c, ok := registry[id]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("undefined bridge id '%d'", id)
}

// GetBridge resolves a bridge by name within the given user scope: it first
// asks the definition provider whether the requesting user may see a bridge
// with this name (owner, public, or explicitly allowed), then looks up the
// live instance by its ID.
func GetBridge(ctx context.Context, scope model.UserScope, name string) (Bridge, error) {
	if defProvider == nil {
		return nil, fmt.Errorf("bridge definition provider is not configured")
	}
	def, err := defProvider.LoadBridge(ctx, scope, name)
	if err != nil {
		return nil, err
	}
	return GetBridgeByID(def.Id)
}

func GetSqlBridge(ctx context.Context, scope model.UserScope, name string) (SqlBridge, error) {
	br, err := GetBridge(ctx, scope, name)
	if err != nil {
		return nil, err
	}

	if sqlBr, ok := br.(SqlBridge); ok {
		return sqlBr, nil
	} else {
		return nil, fmt.Errorf("'%s' is not a SqlBridge", name)
	}
}

func GetMqttBridge(ctx context.Context, scope model.UserScope, name string) (*MqttBridge, error) {
	br, err := GetBridge(ctx, scope, name)
	if err != nil {
		return nil, err
	}

	if mqttBr, ok := br.(*MqttBridge); ok {
		return mqttBr, nil
	} else {
		return nil, fmt.Errorf("'%s' is not a MqttBridge", name)
	}
}

var onTheFlyPrefixes = []string{"sqlite,", "mssql,", "postgres,", "mysql,"}

// ResolveSqlDB resolves a *sql.DB either from an on-the-fly connection spec
// (e.g. "mysql,tcp://...") or from a name-scoped registered bridge. On-the-fly
// specs bypass scope entirely (the caller already holds the credentials);
// named references are resolved through the same scope rules as GetBridge.
func ResolveSqlDB(ctx context.Context, scope model.UserScope, nameOrDSN string) (*sql.DB, error) {
	for _, prefix := range onTheFlyPrefixes {
		if strings.HasPrefix(nameOrDSN, prefix) {
			return connector.Database(nameOrDSN)
		}
	}
	if defProvider == nil {
		return nil, fmt.Errorf("bridge definition provider is not configured")
	}
	def, err := defProvider.LoadBridge(ctx, scope, nameOrDSN)
	if err != nil {
		return nil, err
	}
	return connector.GetDatabaseByID(def.Id)
}
