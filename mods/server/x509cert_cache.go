package server

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/machbase/neo-server/v8/mods/model"
)

// x509CertRecord is keyed by CERT_HASH, the only value that is guaranteed
// unique per issued certificate (the CN/Name is not unique across owners).
type x509CertRecord struct {
	Id        int64
	UserName  string
	NotBefore time.Time
	NotAfter  time.Time

	mu       sync.Mutex
	lastUsed time.Time
}

type X509CertVerifier struct {
	provider *model.Provider
	positive *ttlcache.Cache[string, *x509CertRecord]
	negative *ttlcache.Cache[string, struct{}]
	touchSem chan struct{}
}

func NewX509CertVerifier(provider *model.Provider) *X509CertVerifier {
	return &X509CertVerifier{
		provider: provider,
		positive: ttlcache.New(ttlcache.WithTTL[string, *x509CertRecord](5*time.Minute), ttlcache.WithCapacity[string, *x509CertRecord](2048)),
		negative: ttlcache.New(ttlcache.WithTTL[string, struct{}](10*time.Second), ttlcache.WithCapacity[string, struct{}](2048)),
		touchSem: make(chan struct{}, 8),
	}
}

func (v *X509CertVerifier) Validate(ctx context.Context, certHash string) (bool, error) {
	_, ok, err := v.Resolve(ctx, certHash)
	return ok, err
}

func (v *X509CertVerifier) Resolve(ctx context.Context, certHash string) (*x509CertRecord, bool, error) {
	item := v.positive.Get(certHash)
	if item == nil {
		if v.negative.Get(certHash) != nil {
			return nil, false, nil
		}
		definition, err := v.provider.GetX509CertByHash(ctx, certHash)
		if err != nil {
			if err == sql.ErrNoRows {
				v.negative.Set(certHash, struct{}{}, ttlcache.DefaultTTL)
				return nil, false, nil
			}
			return nil, false, err
		}
		item = v.positive.Set(certHash, &x509CertRecord{Id: definition.Id, UserName: definition.UserName, NotBefore: definition.NotBefore, NotAfter: definition.NotAfter}, ttlcache.DefaultTTL)
	}
	record := item.Value()
	now := time.Now()
	if record.UserName == "" || (!record.NotBefore.IsZero() && now.Before(record.NotBefore)) || (!record.NotAfter.IsZero() && !now.Before(record.NotAfter)) {
		return nil, false, nil
	}
	v.touch(record, now)
	return record, true, nil
}

func (v *X509CertVerifier) Invalidate(certHash string) {
	v.positive.Delete(certHash)
	v.negative.Delete(certHash)
}

// touch throttles LAST_USED_AT updates to at most once per minute per record, and
// caps in-flight background writes so a slow or unavailable DB cannot pile up
// unbounded goroutines/connections.
func (v *X509CertVerifier) touch(record *x509CertRecord, now time.Time) {
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
		_ = v.provider.TouchX509Cert(ctx, record.Id, now)
	}()
}
