package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

const SecretRefEnv = "NEOSHELL_SECRET_REF"

const defaultSecretTTL = 30 * time.Second

var errSecretNotFound = errors.New("secret not found")

type SecretItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type secretEntry struct {
	Items     []SecretItem
	ExpiresAt time.Time
}

type secretConsumeRequest struct {
	Token string `json:"token"`
}

type SecretConsumeResult struct {
	Items []SecretItem `json:"items"`
}

func (ctl *Controller) PutSecret(items []SecretItem, ttl time.Duration) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("secret items are required")
	}
	if ttl <= 0 {
		ttl = defaultSecretTTL
	}
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(randomBytes)
	entry := secretEntry{
		Items:     append([]SecretItem{}, items...),
		ExpiresAt: time.Now().Add(ttl),
	}

	ctl.secretMu.Lock()
	defer ctl.secretMu.Unlock()
	ctl.cleanupExpiredSecretsLocked(time.Now())
	if ctl.secrets == nil {
		ctl.secrets = map[string]secretEntry{}
	}
	ctl.secrets[token] = entry
	return token, nil
}

func (ctl *Controller) RevokeSecret(token string) {
	if token == "" {
		return
	}
	ctl.secretMu.Lock()
	defer ctl.secretMu.Unlock()
	delete(ctl.secrets, token)
}

func (ctl *Controller) ConsumeSecret(token string) ([]SecretItem, error) {
	if token == "" {
		return nil, fmt.Errorf("secret token is required")
	}
	now := time.Now()
	ctl.secretMu.Lock()
	defer ctl.secretMu.Unlock()
	ctl.cleanupExpiredSecretsLocked(now)
	entry, ok := ctl.secrets[token]
	if !ok {
		return nil, errSecretNotFound
	}
	delete(ctl.secrets, token)
	if now.After(entry.ExpiresAt) {
		return nil, errSecretNotFound
	}
	return append([]SecretItem{}, entry.Items...), nil
}

func (ctl *Controller) clearSecrets() {
	ctl.secretMu.Lock()
	defer ctl.secretMu.Unlock()
	for token := range ctl.secrets {
		delete(ctl.secrets, token)
	}
}

func (ctl *Controller) cleanupExpiredSecretsLocked(now time.Time) {
	for token, entry := range ctl.secrets {
		if now.After(entry.ExpiresAt) {
			delete(ctl.secrets, token)
		}
	}
}

func (ctl *Controller) rpcSecretConsume(req secretConsumeRequest) (SecretConsumeResult, error) {
	items, err := ctl.ConsumeSecret(req.Token)
	if err != nil {
		if errors.Is(err, errSecretNotFound) {
			return SecretConsumeResult{}, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
		}
		return SecretConsumeResult{}, invalidParamsError(err)
	}
	return SecretConsumeResult{Items: items}, nil
}

func ConsumeSecret(address string, token string) ([]SecretItem, error) {
	if address == "" {
		return nil, fmt.Errorf("service controller address is required")
	}
	if token == "" {
		return nil, fmt.Errorf("secret token is required")
	}
	network, rpcAddress, err := parseRPCAddress(address)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, rpcAddress, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	req := controllerRPCRequest{
		Version: jsonRPCVersion,
		Method:  "secret.consume",
		ID:      json.RawMessage(`1`),
	}
	params, err := json.Marshal(secretConsumeRequest{Token: token})
	if err != nil {
		return nil, err
	}
	req.Params = params
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}
	var resp struct {
		Version string              `json:"jsonrpc"`
		Result  SecretConsumeResult `json:"result"`
		Error   *controllerRPCError `json:"error"`
		ID      json.RawMessage     `json:"id"`
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result.Items, nil
}
