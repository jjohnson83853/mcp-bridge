package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"strings"
)

// PromptPublicName derives an opaque, prompt-domain name from upstream provenance.
func PromptPublicName(serverID, rawName string) string {
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode([]string{serverID, rawName}); err != nil {
		panic(err)
	}

	blob := bytes.TrimSuffix(canonical.Bytes(), []byte("\n"))
	sum := sha256.Sum256(blob)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:])
	return "p_" + strings.ToLower(encoded)
}
