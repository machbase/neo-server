package mqtt

import (
	"fmt"
	"strings"
	"sync"
)

type OtpPrefixes interface {
	Set(prefix string, key string)
	Get(key string) string
	Match(key string) (string, bool)
}

type otpPrefixes struct {
	keys sync.Map
}

func NewOtpPrefixes() OtpPrefixes {
	return &otpPrefixes{}
}

func (px *otpPrefixes) Set(prefix string, key string) {
	px.keys.Store(prefix, key)
}

func (px *otpPrefixes) Get(key string) string {
	if v, ok := px.keys.Load(key); ok {
		return fmt.Sprintf("%s%s", v.(string), key)
	}
	return key
}

func (px *otpPrefixes) Match(key string) (string, bool) {
	var found = false
	px.keys.Range(func(k any, v any) bool {
		if strings.HasPrefix(key, k.(string)) {
			key = fmt.Sprintf("%s%s", v.(string), key)
			found = true
			return false
		}
		return true
	})
	return key, found
}
