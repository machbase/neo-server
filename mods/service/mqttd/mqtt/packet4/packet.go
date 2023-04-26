package packet4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/mqtterr"
)

type ControlPacket interface {
	Write(io.Writer) (int64, error)
	Unpack(io.Reader) error
	String() string
	Details() Details
}

var PacketNames = map[uint8]string{
	1:  "CONNECT",
	2:  "CONNACK",
	3:  "PUBLISH",
	4:  "PUBACK",
	5:  "PUBREC",
	6:  "PUBREL",
	7:  "PUBCOMP",
	8:  "SUBSCRIBE",
	9:  "SUBACK",
	10: "UNSUBSCRIBE",
	11: "UNSUBACK",
	12: "PINGREQ",
	13: "PINGRESP",
	14: "DISCONNECT",
}

const (
	_           byte = iota
	CONNECT          // 1
	CONNACK          // 2
	PUBLISH          // 3
	PUBACK           // 4
	PUBREC           // 5
	PUBREL           // 6
	PUBCOMP          // 7
	SUBSCRIBE        // 8
	SUBACK           // 9
	UNSUBSCRIBE      // 10
	UNSUBACK         // 11
	PINGREQ          // 12
	PINGRESP         // 13
	DISCONNECT       // 14
)

// error codes returned by Connect()
const (
	Accepted                        = 0x00
	ErrRefusedBadProtocolVersion    = 0x01
	ErrRefusedIDRejected            = 0x02
	ErrRefusedServerUnavailable     = 0x03
	ErrRefusedBadUsernameOrPassword = 0x04
	ErrRefusedNotAuthorized         = 0x05
	ErrNetworkError                 = 0xFE
	ErrProtocolViolation            = 0xFF
)

var ConnackReturnCodes = map[uint8]string{
	0:   "Connection Accepted",
	1:   "Connection Refused: Bad Protocol Version",
	2:   "Connection Refused: Client Identifier Rejected",
	3:   "Connection Refused: Server Unavailable",
	4:   "Connection Refused: Username or Password in unknown format",
	5:   "Connection Refused: Not Authorized",
	254: "Connection Error",
	255: "Connection Refused: Protocol Violation",
}

var (
	ErrorRefusedBadProtocolVersion    = errors.New("unacceptable protocol version")
	ErrorRefusedIDRejected            = errors.New("identifier rejected")
	ErrorRefusedServerUnavailable     = errors.New("server Unavailable")
	ErrorRefusedBadUsernameOrPassword = errors.New("bad user name or password")
	ErrorRefusedNotAuthorized         = errors.New("not Authorized")
	ErrorNetworkError                 = errors.New("network Error")
	ErrorProtocolViolation            = errors.New("protocol Violation")
)

var ConnErrors = map[byte]error{
	Accepted:                        nil,
	ErrRefusedBadProtocolVersion:    ErrorRefusedBadProtocolVersion,
	ErrRefusedIDRejected:            ErrorRefusedIDRejected,
	ErrRefusedServerUnavailable:     ErrorRefusedServerUnavailable,
	ErrRefusedBadUsernameOrPassword: ErrorRefusedBadUsernameOrPassword,
	ErrRefusedNotAuthorized:         ErrorRefusedNotAuthorized,
	ErrNetworkError:                 ErrorNetworkError,
	ErrProtocolViolation:            ErrorProtocolViolation,
}

func ReadPacket(r io.Reader, maxMessageSizeLimit int) (ControlPacket, int, error) {
	var fh FixedHeader
	b := make([]byte, 1)

	nbytes, err := io.ReadFull(r, b)
	if err != nil {
		return nil, nbytes, err
	}

	err = fh.unpack(b[0], r)
	if err != nil {
		return nil, nbytes, err
	}

	if maxMessageSizeLimit > 0 && fh.RemainingLength > maxMessageSizeLimit {
		return nil, nbytes, mqtterr.MaxMessageSizeExceededError(fh.RemainingLength, maxMessageSizeLimit)
	}

	cp, err := NewControlPacketWithHeader(fh)
	if err != nil {
		return nil, nbytes, err
	}

	packetBytes := make([]byte, fh.RemainingLength)
	n, err := io.ReadFull(r, packetBytes)
	if err != nil {
		return nil, nbytes + n, err
	}
	if n != fh.RemainingLength {
		return nil, nbytes + n, errors.New("failed to read expected data")
	}

	err = cp.Unpack(bytes.NewBuffer(packetBytes))
	return cp, nbytes + n, err
}

func NewControlPacket(packetType byte) ControlPacket {
	switch packetType {
	case CONNECT:
		return &ConnectPacket{FixedHeader: FixedHeader{MessageType: CONNECT}}
	case CONNACK:
		return &ConnackPacket{FixedHeader: FixedHeader{MessageType: CONNACK}}
	case DISCONNECT:
		return &DisconnectPacket{FixedHeader: FixedHeader{MessageType: DISCONNECT}}
	case PUBLISH:
		return &PublishPacket{FixedHeader: FixedHeader{MessageType: PUBLISH}}
	case PUBACK:
		return &PubackPacket{FixedHeader: FixedHeader{MessageType: PUBACK}}
	case PUBREC:
		return &PubrecPacket{FixedHeader: FixedHeader{MessageType: PUBREC}}
	case PUBREL:
		return &PubrelPacket{FixedHeader: FixedHeader{MessageType: PUBREL, Qos: 1}}
	case PUBCOMP:
		return &PubcompPacket{FixedHeader: FixedHeader{MessageType: PUBCOMP}}
	case SUBSCRIBE:
		return &SubscribePacket{FixedHeader: FixedHeader{MessageType: SUBSCRIBE, Qos: 1}}
	case SUBACK:
		return &SubackPacket{FixedHeader: FixedHeader{MessageType: SUBACK}}
	case UNSUBSCRIBE:
		return &UnsubscribePacket{FixedHeader: FixedHeader{MessageType: UNSUBSCRIBE, Qos: 1}}
	case UNSUBACK:
		return &UnsubackPacket{FixedHeader: FixedHeader{MessageType: UNSUBACK}}
	case PINGREQ:
		return &PingreqPacket{FixedHeader: FixedHeader{MessageType: PINGREQ}}
	case PINGRESP:
		return &PingrespPacket{FixedHeader: FixedHeader{MessageType: PINGRESP}}
	}
	return nil
}

func NewControlPacketWithHeader(fh FixedHeader) (ControlPacket, error) {
	switch fh.MessageType {
	case CONNECT:
		return &ConnectPacket{FixedHeader: fh}, nil
	case CONNACK:
		return &ConnackPacket{FixedHeader: fh}, nil
	case DISCONNECT:
		return &DisconnectPacket{FixedHeader: fh}, nil
	case PUBLISH:
		return &PublishPacket{FixedHeader: fh}, nil
	case PUBACK:
		return &PubackPacket{FixedHeader: fh}, nil
	case PUBREC:
		return &PubrecPacket{FixedHeader: fh}, nil
	case PUBREL:
		return &PubrelPacket{FixedHeader: fh}, nil
	case PUBCOMP:
		return &PubcompPacket{FixedHeader: fh}, nil
	case SUBSCRIBE:
		return &SubscribePacket{FixedHeader: fh}, nil
	case SUBACK:
		return &SubackPacket{FixedHeader: fh}, nil
	case UNSUBSCRIBE:
		return &UnsubscribePacket{FixedHeader: fh}, nil
	case UNSUBACK:
		return &UnsubackPacket{FixedHeader: fh}, nil
	case PINGREQ:
		return &PingreqPacket{FixedHeader: fh}, nil
	case PINGRESP:
		return &PingrespPacket{FixedHeader: fh}, nil
	}
	return nil, fmt.Errorf("unsupported packet type 0x%x", fh.MessageType)
}

type Details struct {
	Qos       byte
	MessageID uint16
}

type FixedHeader struct {
	MessageType     byte
	Dup             bool
	Qos             byte
	Retain          bool
	RemainingLength int
}

func (fh FixedHeader) String() string {
	return fmt.Sprintf("%s: dup: %t qos: %d retain: %t rLength: %d",
		PacketNames[fh.MessageType], fh.Dup, fh.Qos, fh.Retain, fh.RemainingLength)
}

func boolToByte(b bool) byte {
	switch b {
	case true:
		return 1
	default:
		return 0
	}
}

func (fh *FixedHeader) pack() bytes.Buffer {
	var header bytes.Buffer
	header.WriteByte(fh.MessageType<<4 | boolToByte(fh.Dup)<<3 | fh.Qos<<1 | boolToByte(fh.Retain))
	header.Write(encodeLength(fh.RemainingLength))
	return header
}

func (fh *FixedHeader) unpack(typeAndFlags byte, r io.Reader) error {
	fh.MessageType = typeAndFlags >> 4
	fh.Dup = (typeAndFlags>>3)&0x01 > 0
	fh.Qos = (typeAndFlags >> 1) & 0x03
	fh.Retain = typeAndFlags&0x01 > 0

	var err error
	fh.RemainingLength, err = decodeLength(r)
	return err
}

func decodeByte(b io.Reader) (byte, error) {
	num := make([]byte, 1)
	_, err := b.Read(num)
	if err != nil {
		return 0, err
	}

	return num[0], nil
}

func decodeUint16(b io.Reader) (uint16, error) {
	num := make([]byte, 2)
	_, err := b.Read(num)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(num), nil
}

func encodeUint16(num uint16) []byte {
	bytesResult := make([]byte, 2)
	binary.BigEndian.PutUint16(bytesResult, num)
	return bytesResult
}

func encodeString(field string) []byte {
	return encodeBytes([]byte(field))
}

func decodeString(b io.Reader) (string, error) {
	buf, err := decodeBytes(b)
	return string(buf), err
}

func decodeBytes(b io.Reader) ([]byte, error) {
	fieldLength, err := decodeUint16(b)
	if err != nil {
		return nil, err
	}

	field := make([]byte, fieldLength)
	_, err = b.Read(field)
	if err != nil {
		return nil, err
	}

	return field, nil
}

func encodeBytes(field []byte) []byte {
	fieldLength := make([]byte, 2)
	binary.BigEndian.PutUint16(fieldLength, uint16(len(field)))
	return append(fieldLength, field...)
}

func encodeLength(length int) []byte {
	var encLength []byte
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		encLength = append(encLength, digit)
		if length == 0 {
			break
		}
	}
	return encLength
}

func decodeLength(r io.Reader) (int, error) {
	var rLength uint32
	var multiplier uint32
	b := make([]byte, 1)
	for multiplier < 27 { // fix: Infinite '(digit & 128) == 1' will cause the dead loop
		_, err := io.ReadFull(r, b)
		if err != nil {
			return 0, err
		}

		digit := b[0]
		rLength |= uint32(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}
	return int(rLength), nil
}
