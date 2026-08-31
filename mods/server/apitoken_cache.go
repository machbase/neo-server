package server

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/machbase/neo-server/v8/mods/model"
)

type apiTokenRecord struct {
	UserName  string
	TokenHash string
	NotBefore time.Time
	NotAfter  time.Time

	mu       sync.Mutex
	lastUsed time.Time
}

type ApiTokenVerifier struct {
	provider *model.Provider
	positive *ttlcache.Cache[int64, *apiTokenRecord]
	negative *ttlcache.Cache[int64, struct{}]
	touchSem chan struct{}
}

func NewApiTokenVerifier(provider *model.Provider) *ApiTokenVerifier {
	return &ApiTokenVerifier{
		provider: provider,
		positive: ttlcache.New(ttlcache.WithTTL[int64, *apiTokenRecord](5*time.Minute), ttlcache.WithCapacity[int64, *apiTokenRecord](4096)),
		negative: ttlcache.New(ttlcache.WithTTL[int64, struct{}](10*time.Second), ttlcache.WithCapacity[int64, struct{}](4096)),
		touchSem: make(chan struct{}, 8),
	}
}

func (v *ApiTokenVerifier) Verify(ctx context.Context, token string) (string, bool, error) {
	id, secret, ok := ParseApiToken(token)
	if !ok {
		return "", false, nil
	}
	record := v.positive.Get(id)
	if record == nil {
		if v.negative.Get(id) != nil {
			return "", false, nil
		}
		definition, err := v.provider.GetApiToken(ctx, id)
		if err != nil {
			if err == sql.ErrNoRows {
				v.negative.Set(id, struct{}{}, ttlcache.DefaultTTL)
				return "", false, nil
			}
			return "", false, err
		}
		record = v.positive.Set(id, &apiTokenRecord{UserName: definition.UserName, TokenHash: definition.TokenHash, NotBefore: definition.NotBefore, NotAfter: definition.NotAfter}, ttlcache.DefaultTTL)
	}
	value := record.Value()
	now := time.Now()
	if value.UserName == "" || (!value.NotBefore.IsZero() && now.Before(value.NotBefore)) || (!value.NotAfter.IsZero() && !now.Before(value.NotAfter)) {
		return "", false, nil
	}
	expected, err := hex.DecodeString(value.TokenHash)
	if err != nil {
		return "", false, err
	}
	actual, _ := hex.DecodeString(HashApiTokenSecret(secret))
	if subtle.ConstantTimeCompare(expected, actual) != 1 {
		return "", false, nil
	}
	v.touch(id, value, now)
	return value.UserName, true, nil
}

func (v *ApiTokenVerifier) Invalidate(id int64) {
	v.positive.Delete(id)
	v.negative.Delete(id)
}

func (v *ApiTokenVerifier) touch(id int64, record *apiTokenRecord, now time.Time) {
	record.mu.Lock()
	if now.Sub(record.lastUsed) < time.Minute {
		record.mu.Unlock()
		return
	}
	record.lastUsed = now
	record.mu.Unlock()
	select {
	case v.touchSem <- struct{}{}:
	default:
		// too many touches already in flight; skip, the next successful auth will retry
		return
	}
	go func() {
		defer func() { <-v.touchSem }()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = v.provider.TouchApiToken(ctx, id, now)
	}()
}
