package framing

import (
	"encoding/binary"
	"fmt"
	"io"

	pb "google.golang.org/protobuf/proto"
	castproto "github.com/zopieux/gocast/proto"
)

const maxMessageSize = 65535

// ReadMessage reads a 4-byte big-endian length-prefixed CastMessage.
func ReadMessage(r io.Reader) (*castproto.CastMessage, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf[:])
	if length > maxMessageSize {
		return nil, fmt.Errorf("message length %d exceeds maximum allowed %d", length, maxMessageSize)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	msg := &castproto.CastMessage{}
	if err := pb.Unmarshal(buf, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CastMessage: %w", err)
	}

	return msg, nil
}

// WriteMessage writes a 4-byte big-endian length-prefixed CastMessage.
func WriteMessage(w io.Writer, msg *castproto.CastMessage) error {
	data, err := pb.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal CastMessage: %w", err)
	}

	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))

	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}
