package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

const (
	cursorVersion1      byte = 1
	cursorDecodedLength      = 57
	cursorEncodedLength      = 76
)

func invalidCursorError() error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid cursor"}
}

func encodeCursor(secret [32]byte, epoch uint64, generation uint64, nextOffset uint64) string {
	buf := make([]byte, cursorDecodedLength)
	buf[0] = cursorVersion1
	binary.BigEndian.PutUint64(buf[1:9], epoch)
	binary.BigEndian.PutUint64(buf[9:17], generation)
	binary.BigEndian.PutUint64(buf[17:25], nextOffset)

	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write(buf[:25])
	copy(buf[25:57], mac.Sum(nil))

	return base64.RawURLEncoding.EncodeToString(buf)
}

func decodeAndValidateCursor(secret [32]byte, cursor string, expectedEpoch uint64, expectedGeneration uint64, pageSize int, totalItems int) (int, error) {
	if len(cursor) != cursorEncodedLength {
		return 0, invalidCursorError()
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(raw) != cursorDecodedLength {
		return 0, invalidCursorError()
	}
	// Canonical encoding check: re-encoding must match the exact string.
	if base64.RawURLEncoding.EncodeToString(raw) != cursor {
		return 0, invalidCursorError()
	}
	if raw[0] != cursorVersion1 {
		return 0, invalidCursorError()
	}

	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write(raw[:25])
	expectedMAC := mac.Sum(nil)
	if !hmac.Equal(raw[25:57], expectedMAC) {
		return 0, invalidCursorError()
	}

	epoch := binary.BigEndian.Uint64(raw[1:9])
	if epoch != expectedEpoch {
		return 0, invalidCursorError()
	}

	generation := binary.BigEndian.Uint64(raw[9:17])
	if generation != expectedGeneration {
		return 0, invalidCursorError()
	}

	offset := binary.BigEndian.Uint64(raw[17:25])
	if offset == 0 || offset > uint64(totalItems) || offset%uint64(pageSize) != 0 {
		return 0, invalidCursorError()
	}

	return int(offset), nil
}
