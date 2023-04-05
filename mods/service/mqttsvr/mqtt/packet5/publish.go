package packet5

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
)

// Publish is the Variable Header definition for a publish control packet
type Publish struct {
	Payload    []byte
	Topic      string
	Properties *Properties
	PacketID   uint16
	QoS        byte
	Duplicate  bool
	Retain     bool
}

//Unpack is the implementation of the interface required function for a packet
func (p *Publish) Unpack(r *bytes.Buffer) error {
	var err error
	p.Topic, err = readString(r)
	if err != nil {
		return err
	}
	if p.QoS > 0 {
		p.PacketID, err = readUint16(r)
		if err != nil {
			return err
		}
	}

	err = p.Properties.Unpack(r, PUBLISH)
	if err != nil {
		return err
	}

	p.Payload, err = ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	return nil
}

// Buffers is the implementation of the interface required function for a packet
func (p *Publish) Buffers() net.Buffers {
	var b bytes.Buffer
	writeString(p.Topic, &b)
	if p.QoS > 0 {
		_ = writeUint16(p.PacketID, &b)
	}
	idvp := p.Properties.Pack(PUBLISH)
	encodeVBIdirect(len(idvp), &b)
	return net.Buffers{b.Bytes(), idvp, p.Payload}

}

// WriteTo is the implementation of the interface required function for a packet
func (p *Publish) WriteTo(w io.Writer) (int64, error) {
	f := p.QoS << 1
	if p.Duplicate {
		f |= 1 << 3
	}
	if p.Retain {
		f |= 1
	}

	cp := &ControlPacket{FixedHeader: FixedHeader{Type: PUBLISH, Flags: f}}
	cp.Content = p

	return cp.WriteTo(w)
}
