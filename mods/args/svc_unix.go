//go:build !windows
// +build !windows

package args

type Service struct {
}

func doService(svc *Service) {
	return nil
}
