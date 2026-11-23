package cast

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	MaxPacketSize = 64 * 1024

	ProtocolVersionCastV2 uint32 = 0
	PayloadTypeString     uint32 = 0
	PayloadTypeBinary     uint32 = 1
)

// CastMessage matches Chromium's extensions.api.cast_channel.CastMessage protobuf wire schema.
type CastMessage struct {
	ProtocolVersion uint32
	SourceID        string
	DestinationID   string
	Namespace       string
	PayloadType     uint32
	PayloadUTF8     string
	PayloadBinary   []byte
}

// MarshalCastMessage encodes CastMessage using standard protobuf wire format.
func MarshalCastMessage(m *CastMessage) ([]byte, error) {
	if m == nil {
		return nil, errors.New("cast message is nil")
	}
	if m.SourceID == "" || m.DestinationID == "" || m.Namespace == "" {
		return nil, errors.New("source_id, destination_id, and namespace are required")
	}

	buf := make([]byte, 0, 128+len(m.PayloadUTF8)+len(m.PayloadBinary))
	buf = appendVarintField(buf, 1, uint64(m.ProtocolVersion))
	buf = appendBytesField(buf, 2, []byte(m.SourceID))
	buf = appendBytesField(buf, 3, []byte(m.DestinationID))
	buf = appendBytesField(buf, 4, []byte(m.Namespace))
	buf = appendVarintField(buf, 5, uint64(m.PayloadType))
	if m.PayloadType == PayloadTypeString {
		buf = appendBytesField(buf, 6, []byte(m.PayloadUTF8))
	} else {
		buf = appendBytesField(buf, 7, m.PayloadBinary)
	}

	if len(buf) > MaxPacketSize {
		return nil, fmt.Errorf("cast packet size %d exceeds %d byte limit", len(buf), MaxPacketSize)
	}
	return buf, nil
}

// UnmarshalCastMessage decodes a protobuf-encoded CastMessage.
func UnmarshalCastMessage(data []byte) (*CastMessage, error) {
	if len(data) > MaxPacketSize {
		return nil, fmt.Errorf("cast packet size %d exceeds %d byte limit", len(data), MaxPacketSize)
	}

	msg := &CastMessage{}
	pos := 0
	for pos < len(data) {
		tag, n := binary.Uvarint(data[pos:])
		if n <= 0 {
			return nil, errors.New("invalid protobuf field tag")
		}
		pos += n

		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch wireType {
		case 0: // varint
			val, vn := binary.Uvarint(data[pos:])
			if vn <= 0 {
				return nil, fmt.Errorf("invalid varint for field %d", fieldNum)
			}
			pos += vn
			if val > math.MaxUint32 {
				return nil, fmt.Errorf("varint overflow for field %d", fieldNum)
			}
			switch fieldNum {
			case 1:
				msg.ProtocolVersion = uint32(val)
			case 5:
				msg.PayloadType = uint32(val)
			}
		case 2: // length-delimited
			length, ln := binary.Uvarint(data[pos:])
			if ln <= 0 {
				return nil, fmt.Errorf("invalid length prefix for field %d", fieldNum)
			}
			pos += ln
			if length > uint64(len(data)-pos) {
				return nil, fmt.Errorf("truncated bytes for field %d", fieldNum)
			}
			raw := data[pos : pos+int(length)]
			pos += int(length)
			switch fieldNum {
			case 2:
				msg.SourceID = string(raw)
			case 3:
				msg.DestinationID = string(raw)
			case 4:
				msg.Namespace = string(raw)
			case 6:
				msg.PayloadUTF8 = string(raw)
			case 7:
				cp := make([]byte, len(raw))
				copy(cp, raw)
				msg.PayloadBinary = cp
			}
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type %d on field %d", wireType, fieldNum)
		}
	}

	return msg, nil
}

// WriteFrame writes a 4-byte big-endian length-prefixed CastMessage to w.
func WriteFrame(w io.Writer, msg *CastMessage) error {
	payload, err := MarshalCastMessage(msg)
	if err != nil {
		return err
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

// ReadFrame reads a 4-byte big-endian length-prefixed CastMessage from r.
func ReadFrame(r io.Reader) (*CastMessage, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(hdr[:])
	if size == 0 || size > MaxPacketSize {
		return nil, fmt.Errorf("invalid cast frame length %d", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return UnmarshalCastMessage(buf)
}

func appendVarintField(dst []byte, fieldNum uint64, val uint64) []byte {
	dst = binary.AppendUvarint(dst, (fieldNum<<3)|0)
	return binary.AppendUvarint(dst, val)
}

func appendBytesField(dst []byte, fieldNum uint64, val []byte) []byte {
	dst = binary.AppendUvarint(dst, (fieldNum<<3)|2)
	dst = binary.AppendUvarint(dst, uint64(len(val)))
	return append(dst, val...)
}
