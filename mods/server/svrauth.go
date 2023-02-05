package server

import "crypto/x509"

type AuthHandler interface {
	Enabled() bool
	AuthId(id string, password string) (bool, error)
	AuthCert(cert *x509.Certificate) (bool, error)
}

func NewAuthenticator(serverCertFile string, authorizedKeysDir string, enabled bool) AuthHandler {
	return &authZero{
		enabled: enabled,
	}
}

type authZero struct {
	enabled bool
}

func (az *authZero) Enabled() bool {
	return az.enabled
}

func (az *authZero) AuthId(id string, password string) (bool, error) {
	return true, nil
}

func (az *authZero) AuthCert(cert *x509.Certificate) (bool, error) {
	return true, nil
}
