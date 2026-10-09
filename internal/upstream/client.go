package upstream

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ClientOptions struct {
	ConnectTimeout time.Duration
	CallTimeout    time.Duration
	Transport      *http.Transport
}

func NewClient(options ...ClientOptions) (*http.Client, error) {
	var opts ClientOptions
	if len(options) > 0 {
		opts = options[0]
	}
	transport := opts.Transport
	if transport == nil {
		transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	if opts.ConnectTimeout > 0 {
		transport.DialContext = (&net.Dialer{Timeout: opts.ConnectTimeout}).DialContext
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func CallContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

func NewMCPClientOptions() *mcp.ClientOptions {
	return &mcp.ClientOptions{MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true}}
}

func NewMCPClient(implementation *mcp.Implementation) *mcp.Client {
	return mcp.NewClient(implementation, NewMCPClientOptions())
}

func UniqueHeaders(endpoint string) http.Header {
	digest := sha256.Sum256([]byte(endpoint))
	header := make(http.Header, 1)
	header.Set("Cache-Control", hex.EncodeToString(digest[:]))
	return header
}
