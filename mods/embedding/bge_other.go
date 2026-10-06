//go:build !linux || !amd64

package embedding

import (
	"context"
	"errors"
)

type bgeModel struct{}

func (s *Service) embedBGE(_ context.Context, _ string) (Result, error) {
	return Result{}, errors.New("internal BGE-M3 is supported on Linux x86-64; configure the external provider")
}
