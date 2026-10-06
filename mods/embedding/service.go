package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-client/v2/api"
)

const (
	BGEModel     = "BAAI/bge-m3@5617a9f61b028005a4858fdac845db406aefb181"
	BGESpace     = "bge-m3-5617a9f6-onnx-l2-v1"
	BGEDimension = 1024
)

type Config struct {
	DefaultProvider   string
	CacheDir          string
	RuntimeLibrary    string
	MaxTokens         int
	ExternalURL       string
	ExternalModel     string
	ExternalSpace     string
	ExternalDimension int
	ExternalAPIKeyEnv string
	ExternalTimeout   time.Duration
}

func DefaultConfig() Config {
	externalDimension, _ := strconv.Atoi(os.Getenv("NEO_EMBED_EXTERNAL_DIMENSION"))
	cache := os.Getenv("NEO_EMBED_CACHE")
	if cache == "" {
		root, err := os.UserCacheDir()
		if err != nil {
			root = os.TempDir()
		}
		cache = filepath.Join(root, "machbase-neo", "embedding")
	}
	return Config{
		DefaultProvider:   "bge-m3",
		CacheDir:          cache,
		RuntimeLibrary:    os.Getenv("ONNXRUNTIME_LIB_PATH"),
		MaxTokens:         1024,
		ExternalURL:       os.Getenv("NEO_EMBED_EXTERNAL_URL"),
		ExternalModel:     os.Getenv("NEO_EMBED_EXTERNAL_MODEL"),
		ExternalSpace:     os.Getenv("NEO_EMBED_EXTERNAL_SPACE"),
		ExternalDimension: externalDimension,
		ExternalAPIKeyEnv: "NEO_EMBED_EXTERNAL_API_KEY",
		ExternalTimeout:   60 * time.Second,
	}
}

type Result struct {
	Vector    api.Vector `json:"vector"`
	Dimension int        `json:"dimension"`
	Space     string     `json:"space"`
	Model     string     `json:"model"`
}

type Service struct {
	config Config
	client *http.Client
	loadMu sync.Mutex
	runMu  sync.Mutex
	model  *bgeModel
}

var global struct {
	sync.Mutex
	service *Service
}

func Configure(config Config) error {
	global.Lock()
	defer global.Unlock()
	next := NewService(config)
	if global.service != nil {
		if global.service.config == next.config {
			return nil
		}
		if global.service.model != nil {
			return errors.New("cannot reconfigure a loaded embedding model")
		}
	}
	global.service = next
	return nil
}

func Default() *Service {
	global.Lock()
	defer global.Unlock()
	if global.service == nil {
		global.service = NewService(DefaultConfig())
	}
	return global.service
}

func NewService(config Config) *Service {
	defaults := DefaultConfig()
	if config.DefaultProvider == "" {
		config.DefaultProvider = defaults.DefaultProvider
	}
	if config.CacheDir == "" {
		config.CacheDir = defaults.CacheDir
	}
	if config.MaxTokens <= 0 {
		config.MaxTokens = defaults.MaxTokens
	}
	if config.ExternalAPIKeyEnv == "" {
		config.ExternalAPIKeyEnv = defaults.ExternalAPIKeyEnv
	}
	if config.ExternalTimeout <= 0 {
		config.ExternalTimeout = defaults.ExternalTimeout
	}
	return &Service{config: config, client: &http.Client{Timeout: config.ExternalTimeout}}
}

func (s *Service) Embed(ctx context.Context, text, provider string) (Result, error) {
	if strings.TrimSpace(text) == "" {
		return Result{}, errors.New("embedding input is empty")
	}
	if provider == "" {
		provider = s.config.DefaultProvider
	}
	switch provider {
	case "bge-m3":
		return s.embedBGE(ctx, text)
	case "external":
		return s.embedExternal(ctx, text)
	default:
		return Result{}, fmt.Errorf("unknown embedding provider %q", provider)
	}
}

func (s *Service) EmbedMany(ctx context.Context, texts []string, provider string) ([]Result, error) {
	if len(texts) < 1 || len(texts) > 32 {
		return nil, fmt.Errorf("embedding batch size out of range: %d", len(texts))
	}
	results := make([]Result, len(texts))
	for i, text := range texts {
		result, err := s.Embed(ctx, text, provider)
		if err != nil {
			return nil, fmt.Errorf("input %d: %w", i, err)
		}
		results[i] = result
	}
	return results, nil
}

func (s *Service) embedExternal(ctx context.Context, text string) (Result, error) {
	config := s.config
	if config.ExternalURL == "" || config.ExternalModel == "" || config.ExternalSpace == "" ||
		config.ExternalDimension < 1 || config.ExternalDimension > api.VectorMaxDimension {
		return Result{}, errors.New("external embedding provider is not configured")
	}
	requestBody, err := json.Marshal(map[string]any{"model": config.ExternalModel, "input": text})
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.ExternalURL, bytes.NewReader(requestBody))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if token := os.Getenv(config.ExternalAPIKeyEnv); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return Result{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("external embedding HTTP status %d", response.StatusCode)
	}
	var payload struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&payload); err != nil {
		return Result{}, err
	}
	if len(payload.Data) != 1 || payload.Data[0].Index != 0 || len(payload.Data[0].Embedding) != config.ExternalDimension {
		return Result{}, errors.New("external embedding response dimension or index mismatch")
	}
	vector := make(api.Vector, config.ExternalDimension)
	for i, value := range payload.Data[0].Embedding {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > math.MaxFloat32 {
			return Result{}, fmt.Errorf("external embedding element %d is not finite FLOAT32", i)
		}
		vector[i] = float32(value)
	}
	if err := vector.Validate(); err != nil {
		return Result{}, err
	}
	return Result{Vector: vector, Dimension: len(vector), Space: config.ExternalSpace, Model: config.ExternalModel}, nil
}
