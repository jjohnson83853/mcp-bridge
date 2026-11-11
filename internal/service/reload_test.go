package service_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// reloadTwoServers and reloadAlphaOnly are the flat memberships the reload
// slice is exercised on (plan §4.10, T-08 RED "remove/add mix").
const reloadTwoServers = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: RELOAD_ALPHA_SECRET}
  - id: beta
    endpoint: https://beta.invalid/mcp
    credentials:
      bearer: {env: RELOAD_BETA_SECRET}
client_auth:
  bearer_tokens:
    - {env: RELOAD_CLIENT_TOKEN}
pins: []
`

const reloadAlphaOnly = `schema_version: 1
servers:
  - id: alpha
    endpoint: https://alpha.invalid/mcp
    credentials:
      bearer: {env: RELOAD_ALPHA_SECRET}
client_auth:
  bearer_tokens:
    - {env: RELOAD_CLIENT_TOKEN}
pins: []
`

func reloadEnv(t *testing.T) {
	t.Helper()
	t.Setenv("RELOAD_ALPHA_SECRET", "alpha-secret")
	t.Setenv("RELOAD_BETA_SECRET", "beta-secret")
	t.Setenv("RELOAD_CLIENT_TOKEN", "client-token")
}

func parseReloadConfig(t *testing.T, input string) *config.Config {
	t.Helper()
	cfg, err := config.Parse([]byte(input), ".")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return cfg
}

// fakeClient is one dedicated upstream client. It records the executions it
// served and reports its own closure so the drain contract is observable.
type fakeClient struct {
	name     string
	mu       sync.Mutex
	calls    []string
	closedCh chan struct{}
	closeOne sync.Once
}

func newFakeClient(name string) *fakeClient {
	return &fakeClient{name: name, closedCh: make(chan struct{})}
}

func (c *fakeClient) CallTool(_ context.Context, _, serverID, rawName string, _ json.RawMessage) (json.RawMessage, *core.UpstreamOutcome) {
	c.mu.Lock()
	c.calls = append(c.calls, rawName)
	c.mu.Unlock()
	return json.RawMessage(`{"server":"` + serverID + `"}`), nil
}

func (c *fakeClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

func (c *fakeClient) markClosed() {
	c.closeOne.Do(func() { close(c.closedCh) })
}

// reloadHarness records every reload side effect and injects the faults the
// T-08 RED list names.
type reloadHarness struct {
	runtime *service.Runtime

	mu            sync.Mutex
	clients       map[string][]*fakeClient
	defs          map[string][]core.ToolDefinition
	failDiscover  map[string]error
	failNewClient map[string]error
	failPersist   error
	persists      []*storage.Snapshot
	persistGen    []uint64
	published     []*service.Assembly
	closed        []*fakeClient
	closedCh      chan *fakeClient
	now           func() time.Time
}

func newReloadHarness(t *testing.T) *reloadHarness {
	t.Helper()
	return &reloadHarness{
		runtime: service.NewRuntime(),
		clients: map[string][]*fakeClient{},
		defs: map[string][]core.ToolDefinition{
			"alpha": {{RawName: "read", Description: "alpha read", InputSchema: json.RawMessage(`{"type":"object"}`)}},
			"beta":  {{RawName: "write", Description: "beta write", InputSchema: json.RawMessage(`{"type":"object"}`)}},
		},
		failDiscover:  map[string]error{},
		failNewClient: map[string]error{},
		closedCh:      make(chan *fakeClient, 8),
		now:           func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) },
	}
}

func (h *reloadHarness) deps() service.ReloadDeps {
	return service.ReloadDeps{
		Discover: func(server config.Server) ([]core.ToolDefinition, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			if err := h.failDiscover[server.ID]; err != nil {
				return nil, err
			}
			return h.defs[server.ID], nil
		},
		NewClient: func(server config.Server) (core.Upstream, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			if err := h.failNewClient[server.ID]; err != nil {
				return nil, err
			}
			client := newFakeClient(server.ID)
			h.clients[server.ID] = append(h.clients[server.ID], client)
			return client, nil
		},
		Persist: func(snapshot *storage.Snapshot) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			if err := h.failPersist; err != nil {
				return err
			}
			h.persists = append(h.persists, snapshot)
			generation := uint64(0)
			if current := h.runtime.Current(); current != nil {
				generation = current.Generation()
			}
			h.persistGen = append(h.persistGen, generation)
			return nil
		},
		Close: func(client core.Upstream) error {
			fake, ok := client.(*fakeClient)
			if !ok {
				return errors.New("unexpected client type")
			}
			h.mu.Lock()
			h.closed = append(h.closed, fake)
			h.mu.Unlock()
			fake.markClosed()
			h.closedCh <- fake
			return nil
		},
		Now:                 h.now,
		ModelIdentityDigest: strings.Repeat("a", 64),
	}
}

func (h *reloadHarness) created(serverID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[serverID])
}

func (h *reloadHarness) client(serverID string, ordinal int) *fakeClient {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.clients[serverID][ordinal]
}

func (h *reloadHarness) closedClients() []*fakeClient {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*fakeClient(nil), h.closed...)
}

func (h *reloadHarness) persisted() []*storage.Snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*storage.Snapshot(nil), h.persists...)
}

func (h *reloadHarness) publishedAssemblies() []*service.Assembly {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*service.Assembly(nil), h.published...)
}

func (h *reloadHarness) recordPublished(assembly *service.Assembly) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.published = append(h.published, assembly)
}

func (h *reloadHarness) persistGenerations() []uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]uint64(nil), h.persistGen...)
}

func (h *reloadHarness) failDiscovery(serverID string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failDiscover[serverID] = err
}

func (h *reloadHarness) setFailPersist(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failPersist = err
}

func (h *reloadHarness) setDefs(serverID string, defs ...core.ToolDefinition) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.defs[serverID] = defs
}

func containsClient(clients []*fakeClient, want *fakeClient) bool {
	for _, client := range clients {
		if client == want {
			return true
		}
	}
	return false
}

func waitClosed(t *testing.T, h *reloadHarness, client *fakeClient) {
	t.Helper()
	select {
	case <-client.closedCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("client %q was not closed after drain", client.name)
	}
}

func mustServer(t *testing.T, assembly *service.Assembly, serverID string) service.ServerRuntime {
	t.Helper()
	server, ok := assembly.Server(serverID)
	if !ok {
		t.Fatalf("assembly has no server %q", serverID)
	}
	return server
}

// TestReloadPublishesOnePointerAndReusesMatchingClients pins the happy path:
// one publication per reload, the candidate persisted before it is published,
// matching servers reusing their dedicated client, and the catalog revision
// unchanged when no definition changed.
func TestReloadPublishesOnePointerAndReusesMatchingClients(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	first := parseReloadConfig(t, reloadTwoServers)

	assembly, err := h.runtime.Reload(first, h.deps())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if h.runtime.Current() != assembly {
		t.Fatal("Reload did not publish exactly the assembly it returned")
	}
	if assembly.Generation() != 1 {
		t.Fatalf("first generation = %d; want 1", assembly.Generation())
	}
	if got := assembly.ServerIDs(); len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("ServerIDs = %v; want [alpha beta]", got)
	}
	if assembly.ConfigFingerprint() != first.Fingerprint() {
		t.Fatal("assembly does not expose the candidate's public fingerprint")
	}
	if got, want := assembly.ClientTokenHashes(), first.ClientTokenHashes(); len(got) != len(want) || got[0] != want[0] {
		t.Fatal("assembly does not expose the candidate's inbound token fingerprints")
	}
	if revision := assembly.CatalogRevision(); len(revision) != 64 {
		t.Fatalf("catalog revision %q is not canonical", revision)
	}
	if snapshot := assembly.Snapshot(); snapshot == nil {
		t.Fatal("assembly carries no persisted snapshot")
	} else if snapshot.Envelope.ConfigPublicFingerprint != first.Fingerprint() {
		t.Fatal("persisted snapshot does not carry the candidate's public fingerprint")
	}
	if h.created("alpha") != 1 || h.created("beta") != 1 {
		t.Fatalf("first reload created %d/%d clients; want 1 each", h.created("alpha"), h.created("beta"))
	}
	alphaClient := mustServer(t, assembly, "alpha").Client
	betaClient := mustServer(t, assembly, "beta").Client

	second, err := h.runtime.Reload(first, h.deps())
	if err != nil {
		t.Fatalf("second Reload: %v", err)
	}
	if second.Generation() != 2 {
		t.Fatalf("second generation = %d; want 2", second.Generation())
	}
	if mustServer(t, second, "alpha").Client != alphaClient || mustServer(t, second, "beta").Client != betaClient {
		t.Fatal("an unchanged server did not reuse its dedicated client")
	}
	if h.created("alpha") != 1 || h.created("beta") != 1 {
		t.Fatal("reload created a second client for an unchanged server")
	}
	if second.CatalogRevision() != assembly.CatalogRevision() {
		t.Fatal("an unchanged definition set moved the catalog revision")
	}
	if got := h.persistGenerations(); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("persist generations = %v; want [0 1] (persist must precede publication)", got)
	}
	if closed := h.closedClients(); len(closed) != 0 {
		t.Fatalf("a successful reload closed %d client(s)", len(closed))
	}
}

// TestFailedReloadRetainsExactWorkingAssembly is the AC-17 dedicated row: a
// syntax/secret/immutable-edit/persist fault must leave the exact old auth,
// config, catalog, server, endpoint, credentials, and working client in place,
// publish nothing, and leak no candidate client.
func TestFailedReloadRetainsExactWorkingAssembly(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, h *reloadHarness) *config.Config
	}{
		{
			name: "discovery failure",
			prepare: func(t *testing.T, h *reloadHarness) *config.Config {
				h.failDiscovery("beta", errors.New("upstream refused the connection"))
				return parseReloadConfig(t, reloadTwoServers)
			},
		},
		{
			name: "catalog collision",
			prepare: func(t *testing.T, h *reloadHarness) *config.Config {
				// Public names are server-namespaced (core.PublicName), so the
				// only reachable collision is a duplicate raw name inside one
				// server's own definition set.
				h.setDefs("beta",
					core.ToolDefinition{RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
					core.ToolDefinition{RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`)},
				)
				return parseReloadConfig(t, reloadTwoServers)
			},
		},
		{
			name: "client construction failure",
			prepare: func(t *testing.T, h *reloadHarness) *config.Config {
				t.Setenv("RELOAD_BETA_SECRET", "beta-secret-rotated")
				h.failNewClient["beta"] = errors.New("tls handshake failed")
				return parseReloadConfig(t, reloadTwoServers)
			},
		},
		{
			name: "persist failure",
			prepare: func(t *testing.T, h *reloadHarness) *config.Config {
				h.setFailPersist(errors.New("no space left on device"))
				return parseReloadConfig(t, reloadAlphaOnly)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reloadEnv(t)
			h := newReloadHarness(t)
			working, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), h.deps())
			if err != nil {
				t.Fatalf("baseline Reload: %v", err)
			}
			baselinePersists := len(h.persisted())
			alphaClient := mustServer(t, working, "alpha").Client
			betaClient := mustServer(t, working, "beta").Client
			before := map[string]int{"alpha": h.created("alpha"), "beta": h.created("beta")}

			if _, err := h.runtime.Reload(tc.prepare(t, h), h.deps()); err == nil {
				t.Fatal("failing candidate reload succeeded; want an error")
			}
			if h.runtime.Current() != working {
				t.Fatal("failed reload replaced the working assembly")
			}
			if working.Generation() != 1 || working.ConfigFingerprint() == "" {
				t.Fatal("failed reload mutated the working assembly identity")
			}
			alpha := mustServer(t, h.runtime.Current(), "alpha")
			if alpha.Client != alphaClient || alpha.Endpoint != "https://alpha.invalid/mcp" {
				t.Fatal("failed reload replaced a working server's endpoint or client")
			}
			if mustServer(t, h.runtime.Current(), "beta").Client != betaClient {
				t.Fatal("failed reload replaced beta's working client")
			}
			if len(h.persisted()) != baselinePersists {
				t.Fatal("failed reload persisted a rejected candidate")
			}
			var candidateClients []*fakeClient
			for _, serverID := range []string{"alpha", "beta"} {
				for ordinal := before[serverID]; ordinal < h.created(serverID); ordinal++ {
					candidateClients = append(candidateClients, h.client(serverID, ordinal))
				}
			}
			closed := h.closedClients()
			for _, client := range closed {
				if !containsClient(candidateClients, client) {
					t.Fatal("failed reload closed a client it did not create")
				}
			}
			for _, client := range candidateClients {
				if !containsClient(closed, client) {
					t.Fatal("failed reload leaked the candidate client it created")
				}
			}
		})
	}
}

// TestReloadRebindsEndpointIdentityChange pins the revised §4.10 step 2: a
// live server whose endpoint changed rebinds — the reload succeeds, alpha gets
// a rotated client on the new endpoint while beta keeps its exact client, the
// old alpha client is closed after drain, and the published snapshot carries
// the rebound binding fingerprint.
func TestReloadRebindsEndpointIdentityChange(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	working, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), h.deps())
	if err != nil {
		t.Fatalf("baseline Reload: %v", err)
	}
	alphaClient := h.client("alpha", 0)
	betaClient := h.client("beta", 0)

	next, err := h.runtime.Reload(parseReloadConfig(t, strings.Replace(reloadTwoServers,
		"https://alpha.invalid/mcp", "https://alpha-2.invalid/mcp", 1)), h.deps())
	if err != nil {
		t.Fatalf("endpoint-change Reload: %v", err)
	}
	if h.runtime.Current() != next || next == working {
		t.Fatal("endpoint-change reload did not publish a new working assembly")
	}
	alpha := mustServer(t, next, "alpha")
	if alpha.Endpoint != "https://alpha-2.invalid/mcp" {
		t.Fatalf("alpha endpoint = %q; want the rebound endpoint", alpha.Endpoint)
	}
	if h.created("alpha") != 2 || h.client("alpha", 1) == alphaClient {
		t.Fatal("endpoint-change reload did not rotate alpha's client")
	}
	if mustServer(t, next, "beta").Client != betaClient {
		t.Fatal("endpoint-change reload replaced beta's working client")
	}
	if !containsClient(h.closedClients(), alphaClient) {
		waitClosed(t, h, alphaClient)
	}
	if !containsClient(h.closedClients(), alphaClient) {
		t.Fatal("the replaced alpha client was never closed")
	}
	if containsClient(h.closedClients(), betaClient) {
		t.Fatal("beta's working client was closed")
	}
	if next.Generation() != working.Generation()+1 {
		t.Fatalf("generation = %d; want %d", next.Generation(), working.Generation()+1)
	}

	snapshot := next.Snapshot()
	bindings := map[string]storage.Binding{}
	for _, binding := range snapshot.Bindings {
		bindings[binding.ServerID] = binding
	}
	rebound, ok := bindings["alpha"]
	if !ok || rebound.Retired {
		t.Fatalf("alpha binding = %+v; want a live rebound binding", rebound)
	}
	if rebound.EndpointFingerprint != mustFingerprint("https://alpha-2.invalid/mcp") {
		t.Fatalf("alpha binding fingerprint = %q; want the rebound endpoint fingerprint", rebound.EndpointFingerprint)
	}
	if beta, ok := bindings["beta"]; !ok || beta.Retired || beta.EndpointFingerprint == rebound.EndpointFingerprint {
		t.Fatalf("beta binding = %+v; want an unchanged live binding", beta)
	}
}

func mustFingerprint(endpoint string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(endpoint)))
}

// TestRemovedServerClientDrainsWithoutReplay pins §4.10 step 6: a removed
// server becomes unreachable in the new generation, its client is retired but
// only closed after the lease that was already in flight is released, and that
// in-flight call still reaches exactly its original client.
func TestRemovedServerClientDrainsWithoutReplay(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	assembly, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), h.deps())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	alphaClient := mustServer(t, assembly, "alpha").Client
	betaClient := mustServer(t, assembly, "beta").Client
	betaFake := h.client("beta", 0)

	lease, ok := assembly.Acquire("beta")
	if !ok {
		t.Fatal("Acquire(beta) refused on the live generation")
	}
	if lease.Server().Client != betaClient {
		t.Fatal("lease does not pin the exact original client object")
	}
	next, err := h.runtime.Reload(parseReloadConfig(t, reloadAlphaOnly), h.deps())
	if err != nil {
		t.Fatalf("removal Reload: %v", err)
	}
	if mustServer(t, next, "alpha").Client != alphaClient {
		t.Fatal("removal reload did not reuse the retained server's client")
	}
	if _, ok := next.Server("beta"); ok {
		t.Fatal("removed server is still reachable in the new generation")
	}
	if _, ok := assembly.Acquire("beta"); ok {
		t.Fatal("a retired generation still hands out new leases")
	}
	if closed := h.closedClients(); len(closed) != 0 {
		t.Fatal("removed server's client was closed before its lease drained")
	}

	if _, outcome := lease.Server().Client.CallTool(context.Background(), "t_x", "beta", "write", json.RawMessage(`{}`)); outcome != nil {
		t.Fatal("the leased call did not reach its original client")
	}
	if betaFake.callCount() != 1 {
		t.Fatalf("original client served %d calls; want exactly 1 (no replay)", betaFake.callCount())
	}

	lease.Release()
	waitClosed(t, h, betaFake)
	lease.Release()
	if closed := h.closedClients(); len(closed) != 1 || closed[0] != betaFake {
		t.Fatalf("closed clients = %v; want exactly the removed server's client once", closed)
	}
}

// TestCredentialRotationReplacesOnlyThatClient pins §4.10 step 2: a credential
// rotation rebuilds only the affected server's dedicated client, leaks no
// secret, and does not move the catalog revision for unchanged definitions.
func TestCredentialRotationReplacesOnlyThatClient(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	working, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), h.deps())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	alphaClient := mustServer(t, working, "alpha").Client
	betaClient := mustServer(t, working, "beta").Client

	t.Setenv("RELOAD_ALPHA_SECRET", "alpha-secret-rotated")
	rotated, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), h.deps())
	if err != nil {
		t.Fatalf("rotation Reload: %v", err)
	}
	if mustServer(t, rotated, "alpha").Client == alphaClient {
		t.Fatal("rotated credentials reused the old alpha client")
	}
	if mustServer(t, rotated, "beta").Client != betaClient {
		t.Fatal("rotation replaced an unaffected server's client")
	}
	if h.created("alpha") != 2 || h.created("beta") != 1 {
		t.Fatalf("client creations alpha=%d beta=%d; want 2 and 1", h.created("alpha"), h.created("beta"))
	}
	if rotated.CatalogRevision() != working.CatalogRevision() {
		t.Fatal("credential rotation moved the catalog revision")
	}
	waitClosed(t, h, h.client("alpha", 0))
	if closed := h.closedClients(); len(closed) != 1 || closed[0] != h.client("alpha", 0) {
		t.Fatalf("closed clients = %v; want only the rotated alpha client", closed)
	}
}

// TestRuntimeAcquireReadsOnePointerAndRefusesRetiredGenerations pins the
// admission contract: nothing is admitted before the first publication, an
// admitted call is pinned to its generation's exact catalog and client, and a
// pointer to a superseded generation stops admitting as soon as it is retired.
func TestRuntimeAcquireReadsOnePointerAndRefusesRetiredGenerations(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)

	if _, ok := h.runtime.Acquire("alpha"); ok {
		t.Fatal("Acquire admitted a call before any generation was published")
	}

	working, err := h.runtime.Reload(parseReloadConfig(t, reloadTwoServers), h.deps())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	lease, ok := h.runtime.Acquire("alpha")
	if !ok {
		t.Fatal("Acquire refused a configured server of the live generation")
	}
	if lease.Server().Client != mustServer(t, working, "alpha").Client {
		t.Fatal("admitted call was not pinned to the live generation's client")
	}
	if _, ok := working.Catalog().Binding(core.PublicName("alpha", "read")); !ok {
		t.Fatal("published catalog does not resolve the configured server's tool")
	}
	if _, ok := h.runtime.Acquire("gamma"); ok {
		t.Fatal("Acquire admitted a server the membership does not contain")
	}
	lease.Release()

	next, err := h.runtime.Reload(parseReloadConfig(t, reloadAlphaOnly), h.deps())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if _, ok := working.Acquire("alpha"); ok {
		t.Fatal("a superseded generation admitted a new lease")
	}
	if _, ok := h.runtime.Acquire("beta"); ok {
		t.Fatal("Acquire admitted a server that left the membership")
	}
	carried, ok := h.runtime.Acquire("alpha")
	if !ok {
		t.Fatal("Acquire refused a carried-forward server of the live generation")
	}
	if carried.Server().Client != mustServer(t, next, "alpha").Client {
		t.Fatal("live generation did not pin the carried-forward client")
	}
	carried.Release()
}

// TestReloadRejectsUnusableDependencySets pins the fail-closed API contract:
// a reload without a candidate or without the discovery, client, or clock
// collaborators publishes nothing and leaves no client behind.
func TestReloadRejectsUnusableDependencySets(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	candidate := parseReloadConfig(t, reloadTwoServers)

	broken := map[string]func(*service.ReloadDeps){
		"discovery": func(d *service.ReloadDeps) { d.Discover = nil },
		"client":    func(d *service.ReloadDeps) { d.NewClient = nil },
		"clock":     func(d *service.ReloadDeps) { d.Now = nil },
	}
	for name, breakDep := range broken {
		t.Run("missing "+name, func(t *testing.T) {
			deps := h.deps()
			breakDep(&deps)
			if _, err := h.runtime.Reload(candidate, deps); err == nil {
				t.Fatalf("Reload accepted a dependency set without %s", name)
			}
		})
	}
	if _, err := h.runtime.Reload(nil, h.deps()); err == nil {
		t.Fatal("Reload accepted a nil candidate")
	}
	if h.runtime.Current() != nil {
		t.Fatal("a rejected reload published a generation")
	}
	if created := h.created("alpha") + h.created("beta"); created != 0 {
		t.Fatalf("rejected reloads created %d client(s)", created)
	}
	if closed := h.closedClients(); len(closed) != 0 {
		t.Fatalf("rejected reloads closed %d client(s)", len(closed))
	}
}

// Pins are resolved against the published catalog: they are exposed on the
// assembly, and a pin that the catalog does not contain rejects the whole
// candidate (no partial pin set, no publication).
func TestReloadResolvesPinsAndFailsClosedOnUnknownPin(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	h.setDefs("alpha", core.ToolDefinition{RawName: "read", InputSchema: json.RawMessage(`{"type":"object"}`)})

	pinnedYAML := strings.Replace(reloadAlphaOnly, "pins: []", "pins:\n  - {server: alpha, tool: read}", 1)
	assembly, err := h.runtime.Reload(parseReloadConfig(t, pinnedYAML), h.deps())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	pins := assembly.Pins()
	if len(pins) != 1 || pins[0].PinnedTool.ServerID != "alpha" || pins[0].PinnedTool.RawName != "read" {
		t.Fatalf("resolved pins = %+v; want exactly alpha/read", pins)
	}
	if pins[0].Binding == nil || pins[0].Binding.RawName != "read" {
		t.Fatalf("resolved pin lost its binding: %+v", pins[0])
	}
	names := assembly.PinnedNames()
	if len(names) != 1 || !names[core.PublicName("alpha", "read")] {
		t.Fatalf("PinnedNames = %v; want exactly the public name of alpha/read", names)
	}

	// The catalog no longer carries the pinned raw name: fail closed.
	h.setDefs("alpha", core.ToolDefinition{RawName: "write", InputSchema: json.RawMessage(`{"type":"object"}`)})
	before := h.runtime.Current()
	if _, err := h.runtime.Reload(parseReloadConfig(t, pinnedYAML), h.deps()); err == nil {
		t.Fatal("a pin outside the catalog was accepted")
	}
	if h.runtime.Current() != before {
		t.Fatal("a rejected pin reload published a generation")
	}
}

func TestLoadPromptsFailureEmitsWarning(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	deps := h.deps()
	deps.LoadPrompts = func() (*storage.PromptSnapshot, error) {
		return nil, errors.New("load failed")
	}
	var stdout, stderr bytes.Buffer
	deps.Logger = observe.NewLogger(&stdout, &stderr)

	if _, err := h.runtime.AdmitStartup(parseReloadConfig(t, reloadAlphaOnly), deps, nil); err != nil {
		t.Fatalf("startup admission: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("warning was written to stdout: %s", stdout.String())
	}
	assertPromptFailureWarning(t, &stderr, "prompt_load")
}

func TestPersistPromptsFailureEmitsWarningAndRestoresPreviousPrompts(t *testing.T) {
	reloadEnv(t)
	h := newReloadHarness(t)
	candidate := parseReloadConfig(t, reloadAlphaOnly)
	previous := storage.Prompt{ServerID: "alpha", RawName: "old", PublicName: "p_old", Description: "previous"}
	deps := h.deps()
	deps.LoadPrompts = func() (*storage.PromptSnapshot, error) {
		return &storage.PromptSnapshot{SchemaVersion: storage.PromptSchemaVersion, Prompts: []storage.Prompt{previous}}, nil
	}
	deps.PersistPrompts = func(*storage.PromptSnapshot) error {
		return errors.New("persist failed")
	}
	var stdout, stderr bytes.Buffer
	deps.Logger = observe.NewLogger(&stdout, &stderr)

	if _, err := h.runtime.AdmitStartup(candidate, deps, nil); err != nil {
		t.Fatalf("startup admission: %v", err)
	}
	reloaded, err := h.runtime.Reload(candidate, deps)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if prompts := reloaded.Prompts(); len(prompts) != 1 || prompts[0].ServerID != previous.ServerID || prompts[0].RawName != previous.RawName || prompts[0].PublicName != previous.PublicName || prompts[0].Description != previous.Description {
		t.Fatalf("prompts after failed persistence = %#v; want previous prompt %#v", prompts, previous)
	}
	if stdout.Len() != 0 {
		t.Fatalf("warning was written to stdout: %s", stdout.String())
	}
	assertPromptFailureWarning(t, &stderr, "prompt_persist")
}

func assertPromptFailureWarning(t *testing.T, output *bytes.Buffer, operation string) {
	t.Helper()
	if count := bytes.Count(output.Bytes(), []byte{'\n'}); count != 1 {
		t.Fatalf("warning record count = %d; want 1: %q", count, output.String())
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("decode warning record: %v: %q", err, output.String())
	}
	if record["event"] != "internal_warning" || record["operation"] != operation {
		t.Fatalf("warning event/operation = %v/%v; want internal_warning/%s", record["event"], record["operation"], operation)
	}
}
