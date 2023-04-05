package security

import (
	"sync"
	"time"
)

type AuthServer interface {
	ValidateClientToken(token string) (bool, error)
	ValidateClientCertificate(clientId string, certHash string) (bool, error)
	ValidateSshPublicKey(keyType string, key string) (bool, error)
}

type JwtCacheValue struct {
	Rt   string
	When time.Time
}

type JwtCache interface {
	SetRefreshToken(id string, rt string)
	GetRefreshToken(id string) (string, bool)
	RemoveRefreshToken(id string)
}

type jwtMemCache struct {
	rtTable map[string]*JwtCacheValue
	rtLock  sync.RWMutex
}

func NewJwtCache() JwtCache {
	return &jwtMemCache{
		rtTable: make(map[string]*JwtCacheValue),
	}
}

func (svr *jwtMemCache) SetRefreshToken(id string, rt string) {
	svr.rtLock.Lock()
	defer svr.rtLock.Unlock()
	svr.rtTable[id] = &JwtCacheValue{
		Rt:   rt,
		When: time.Now(),
	}
}

func (svr *jwtMemCache) GetRefreshToken(id string) (string, bool) {
	svr.rtLock.RLock()
	defer svr.rtLock.RUnlock()
	val, ok := svr.rtTable[id]
	if val != nil {
		return val.Rt, ok
	} else {
		return "", ok
	}
}

func (svr *jwtMemCache) RemoveRefreshToken(id string) {
	svr.rtLock.Lock()
	defer svr.rtLock.Unlock()
	delete(svr.rtTable, id)
}
