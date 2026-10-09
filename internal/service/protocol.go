package service

import (
	"errors"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const ProtocolVersionHeader = "MCP-Protocol-Version"

var protocolVersions = [...]string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

func AdvertisedProtocolVersions() []string {
	return append([]string(nil), protocolVersions[:]...)
}

func NoReplayClientOptions() *mcp.ClientOptions {
	return &mcp.ClientOptions{MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true}}
}

func NegotiateInitializeOffer(offered string) string {
	if offered == "2026-07-28" {
		return protocolVersions[0]
	}
	for _, supported := range protocolVersions {
		if supported == offered {
			return supported
		}
	}
	return ""
}

var ErrUnsupportedProtocolVersion = errors.New("unsupported protocol version")

func ValidateCallVersion(version string, headers http.Header) error {
	if version == "server/discover" {
		return nil
	}
	headerVersion := headers.Get(ProtocolVersionHeader)
	if headerVersion == "" {
		if values := headers[ProtocolVersionHeader]; len(values) > 0 {
			headerVersion = values[0]
		}
	}
	if version != headerVersion {
		return ErrUnsupportedProtocolVersion
	}
	for _, supported := range protocolVersions {
		if version == supported {
			return nil
		}
	}
	return ErrUnsupportedProtocolVersion
}
