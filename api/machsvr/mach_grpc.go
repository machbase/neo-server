package machsvr

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"

	"github.com/machbase/neo-client/api"
)

type RPCServer struct {
	log          *slog.Logger
	db           api.Database
	authProvider AuthProvider
}

func NewRPCServer(db api.Database, opts ...RPCServerOption) *RPCServer {
	s := &RPCServer{
		db: db,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.log == nil {
		s.log = slog.Default()
	}
	if s.authProvider == nil {
		s.authProvider = &DefaultAuthProvider{}
	}
	return s
}

type RPCServerOption func(*RPCServer)

func WithLogger(log *slog.Logger) RPCServerOption {
	return func(s *RPCServer) {
		s.log = log
	}
}

type AuthProvider interface {
	ValidateUserOtp(user string, otp string) (bool, error)
	GenerateSnowflake() string
}

func WithAuthProvider(auth AuthProvider) RPCServerOption {
	return func(s *RPCServer) {
		s.authProvider = auth
	}
}

type DefaultAuthProvider struct {
}

var _ AuthProvider = (*DefaultAuthProvider)(nil)

func (dap *DefaultAuthProvider) ValidateUserOtp(user string, otp string) (bool, error) {
	return false, nil
}

func (dap *DefaultAuthProvider) GenerateSnowflake() string {
	r := rand.Float64()
	return fmt.Sprintf("%x", math.Float64bits(r))
}
