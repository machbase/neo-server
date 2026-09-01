package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/model"
)

const ApiTokenPrefix = "nt_"

const apiTokenAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func NewApiTokenSecret() (string, error) {
	const length = 43
	secret := make([]byte, length)
	for index := 0; index < length; {
		var randomByte [1]byte
		if _, err := rand.Read(randomByte[:]); err != nil {
			return "", err
		}
		if randomByte[0] >= 248 {
			continue
		}
		secret[index] = apiTokenAlphabet[randomByte[0]%byte(len(apiTokenAlphabet))]
		index++
	}
	return string(secret), nil
}

func FormatApiToken(id int64, secret string) string {
	return ApiTokenPrefix + strconv.FormatInt(id, 36) + "_" + secret
}

func ParseApiToken(token string) (int64, string, bool) {
	if !strings.HasPrefix(token, ApiTokenPrefix) {
		return 0, "", false
	}
	parts := strings.Split(strings.TrimPrefix(token, ApiTokenPrefix), "_")
	if len(parts) != 2 || parts[0] == "" || len(parts[1]) != 43 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[0], 36, 64)
	if err != nil || id < 1 {
		return 0, "", false
	}
	for _, character := range parts[1] {
		if !strings.ContainsRune(apiTokenAlphabet, character) {
			return 0, "", false
		}
	}
	return id, parts[1], true
}

func HashApiTokenSecret(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(hash[:])
}

func HintApiToken(id int64, secret string) string {
	if len(secret) < 8 {
		return ApiTokenPrefix + strconv.FormatInt(id, 36) + "_****"
	}
	return ApiTokenPrefix + strconv.FormatInt(id, 36) + "_" + secret[:4] + "****" + secret[len(secret)-4:]
}

type ApiTokenInfo struct {
	Id         int64  `json:"id"`
	Name       string `json:"name"`
	User       string `json:"user"`
	Hint       string `json:"hint"`
	CreatedAt  int64  `json:"createdAt"`
	NotAfter   int64  `json:"notAfter"`
	LastUsedAt int64  `json:"lastUsedAt,omitempty"`
}

type GeneratedApiToken struct {
	ApiTokenInfo
	Token string `json:"token"`
}

// listApiTokens returns the caller's own API tokens.
//
// params:
//
// return: API token information list
func (s *Server) listApiTokens(ctx context.Context) ([]*ApiTokenInfo, error) {
	if s.models == nil {
		return nil, errors.New("model provider is not available")
	}
	scope, err := modelUserScopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	definitions, err := s.models.GetAllApiTokens(ctx, scope)
	if err != nil {
		return nil, err
	}
	result := make([]*ApiTokenInfo, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, apiTokenInfo(definition))
	}
	return result, nil
}

// generateApiToken generates a new API token owned by the caller.
//
// params:
//   - name: a label for the token
//   - notAfter: expiration unix epoch in seconds; 0 uses the default of 10 years from now
//
// return: the generated API token, including the one-time plaintext token value
func (s *Server) generateApiToken(ctx context.Context, name string, notAfter int64) (*GeneratedApiToken, error) {
	if s.models == nil {
		return nil, errors.New("model provider is not available")
	}
	scope, err := modelUserScopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("token name is required")
	}
	if notAfter == 0 {
		notAfter = time.Now().AddDate(10, 0, 0).Unix()
	}
	secret, err := NewApiTokenSecret()
	if err != nil {
		return nil, err
	}
	definition := &model.ApiTokenDefinition{Name: name, TokenHash: HashApiTokenSecret(secret), TokenHint: "pending", CreatedAt: time.Now(), NotBefore: time.Now(), NotAfter: time.Unix(notAfter, 0)}
	if err := s.models.SaveApiToken(ctx, scope, definition); err != nil {
		return nil, err
	}
	definition.TokenHint = HintApiToken(definition.Id, secret)
	if err := s.models.UpdateApiTokenHint(ctx, scope, definition.Id, definition.TokenHint); err != nil {
		return nil, err
	}
	if s.apiTokenVerifier != nil {
		s.apiTokenVerifier.Invalidate(definition.Id)
	}
	return &GeneratedApiToken{ApiTokenInfo: *apiTokenInfo(definition), Token: FormatApiToken(definition.Id, secret)}, nil
}

// deleteApiToken deletes one of the caller's own API tokens.
//
// params:
//   - id: API token identifier
//
// return: null on success
func (s *Server) deleteApiToken(ctx context.Context, id int64) error {
	if s.models == nil {
		return errors.New("model provider is not available")
	}
	scope, err := modelUserScopeFromContext(ctx)
	if err != nil {
		return err
	}
	if err := s.models.RemoveApiToken(ctx, scope, id); err != nil {
		return err
	}
	if s.apiTokenVerifier != nil {
		s.apiTokenVerifier.Invalidate(id)
	}
	return nil
}

func apiTokenInfo(definition *model.ApiTokenDefinition) *ApiTokenInfo {
	info := &ApiTokenInfo{Id: definition.Id, Name: definition.Name, User: definition.UserName, Hint: definition.TokenHint, CreatedAt: definition.CreatedAt.Unix(), NotAfter: definition.NotAfter.Unix()}
	if !definition.LastUsedAt.IsZero() {
		info.LastUsedAt = definition.LastUsedAt.Unix()
	}
	return info
}
