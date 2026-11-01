package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/catalog"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/search"
)

const exposureEpochHeader = "X-Mcp-Bridge-Exposure-Epoch"

type progressiveRequestCapture struct {
	requests chan *http.Request
}

func (c *progressiveRequestCapture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.requests <- r
	w.WriteHeader(http.StatusNoContent)
}

func addAttackerEpochHeaders(header http.Header) {
	header[exposureEpochHeader] = []string{"17", "18"}
	header["x-mcp-bridge-exposure-epoch"] = []string{"999"}
	header["X-MCP-BRIDGE-EXPOSURE-EPOCH"] = []string{"0"}
}

func requireCanonicalEpoch(t *testing.T, header http.Header) uint64 {
	t.Helper()
	var values []string
	for key, entries := range header {
		if strings.EqualFold(key, exposureEpochHeader) {
			values = append(values, entries...)
		}
	}
	if len(values) != 1 {
		t.Fatalf("delegated epoch header values = %q, want exactly one", values)
	}
	epoch, err := strconv.ParseUint(values[0], 10, 64)
	if err != nil || epoch == 0 || strconv.FormatUint(epoch, 10) != values[0] {
		t.Fatalf("delegated epoch = %q, want canonical nonzero decimal", values[0])
	}
	return epoch
}

func TestProgressiveMetadataSessionCarrier(t *testing.T) {
	capture := &progressiveRequestCapture{requests: make(chan *http.Request, 4)}
	inbound := testInbound(capture)
	attachTestSurface(inbound, newCarrierTestSurface(t))

	initialize := requestFor(http.MethodPost, "", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	addAttackerEpochHeaders(initialize.Header)
	initialize.Header.Set("X-Request-ID", "0123456789abcdef0123456789abcdef")
	initialize.Header.Set("X-Mcp-Bridge-Internal-Request-Id", "attacker-request-id")
	initializeOriginal := initialize.Header.Clone()
	initializeResponse := serve(inbound, initialize)
	if initializeResponse.Code != http.StatusNoContent {
		t.Fatalf("initialize response = %d, want 204", initializeResponse.Code)
	}
	delegatedInitialize := <-capture.requests
	if values := valuesEqualFold(delegatedInitialize.Header, exposureEpochHeader); len(values) != 0 {
		t.Fatalf("initialize delegated attacker epoch values = %q, want none", values)
	}
	if !headersEqual(initialize.Header, initializeOriginal) {
		t.Fatalf("initialize request header mutated: got %v, want %v", initialize.Header, initializeOriginal)
	}
	if got := delegatedInitialize.Header.Get("X-Request-ID"); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("delegated X-Request-ID = %q, want caller value", got)
	}
	if got := delegatedInitialize.Header.Get("X-Mcp-Bridge-Internal-Request-Id"); got == "" || got == "attacker-request-id" {
		t.Fatalf("delegated internal request ID = %q, want server-owned value", got)
	}

	sessionID := initializeResponse.Header().Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatal("initialize did not issue a session")
	}
	recordEpoch := sessionRecordEpoch(t, inbound, sessionID)
	if recordEpoch == 0 {
		t.Fatal("ordinary response-issued session has no stable nonzero epoch")
	}

	existing := requestFor(http.MethodPost, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	addAttackerEpochHeaders(existing.Header)
	existing.Header.Set("X-Request-ID", "fedcba9876543210fedcba9876543210")
	existing.Header.Set("X-Mcp-Bridge-Internal-Request-Id", "attacker-second-id")
	existingOriginal := existing.Header.Clone()
	existingResponse := serve(inbound, existing)
	if existingResponse.Code != http.StatusNoContent {
		t.Fatalf("existing-session response = %d, want 204", existingResponse.Code)
	}
	delegatedExisting := <-capture.requests
	epoch := requireCanonicalEpoch(t, delegatedExisting.Header)
	if epoch != recordEpoch {
		t.Fatalf("delegated epoch = %d, want registered record epoch %d", epoch, recordEpoch)
	}
	if !headersEqual(existing.Header, existingOriginal) {
		t.Fatalf("existing request header mutated: got %v, want %v", existing.Header, existingOriginal)
	}
	if got := delegatedExisting.Header.Get("X-Request-ID"); got != "fedcba9876543210fedcba9876543210" {
		t.Fatalf("delegated X-Request-ID = %q, want caller value", got)
	}
	if got := delegatedExisting.Header.Get("X-Mcp-Bridge-Internal-Request-Id"); got == "" || got == "attacker-second-id" {
		t.Fatalf("delegated internal request ID = %q, want server-owned value", got)
	}
	if got := valuesEqualFold(delegatedExisting.Header, exposureEpochHeader); len(got) != 1 || got[0] != strconv.FormatUint(epoch, 10) {
		t.Fatalf("delegated epoch values = %q, want one canonical value", got)
	}

	// Neither a failed credential nor a session-binding failure may delegate a
	// request carrying an attacker-supplied epoch.
	unauthorized := requestFor(http.MethodPost, sessionID, `{}`)
	unauthorized.Header.Set("Authorization", "Bearer invalid")
	addAttackerEpochHeaders(unauthorized.Header)
	if response := serve(inbound, unauthorized); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized response = %d, want 401", response.Code)
	}
	select {
	case <-capture.requests:
		t.Fatal("unauthorized request reached delegated handler")
	default:
	}

	inbound.mu.Lock()
	record := inbound.sessions[sessionID]
	record.token = tokenHashFor("different-token")[0]
	inbound.sessions[sessionID] = record
	inbound.mu.Unlock()
	mismatched := requestFor(http.MethodPost, sessionID, `{}`)
	addAttackerEpochHeaders(mismatched.Header)
	if response := serve(inbound, mismatched); response.Code != http.StatusNotFound {
		t.Fatalf("mismatched session response = %d, want 404", response.Code)
	}
	select {
	case <-capture.requests:
		t.Fatal("mismatched session reached delegated handler")
	default:
	}
}

func TestProgressiveMetadataSessionEnrollment(t *testing.T) {
	harness := newProgressiveSessionHarness(t)
	session := harness.initialize(t)
	owner := harness.owner(t, session)
	if harness.projection(owner) != nil {
		t.Fatal("ordinary session registration eagerly created an exposure projection")
	}

	if response := harness.send(t, http.MethodPost, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil); response.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d, want 202", response.StatusCode)
	}
	if harness.projection(owner) != nil {
		t.Fatal("initialized notification eagerly enrolled an exposure projection")
	}

	const requests = 24
	var workers sync.WaitGroup
	for index := range requests {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			response := harness.send(t, http.MethodPost, session, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/list","params":{}}`, index+1), func(header http.Header) {
				addAttackerEpochHeaders(header)
			})
			if response.StatusCode != http.StatusOK {
				t.Errorf("concurrent tools/list %d status = %d, want 200", index, response.StatusCode)
			}
			names := decodeListedToolNames(t, response.body)
			wantNames := []string{catalog.ToolCallTool, catalog.ToolDescribeTool, catalog.ToolSearchTools, config.WireToolName("alpha", "old")}
			if !sameStrings(names, wantNames) {
				t.Errorf("concurrent tools/list %d names = %q, want the three bridge built-ins and current pin %q", index, names, wantNames)
			}
		}(index)
	}
	workers.Wait()

	projection := harness.projection(owner)
	if projection == nil {
		t.Fatal("concurrent first relevant requests did not lazily enroll a projection")
	}
	if len(projection.interests) != 0 {
		t.Fatalf("baseline interests = %d, want none", len(projection.interests))
	}
	if got := harness.ownerCount(session); got != 1 {
		t.Fatalf("live ordinary owners for session = %d, want one", got)
	}

	marker := logicalToolTarget{serverID: "test", rawName: "existing-interest"}
	harness.addInterest(owner, marker)
	previous := harness.projection(owner)
	response := harness.send(t, http.MethodPost, session, `{"jsonrpc":"2.0","id":99,"method":"tools/list","params":{}}`, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("subsequent tools/list status = %d, want 200", response.StatusCode)
	}
	if got := harness.projection(owner); got != previous {
		t.Fatal("repeated enrollment replaced the existing immutable projection")
	}
	if _, exists := harness.projection(owner).interests[marker]; !exists {
		t.Fatal("repeated enrollment reset an existing discovery interest")
	}

	if response := harness.send(t, http.MethodDelete, session, "", nil); response.StatusCode >= http.StatusInternalServerError {
		t.Fatalf("DELETE status = %d, want a completed SDK response", response.StatusCode)
	}
	harness.assertRetired(t, owner)
	if err := harness.surface.publishOwnerProjection(owner, harness.assembly); err == nil {
		t.Fatal("retired projected incarnation accepted delayed publication")
	}

	unprojectedSession := harness.initialize(t)
	unprojectedOwner := harness.owner(t, unprojectedSession)
	if harness.projection(unprojectedOwner) != nil {
		t.Fatal("second ordinary session was enrolled without a relevant operation")
	}
	_ = harness.send(t, http.MethodDelete, unprojectedSession, "", nil)
	harness.assertRetired(t, unprojectedOwner)
	if err := harness.surface.publishOwnerProjection(unprojectedOwner, harness.assembly); err == nil {
		t.Fatal("retired never-projected incarnation accepted delayed publication")
	}

	expiringSession := harness.initialize(t)
	expiringOwner := harness.owner(t, expiringSession)
	if harness.projection(expiringOwner) != nil {
		t.Fatal("expiring ordinary session was enrolled without a relevant operation")
	}
	*harness.clock = harness.base.Add(2 * time.Second)
	response = harness.send(t, http.MethodPost, expiringSession, `{"jsonrpc":"2.0","id":100,"method":"tools/list","params":{}}`, nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expired session tools/list status = %d, want 404", response.StatusCode)
	}
	harness.assertRetired(t, expiringOwner)
	if err := harness.surface.publishOwnerProjection(expiringOwner, harness.assembly); err == nil {
		t.Fatal("expired never-projected incarnation accepted delayed publication")
	}
}

type progressiveSessionHarness struct {
	server   *httptest.Server
	client   *http.Client
	inbound  *InboundHandler
	surface  *ToolSurface
	assembly *Assembly
	clock    *time.Time
	base     time.Time
}

func newProgressiveSessionHarness(t *testing.T) *progressiveSessionHarness {
	t.Helper()
	runtime := NewRuntime()
	assembly, err := runtime.Reload(progressiveTestConfig(t, "old"), progressiveTestDeps(&progressiveTestUpstream{}))
	if err != nil {
		t.Fatalf("admit pinned test assembly: %v", err)
	}
	surface, err := NewToolSurface(runtime, ToolSurfaceConfig{Model: search.ModelIdentity{Dimensions: 1}})
	if err != nil {
		t.Fatalf("new tool surface: %v", err)
	}
	server := NewMCPServer()
	if err := surface.Register(server); err != nil {
		t.Fatalf("register tool surface: %v", err)
	}
	if err := surface.Published(assembly); err != nil {
		t.Fatalf("publish accepted test assembly: %v", err)
	}

	base := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	clock := base
	options := InboundOptions{
		SessionIdle: time.Second,
		Now:         func() time.Time { return clock },
	}
	toolSurfaceField := reflect.ValueOf(&options).Elem().FieldByName("ToolSurface")
	if toolSurfaceField.IsValid() && toolSurfaceField.CanSet() {
		toolSurfaceField.Set(reflect.ValueOf(surface))
	}
	inbound := NewInboundHandler(NewStreamableHandler(server), tokenHashFor(sessionTestToken), options)
	httpServer := httptest.NewServer(inbound)
	t.Cleanup(httpServer.Close)
	return &progressiveSessionHarness{
		server:   httpServer,
		client:   &http.Client{Timeout: 5 * time.Second},
		inbound:  inbound,
		surface:  surface,
		assembly: assembly,
		clock:    &clock,
		base:     base,
	}
}

type progressiveHTTPResponse struct {
	*http.Response
	body []byte
}

func (h *progressiveSessionHarness) send(t *testing.T, method, sessionID, body string, mutate func(http.Header)) progressiveHTTPResponse {
	t.Helper()
	request, err := http.NewRequest(method, h.server.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("create %s request: %v", method, err)
	}
	request.Header.Set("Authorization", "Bearer "+sessionTestToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		request.Header.Set(sessionIDHeader, sessionID)
		request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
	}
	if mutate != nil {
		mutate(request.Header)
	}
	response, err := h.client.Do(request)
	if err != nil {
		t.Fatalf("%s /mcp: %v", method, err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatalf("read %s response: %v", method, err)
	}
	return progressiveHTTPResponse{Response: response, body: payload}
}

func (h *progressiveSessionHarness) initialize(t *testing.T) string {
	t.Helper()
	response := h.send(t, http.MethodPost, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"session-test","version":"1"}}}`, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200 (body=%s)", response.StatusCode, response.body)
	}
	sessionID := response.Header.Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatalf("initialize omitted %s", sessionIDHeader)
	}
	return sessionID
}

func (h *progressiveSessionHarness) owner(t *testing.T, sessionID string) projectionOwner {
	t.Helper()
	coordinator := h.surface.projectionCoordinator()
	coordinator.barrier.Lock()
	var owner projectionOwner
	for candidate := range coordinator.liveOwners {
		if candidate.kind == projectionSession && candidate.sessionID == sessionID {
			owner = candidate
			break
		}
	}
	coordinator.barrier.Unlock()
	if owner.epoch == 0 {
		t.Fatalf("registered session %q has no live nonzero projection incarnation", sessionID)
	}
	if recordEpoch := sessionRecordEpoch(t, h.inbound, sessionID); recordEpoch != owner.epoch {
		t.Fatalf("session record epoch = %d, live owner epoch = %d", recordEpoch, owner.epoch)
	}
	return owner
}

func newCarrierTestSurface(t *testing.T) *ToolSurface {
	t.Helper()
	surface, err := NewToolSurface(NewRuntime(), ToolSurfaceConfig{Model: search.ModelIdentity{Dimensions: 1}})
	if err != nil {
		t.Fatalf("new carrier test surface: %v", err)
	}
	return surface
}

func attachTestSurface(inbound *InboundHandler, surface *ToolSurface) {
	options := reflect.ValueOf(&inbound.options).Elem()
	field := options.FieldByName("ToolSurface")
	if field.IsValid() && field.CanSet() {
		field.Set(reflect.ValueOf(surface))
	}
}

func sessionRecordEpoch(t *testing.T, inbound *InboundHandler, sessionID string) uint64 {
	t.Helper()
	inbound.mu.Lock()
	record, exists := inbound.sessions[sessionID]
	inbound.mu.Unlock()
	if !exists {
		t.Fatalf("session %q is not registered", sessionID)
	}
	epochField := reflect.ValueOf(record).FieldByName("epoch")
	if !epochField.IsValid() || epochField.Kind() != reflect.Uint64 {
		return 0
	}
	return epochField.Uint()
}

func (h *progressiveSessionHarness) projection(owner projectionOwner) *toolProjection {
	coordinator := h.surface.projectionCoordinator()
	coordinator.barrier.Lock()
	defer coordinator.barrier.Unlock()
	return coordinator.projections[owner]
}

func (h *progressiveSessionHarness) ownerCount(sessionID string) int {
	coordinator := h.surface.projectionCoordinator()
	coordinator.barrier.Lock()
	defer coordinator.barrier.Unlock()
	count := 0
	for owner := range coordinator.liveOwners {
		if owner.kind == projectionSession && owner.sessionID == sessionID {
			count++
		}
	}
	return count
}

func (h *progressiveSessionHarness) addInterest(owner projectionOwner, target logicalToolTarget) {
	coordinator := h.surface.projectionCoordinator()
	coordinator.barrier.Lock()
	defer coordinator.barrier.Unlock()
	projection := coordinator.projections[owner]
	next := *projection
	next.interests = cloneInterests(projection.interests)
	next.interests[target] = struct{}{}
	coordinator.projections[owner] = &next
}

func (h *progressiveSessionHarness) assertRetired(t *testing.T, owner projectionOwner) {
	t.Helper()
	coordinator := h.surface.projectionCoordinator()
	coordinator.barrier.Lock()
	_, live := coordinator.liveOwners[owner]
	_, projected := coordinator.projections[owner]
	coordinator.barrier.Unlock()
	if live || projected {
		t.Fatalf("retired owner live=%t projected=%t, want both false", live, projected)
	}
	h.inbound.mu.Lock()
	_, registered := h.inbound.sessions[owner.sessionID]
	h.inbound.mu.Unlock()
	if registered {
		t.Fatalf("retired session %q remains in inbound admission map", owner.sessionID)
	}
}

func decodeListedToolNames(t *testing.T, body []byte) []string {
	t.Helper()
	payload := body
	if dataStart := strings.Index(string(body), "data: "); dataStart >= 0 {
		payload = body[dataStart+len("data: "):]
		if end := strings.IndexByte(string(payload), '\n'); end >= 0 {
			payload = payload[:end]
		}
	}
	var envelope struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode tools/list: %v (body=%s)", err, body)
	}
	names := make([]string, 0, len(envelope.Result.Tools))
	for _, tool := range envelope.Result.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	left = append([]string(nil), left...)
	right = append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func valuesEqualFold(header http.Header, name string) []string {
	var values []string
	for key, entries := range header {
		if strings.EqualFold(key, name) {
			values = append(values, entries...)
		}
	}
	return values
}

func headersEqual(left, right http.Header) bool {
	if len(left) != len(right) {
		return false
	}
	for key, values := range left {
		other, ok := right[key]
		if !ok || strings.Join(values, "\x00") != strings.Join(other, "\x00") {
			return false
		}
	}
	return true
}
