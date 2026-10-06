//go:build !linux

package embedding

import "context"

func lockAssets(_ context.Context, _ string) (func(), error) {
	return func() {}, nil
}
