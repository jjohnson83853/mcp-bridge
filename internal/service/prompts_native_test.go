package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connectPromptClient(t *testing.T, server *mcp.Server, changed func(context.Context, *mcp.PromptListChangedRequest)) *mcp.ClientSession {
	t.Helper()
	return connectPromptClientVersion(t, server, "2025-11-25", changed)
}

func connectPromptClientVersion(t *testing.T, server *mcp.Server, version string, changed func(context.Context, *mcp.PromptListChangedRequest)) *mcp.ClientSession {
	t.Helper()
	hs := httptest.NewServer(NewStreamableHandler(server))
	t.Cleanup(hs.Close)
	options := &mcp.ClientOptions{}
	if changed != nil {
		options.PromptListChangedHandler = changed
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "prompt-test", Version: "1"}, options)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: hs.URL, MaxRetries: -1}, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestPromptNativeListGetAndChanged(t *testing.T) {
	server := NewMCPServer()
	cold := connectPromptClient(t, server, nil)
	caps := cold.InitializeResult().Capabilities
	if caps.Prompts == nil || !caps.Prompts.ListChanged {
		t.Fatalf("cold prompts capability=%#v, want listChanged", caps.Prompts)
	}
	page, err := cold.ListPrompts(context.Background(), nil)
	if err != nil || len(page.Prompts) != 0 {
		t.Fatalf("cold list = %#v, %v; want empty", page, err)
	}
	_ = cold.Close()

	assembly := &Assembly{prompts: []storage.Prompt{
		{ServerID: "zeta", RawName: "raw-z", PublicName: "p_z", Title: "Z title", Description: "Z description", Arguments: json.RawMessage(`[ {"name":"x","description":"arg"} ]`)},
		{ServerID: "alpha", RawName: "raw-b", PublicName: "p_b", Title: "B title", Description: "B description", Arguments: json.RawMessage(`[ {"name":"x","description":"arg"} ]`)},
		{ServerID: "alpha", RawName: "raw-a", PublicName: "p_a", Title: "A title", Description: "A description", Arguments: json.RawMessage(`[ {"name":"x","description":"arg"} ]`)},
	}}
	var notifications = make(chan struct{}, 4)
	server = NewMCPServer()
	surface := NewPromptSurface(NewRuntime())
	surface.Register(server)
	surface.Published(assembly)
	client := connectPromptClient(t, server, func(context.Context, *mcp.PromptListChangedRequest) { notifications <- struct{}{} })
	want := []string{"p_a", "p_b", "p_z"}
	gotPage, err := client.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(gotPage.Prompts))
	for _, prompt := range gotPage.Prompts {
		got = append(got, prompt.Name)
		if prompt.Title == "" || prompt.Description == "" || len(prompt.Arguments) == 0 {
			t.Fatalf("prompt metadata not mirrored: %#v", prompt)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prompt names=%v, want %v", got, want)
	}
	encoded, _ := json.Marshal(gotPage)
	for _, raw := range []string{"raw-z", "raw-b", "raw-a"} {
		if strings.Contains(string(encoded), raw) {
			t.Fatalf("raw name %q leaked in response %s", raw, encoded)
		}
	}
	changed := &Assembly{prompts: []storage.Prompt{assembly.prompts[0], {ServerID: "alpha", RawName: "raw-c", PublicName: "p_c", Title: "C", Arguments: json.RawMessage("[]")}}}
	surface.Published(changed)
	select {
	case <-notifications:
	case <-time.After(time.Second):
		t.Fatal("changed publication notification not delivered")
	}
	updated, err := client.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Prompts) != 2 || updated.Prompts[0].Name == "p_b" || updated.Prompts[1].Name == "p_b" {
		t.Fatalf("updated prompts=%#v", updated.Prompts)
	}
	settle := time.NewTimer(100 * time.Millisecond)
	defer settle.Stop()
	for {
		select {
		case <-notifications:
			if !settle.Stop() {
				<-settle.C
			}
			settle.Reset(100 * time.Millisecond)
		case <-settle.C:
			goto settled
		}
	}
settled:
	surface.Published(changed)
	select {
	case <-notifications:
		t.Fatal("unchanged publication emitted notification")
	case <-time.After(100 * time.Millisecond):
	}

	pagedServer := mcp.NewServer(&mcp.Implementation{Name: "paged", Version: "1"}, &mcp.ServerOptions{HasPrompts: true, PageSize: 1})
	pagedSurface := NewPromptSurface(NewRuntime())
	pagedSurface.Register(pagedServer)
	pagedSurface.Published(assembly)
	pagedClient := connectPromptClient(t, pagedServer, nil)
	cursor := ""
	seen := map[string]bool{}
	for {
		got, err := pagedClient.ListPrompts(context.Background(), &mcp.ListPromptsParams{Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Prompts) > 1 {
			t.Fatalf("page exceeded size: %d", len(got.Prompts))
		}
		for _, prompt := range got.Prompts {
			if seen[prompt.Name] {
				t.Fatalf("duplicate prompt %q", prompt.Name)
			}
			seen[prompt.Name] = true
		}
		if got.NextCursor == "" {
			break
		}
		cursor = got.NextCursor
	}
	if len(seen) != len(assembly.prompts) {
		t.Fatalf("paged prompts=%v", seen)
	}
	for _, pair := range [][2]string{{"alpha", "raw-a"}, {"zeta", "raw-z"}} {
		if core.PromptPublicName(pair[0], pair[1]) == core.PublicName(pair[0], pair[1]) {
			t.Fatalf("prompt name collides with tool name for %q/%q", pair[0], pair[1])
		}
	}

	// Supported-version evidence: every accepted legacy inbound version serves
	// prompts/list and prompts/get at its negotiated version.
	for _, version := range []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"} {
		versionRuntime := NewRuntime()
		versionUpstream := &promptTestUpstream{}
		versionRuntime.current.Store(&Assembly{servers: map[string]ServerRuntime{"alpha": {ID: "alpha", Client: versionUpstream}}})
		versionServer := NewMCPServer()
		versionSurface := NewPromptSurface(versionRuntime)
		versionSurface.Register(versionServer)
		versionSurface.Published(assembly)
		versionClient := connectPromptClientVersion(t, versionServer, version, nil)
		if negotiated := versionClient.InitializeResult().ProtocolVersion; negotiated != version {
			t.Fatalf("negotiated version=%q, want %q", negotiated, version)
		}
		listed, err := versionClient.ListPrompts(context.Background(), nil)
		if err != nil || len(listed.Prompts) != len(assembly.prompts) {
			t.Fatalf("prompts/list on %s = %#v, %v", version, listed, err)
		}
		fetched, err := versionClient.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "p_a", Arguments: map[string]string{"x": "keep"}})
		if err != nil || len(fetched.Messages) != 1 || versionUpstream.calls != 1 {
			t.Fatalf("prompts/get on %s = %#v, %v (calls=%d)", version, fetched, err, versionUpstream.calls)
		}
	}
}

type promptTestUpstream struct {
	core.Upstream
	calls int
	name  string
	args  map[string]string
	err   error
}

func (p *promptTestUpstream) GetPrompt(_ context.Context, _, raw string, args map[string]string) (*mcp.GetPromptResult, error) {
	p.calls++
	p.name = raw
	p.args = args
	if p.err != nil {
		return nil, p.err
	}
	return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: "forwarded"}}}}, nil
}

func TestPromptGetRoutingAndSafeErrors(t *testing.T) {
	server := NewMCPServer()
	runtime := NewRuntime()
	alpha := &promptTestUpstream{}
	beta := &promptTestUpstream{}
	runtime.current.Store(&Assembly{servers: map[string]ServerRuntime{"alpha": {ID: "alpha", Client: alpha}, "beta": {ID: "beta", Client: beta}}})
	assembly := &Assembly{prompts: []storage.Prompt{{ServerID: "alpha", RawName: "secret-alpha", PublicName: "p_alpha"}, {ServerID: "beta", RawName: "secret-beta", PublicName: "p_beta"}}}
	surface := NewPromptSurface(runtime)
	surface.Register(server)
	surface.Published(assembly)
	client := connectPromptClient(t, server, nil)
	args := map[string]string{"keep": "exact"}
	for _, item := range []struct {
		name, raw string
		upstream  *promptTestUpstream
	}{{"p_alpha", "secret-alpha", alpha}, {"p_beta", "secret-beta", beta}} {
		result, err := client.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: item.name, Arguments: args})
		if err != nil || len(result.Messages) != 1 || item.upstream.calls != 1 || item.upstream.name != item.raw || !reflect.DeepEqual(item.upstream.args, args) {
			t.Fatalf("get %s result=%#v err=%v upstream=%#v", item.name, result, err, item.upstream)
		}
	}
	_, err := client.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "p_unknown"})
	if err == nil || alpha.calls != 1 || beta.calls != 1 {
		t.Fatalf("unknown prompt error=%v calls=(%d,%d)", err, alpha.calls, beta.calls)
	}

	leaky := &promptTestUpstream{err: errors.New("raw-secret-name /srv/private/socket transport failure")}
	runtime.current.Store(&Assembly{servers: map[string]ServerRuntime{"gamma": {ID: "gamma", Client: leaky}}})
	surface.Published(&Assembly{prompts: []storage.Prompt{{ServerID: "gamma", RawName: "raw-secret-name", PublicName: "p_gamma"}}})
	_, err = client.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "p_gamma", Arguments: args})
	if err == nil {
		t.Fatal("failing upstream surfaced as success")
	}
	for _, secret := range []string{"raw-secret-name", "/srv/private/socket", "transport failure"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked %q: %v", secret, err)
		}
	}
	if leaky.calls != 1 {
		t.Fatalf("leaky upstream calls=%d, want exactly 1", leaky.calls)
	}
}
func TestPromptPublicationRejectsBadDefinitionsSafely(t *testing.T) {
	server := NewMCPServer()
	var logs []string
	surface := NewPromptSurface(NewRuntime(), func(line string) { logs = append(logs, line) })
	surface.Register(server)
	good := storage.Prompt{ServerID: "alpha", RawName: "raw-good", PublicName: "p_good", Title: "Good", Arguments: json.RawMessage(`[]`)}
	surface.Published(&Assembly{prompts: []storage.Prompt{good}})
	surface.Published(&Assembly{prompts: []storage.Prompt{good, {ServerID: "beta", RawName: "private", PublicName: "p_good"}}})
	client := connectPromptClient(t, server, nil)
	page, err := client.ListPrompts(context.Background(), nil)
	if err != nil || len(page.Prompts) != 1 || page.Prompts[0].Name != "p_good" {
		t.Fatalf("duplicate publication corrupted registrations: %#v, %v", page, err)
	}
	if len(logs) != 1 || strings.Contains(strings.Join(logs, " "), "private") {
		t.Fatalf("duplicate rejection logs=%q", logs)
	}
	logs = nil
	surface.Published(&Assembly{prompts: []storage.Prompt{good, {ServerID: "beta", RawName: "private", PublicName: "p_bad", Arguments: json.RawMessage(`not-json`)}}})
	page, err = client.ListPrompts(context.Background(), nil)
	if err != nil || len(page.Prompts) != 1 || page.Prompts[0].Name != "p_good" {
		t.Fatalf("invalid definition corrupted registrations: %#v, %v", page, err)
	}
	if len(logs) != 1 || strings.Contains(strings.Join(logs, " "), "private") {
		t.Fatalf("schema rejection logs=%q", logs)
	}
}
