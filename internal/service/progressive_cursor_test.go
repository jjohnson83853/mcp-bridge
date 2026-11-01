package service

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestProgressiveMetadataCursorValidation(t *testing.T) {
	const (
		epoch      = uint64(3)
		generation = uint64(8)
		pageSize   = 2
		totalItems = 5
	)
	secret := [32]byte{1, 2, 3, 4}
	valid := encodeCursor(secret, epoch, generation, 4)
	invalidVersion := mutateCursorByte(t, valid, 0, 2)
	tamperedMAC := mutateCursorByte(t, valid, 25, 0xff)
	zeroOffset := encodeCursor(secret, epoch, generation, 0)
	outOfRange := encodeCursor(secret, epoch, generation, 6)
	nonBoundary := encodeCursor(secret, epoch, generation, 3)
	staleGeneration := encodeCursor(secret, epoch+0, generation+1, 4)
	crossOwner := encodeCursor(secret, epoch+1, generation, 4)
	retiredIncarnation := encodeCursor(secret, epoch, generation, 4)
	wrongDecodedSize56 := base64.RawURLEncoding.EncodeToString(make([]byte, 56))
	wrongDecodedSize58 := base64.RawURLEncoding.EncodeToString(make([]byte, 58))
	malformedBase64 := strings.Repeat("!", cursorEncodedLength)
	noncanonical := noncanonicalCursor(t, valid)

	badSecret := secret
	badSecret[0]++
	cases := []struct {
		name       string
		cursor     string
		secret     [32]byte
		epoch      uint64
		generation uint64
	}{
		{name: "malformed base64", cursor: malformedBase64, secret: secret, epoch: epoch, generation: generation},
		{name: "noncanonical base64", cursor: noncanonical, secret: secret, epoch: epoch, generation: generation},
		{name: "short cursor", cursor: valid[:75], secret: secret, epoch: epoch, generation: generation},
		{name: "long cursor", cursor: valid + "A", secret: secret, epoch: epoch, generation: generation},
		{name: "empty cursor", cursor: "", secret: secret, epoch: epoch, generation: generation},
		{name: "56-byte decoded cursor", cursor: wrongDecodedSize56, secret: secret, epoch: epoch, generation: generation},
		{name: "58-byte decoded cursor", cursor: wrongDecodedSize58, secret: secret, epoch: epoch, generation: generation},
		{name: "wrong version", cursor: invalidVersion, secret: secret, epoch: epoch, generation: generation},
		{name: "tampered MAC", cursor: tamperedMAC, secret: secret, epoch: epoch, generation: generation},
		{name: "cursor from previous process secret", cursor: valid, secret: badSecret, epoch: epoch, generation: generation},
		{name: "zero offset", cursor: zeroOffset, secret: secret, epoch: epoch, generation: generation},
		{name: "offset past total items", cursor: outOfRange, secret: secret, epoch: epoch, generation: generation},
		{name: "offset not on page boundary", cursor: nonBoundary, secret: secret, epoch: epoch, generation: generation},
		{name: "stale generation", cursor: staleGeneration, secret: secret, epoch: epoch, generation: generation},
		{name: "same bearer used by another owner", cursor: crossOwner, secret: secret, epoch: epoch, generation: generation},
		{name: "retired or replaced incarnation", cursor: retiredIncarnation, secret: secret, epoch: epoch + 2, generation: generation},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			offset, err := decodeAndValidateCursor(test.secret, test.cursor, test.epoch, test.generation, pageSize, totalItems)
			if offset != 0 {
				t.Fatalf("invalid cursor returned partial offset %d", offset)
			}
			var rpcErr *jsonrpc.Error
			if !errors.As(err, &rpcErr) {
				t.Fatalf("error = %T %v, want *jsonrpc.Error", err, err)
			}
			if rpcErr.Code != jsonrpc.CodeInvalidParams || rpcErr.Message != "invalid cursor" {
				t.Fatalf("error = (%d, %q), want (%d, %q)", rpcErr.Code, rpcErr.Message, jsonrpc.CodeInvalidParams, "invalid cursor")
			}
		})
	}
}

func mutateCursorByte(t *testing.T, cursor string, index int, value byte) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatalf("decode test cursor: %v", err)
	}
	raw[index] = value
	return base64.RawURLEncoding.EncodeToString(raw)
}

func noncanonicalCursor(t *testing.T, cursor string) string {
	t.Helper()
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := cursor[len(cursor)-1]
	index := strings.IndexByte(alphabet, last)
	if index < 0 {
		t.Fatalf("cursor has invalid final base64 character %q", last)
	}
	alternate := alphabet[(index&^3)|((index+1)&3)]
	if alternate == last {
		t.Fatal("failed to construct noncanonical base64 cursor")
	}
	return cursor[:len(cursor)-1] + string(alternate)
}
