package machnet

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const marshalHeaderSize = 16

type MarshalUnit struct {
	id     uint32
	typ    uint32
	length int
	data   []byte
}

type MarshalWriter struct {
	protocolID byte
	stmtID     uint32
	adds       uint16

	current bytes.Buffer
	bodies  [][]byte
}

func newMarshalWriter(protocolID byte, stmtID uint32, adds uint16) *MarshalWriter {
	return &MarshalWriter{protocolID: protocolID, stmtID: stmtID, adds: adds}
}

func (w *MarshalWriter) addString(id uint32, value string) {
	w.addVariable(id, cmiStringType, []byte(value))
}

func (w *MarshalWriter) addBinary(id uint32, value []byte) {
	w.addVariable(id, cmiBinaryType, value)
}

func (w *MarshalWriter) addUInt32(id uint32, value uint32) {
	var unit [marshalHeaderSize]byte
	binary.LittleEndian.PutUint32(unit[0:4], id)
	binary.LittleEndian.PutUint32(unit[4:8], cmiUIntType)
	binary.LittleEndian.PutUint32(unit[8:12], value)
	w.enqueue(unit[:])
}

func (w *MarshalWriter) addSInt32(id uint32, value int32) {
	var unit [marshalHeaderSize]byte
	binary.LittleEndian.PutUint32(unit[0:4], id)
	binary.LittleEndian.PutUint32(unit[4:8], cmiSIntType)
	binary.LittleEndian.PutUint32(unit[8:12], uint32(value))
	w.enqueue(unit[:])
}

func (w *MarshalWriter) addUInt64(id uint32, value uint64) {
	var unit [marshalHeaderSize]byte
	binary.LittleEndian.PutUint32(unit[0:4], id)
	binary.LittleEndian.PutUint32(unit[4:8], cmiULongType)
	binary.LittleEndian.PutUint64(unit[8:16], value)
	w.enqueue(unit[:])
}

func (w *MarshalWriter) addSInt64(id uint32, value int64) {
	var unit [marshalHeaderSize]byte
	binary.LittleEndian.PutUint32(unit[0:4], id)
	binary.LittleEndian.PutUint32(unit[4:8], cmiSLongType)
	binary.LittleEndian.PutUint64(unit[8:16], uint64(value))
	w.enqueue(unit[:])
}

func (w *MarshalWriter) addVariable(id uint32, typ uint32, payload []byte) {
	length := len(payload)
	padded := align8(length)
	unit := make([]byte, marshalHeaderSize+padded)
	binary.LittleEndian.PutUint32(unit[0:4], id)
	binary.LittleEndian.PutUint32(unit[4:8], typ)
	binary.LittleEndian.PutUint64(unit[8:16], uint64(length))
	copy(unit[marshalHeaderSize:], payload)
	w.enqueue(unit)
}

func (w *MarshalWriter) enqueue(unit []byte) {
	if len(unit) > cmiPacketMaxBody {
		w.flushCurrent()
		w.bodies = append(w.bodies, append([]byte(nil), unit...))
		return
	}
	if w.current.Len()+len(unit) > cmiPacketMaxBody {
		w.flushCurrent()
	}
	_, _ = w.current.Write(unit)
}

func (w *MarshalWriter) flushCurrent() {
	if w.current.Len() == 0 {
		return
	}
	w.bodies = append(w.bodies, append([]byte(nil), w.current.Bytes()...))
	w.current.Reset()
}

func (w *MarshalWriter) finalize() [][]byte {
	w.flushCurrent()
	if len(w.bodies) == 0 {
		w.bodies = [][]byte{{}}
	}
	total := len(w.bodies)
	ret := make([][]byte, 0, total)
	for idx, body := range w.bodies {
		flag := byte(0)
		if total > 1 {
			switch idx {
			case 0:
				flag = 1
			case total - 1:
				flag = 3
			default:
				flag = 2
			}
		}
		ret = append(ret, buildPacket(w.protocolID, w.stmtID, w.adds, flag, body))
	}
	return ret
}

type MarshalReader struct {
	buf []byte
	off int
}

func newMarshalReader(buf []byte) *MarshalReader {
	return &MarshalReader{buf: buf}
}

func (r *MarshalReader) next() (MarshalUnit, bool, error) {
	if r.off >= len(r.buf) {
		return MarshalUnit{}, false, nil
	}
	if r.off+marshalHeaderSize > len(r.buf) {
		return MarshalUnit{}, false, fmt.Errorf("incomplete marshal header")
	}
	id := binary.LittleEndian.Uint32(r.buf[r.off : r.off+4])
	typ := binary.LittleEndian.Uint32(r.buf[r.off+4 : r.off+8])
	unitOff := r.off + marshalHeaderSize
	switch typ {
	case cmiStringType, cmiBinaryType, cmiDateType, cmiTNumType, cmiNumType, cmiRowsType:
		length := int(binary.LittleEndian.Uint64(r.buf[r.off+8 : r.off+16]))
		padded := align8(length)
		end := unitOff + padded
		if end > len(r.buf) {
			return MarshalUnit{}, false, fmt.Errorf("marshal overflow type=%d off=%d len=%d padded=%d buf=%d", typ, r.off, length, padded, len(r.buf))
		}
		data := r.buf[unitOff : unitOff+length]
		r.off = end
		return MarshalUnit{id: id, typ: typ, length: length, data: data}, true, nil
	case cmiSCharType, cmiUCharType:
		if r.off+9 > len(r.buf) {
			return MarshalUnit{}, false, fmt.Errorf("marshal overflow type=%d off=%d need=%d buf=%d", typ, r.off, r.off+9, len(r.buf))
		}
		data := r.buf[r.off+8 : r.off+9]
		r.off += marshalHeaderSize
		return MarshalUnit{id: id, typ: typ, data: data}, true, nil
	case cmiSShortType, cmiUShortType:
		if r.off+10 > len(r.buf) {
			return MarshalUnit{}, false, fmt.Errorf("marshal overflow type=%d off=%d need=%d buf=%d", typ, r.off, r.off+10, len(r.buf))
		}
		data := r.buf[r.off+8 : r.off+10]
		r.off += marshalHeaderSize
		return MarshalUnit{id: id, typ: typ, data: data}, true, nil
	case cmiSIntType, cmiUIntType:
		if r.off+12 > len(r.buf) {
			return MarshalUnit{}, false, fmt.Errorf("marshal overflow type=%d off=%d need=%d buf=%d", typ, r.off, r.off+12, len(r.buf))
		}
		data := r.buf[r.off+8 : r.off+12]
		r.off += marshalHeaderSize
		return MarshalUnit{id: id, typ: typ, data: data}, true, nil
	default:
		if r.off+16 > len(r.buf) {
			return MarshalUnit{}, false, fmt.Errorf("marshal overflow type=%d off=%d need=%d buf=%d", typ, r.off, r.off+16, len(r.buf))
		}
		data := r.buf[r.off+8 : r.off+16]
		r.off += marshalHeaderSize
		return MarshalUnit{id: id, typ: typ, data: data}, true, nil
	}
}

func collectUnits(body []byte) (map[uint32][]MarshalUnit, error) {
	ret := map[uint32][]MarshalUnit{}
	r := newMarshalReader(body)
	for {
		u, ok, err := r.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		ret[u.id] = append(ret[u.id], u)
	}
	return ret, nil
}

func firstUnit(m map[uint32][]MarshalUnit, id uint32) (MarshalUnit, bool) {
	v := m[id]
	if len(v) == 0 {
		return MarshalUnit{}, false
	}
	return v[0], true
}
