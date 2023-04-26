package packet5

import (
	"bytes"
	"io"
	"net"
)

// Auth is the Variable Header definition for a Auth control packet
type Auth struct {
	Properties *Properties
	ReasonCode byte
}

// AuthSuccess is the return code for successful authentication
const (
	AuthSuccess                = 0x00
	AuthContinueAuthentication = 0x18
	AuthReauthenticate         = 0x19
)

// Unpack is the implementation of the interface required function for a packet
func (a *Auth) Unpack(r *bytes.Buffer) error {
	var err error

	success := r.Len() == 0
	noProps := r.Len() == 1
	if !success {
		a.ReasonCode, err = r.ReadByte()
		if err != nil {
			return err
		}

		if !noProps {
			err = a.Properties.Unpack(r, AUTH)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Buffers is the implementation of the interface required function for a packet
func (a *Auth) Buffers() net.Buffers {
	idvp := a.Properties.Pack(AUTH)
	propLen := encodeVBI(len(idvp))
	n := net.Buffers{[]byte{a.ReasonCode}, propLen}
	if len(idvp) > 0 {
		n = append(n, idvp)
	}
	return n
}

// WriteTo is the implementation of the interface required function for a packet
func (a *Auth) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: AUTH}}
	cp.Content = a

	return cp.WriteTo(w)
}
