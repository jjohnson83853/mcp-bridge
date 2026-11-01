package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// This file owns the ticket-03 session outcome diagnostics regressions: the
// composed HTTP-to-typed-log slice distinguishing legal fresh initialization,
// missing session headers, rehydrated entries and existing-session bearer
// binding mismatches, with a privacy-safe independent diagnostic reference.

// seqSessionRefs returns an injectable deterministic session-reference
// generator (32 lowercase hex, sequence-numbered) and a call counter so tests
// can pair observations and prove independent entropy injection.
func seqSessionRefs() (func() (string, error), *int) {
	var mu sync.Mutex
	calls := 0
	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return fmt.Sprintf("%032x", calls), nil
	}, &calls
}

// sessionDiagFixture holds one composed diagnostics capture over an inbound
// edge: a controlled sink collects the raw typed record bytes.
type sessionDiagFixture struct {
	sink    *bytes.Buffer
	inbound *InboundHandler
	handler http.Handler
}

// newSessionDiagFixture composes the outer completion owner in front of the
// inbound edge exactly as production does, with injected clock and session
// reference entropy.
func newSessionDiagFixture(t *testing.T, inner http.Handler, tokens []string, newRef func() (string, error), tune func(*InboundOptions)) *sessionDiagFixture {
	t.Helper()
	sink := &bytes.Buffer{}
	diag := observe.NewHTTPDiagnostics(observe.NewLogger(sink), observe.HTTPDiagnosticsOptions{
		Now:   func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) },
		NewID: func() string { return "0123456789abcdef0123456789abcdef" },
	})
	options := InboundOptions{
		Logger:        observe.NewLogger(sink),
		Diagnostics:   diag,
		NewSessionRef: newRef,
	}
	if len(tokens) > 0 {
		options.Tokens = func() [][32]byte { return tokenHashForAll(tokens) }
	}
	if tune != nil {
		tune(&options)
	}
	inbound := NewInboundHandler(inner, nil, options)
	return &sessionDiagFixture{
		sink:    sink,
		inbound: inbound,
		handler: diag.Wrap(inbound),
	}
}

// tokenHashForAll is the test-side fingerprint list of plaintext tokens.
func tokenHashForAll(tokens []string) [][32]byte {
	hashes := make([][32]byte, 0, len(tokens))
	for _, token := range tokens {
		hashes = append(hashes, sha256.Sum256([]byte(token)))
	}
	return hashes
}

// typedRecords decodes every http_complete record from the raw sink bytes.
func typedRecords(t *testing.T, sink *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(sink.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode record %q: %v", line, err)
		}
		if record["event"] == "http_complete" {
			records = append(records, record)
		}
	}
	return records
}

func recString(record map[string]any, key string) string {
	value, _ := record[key].(string)
	return value
}

func recHas(record map[string]any, key string) bool {
	_, ok := record[key]
	return ok
}

// validDiagnosticRef reports whether the value is a 32 lowercase hex
// diagnostic reference, the only session-reference shape the schema allows.
func validDiagnosticRef(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// sessionPost performs one composed POST against the fixture.
func sessionPost(t *testing.T, handler http.Handler, body, token, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

const sessionDiagInitialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"wire-client","version":"1"}}}`
const sessionDiagControl = `{"jsonrpc":"2.0","id":2,"method":"notifications/initialized"}`

func TestUnsupportedMethodDoesNotRehydrateUnknownSession(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			inbound := NewInboundHandler(
				NewStreamableHandler(NewMCPServer()),
				tokenHashForAll([]string{"token-a"}),
			)
			request := httptest.NewRequest(method, "/mcp", nil)
			request.Header.Set("Authorization", "Bearer token-a")
			request.Header.Set(sessionIDHeader, "unknown-session")
			response := httptest.NewRecorder()

			inbound.ServeHTTP(response, request)

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status=%d, want 405", response.Code)
			}
			if len(inbound.sessions) != 0 || len(inbound.sdkSessions) != 0 {
				t.Fatalf("unsupported method allocated bridge state: sessions=%d SDK sessions=%d",
					len(inbound.sessions), len(inbound.sdkSessions))
			}
		})
	}
}

func TestSyntheticBootstrapFailureRollsBackSession(t *testing.T) {
	const sdkID = "orphan-sdk-session"
	deletes := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if r.Header.Get(sessionIDHeader) == "" {
				w.Header().Set(sessionIDHeader, sdkID)
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":"rehydrate","error":{"code":-1,"message":"initialize failed"}}`)
				return
			}
		case http.MethodDelete:
			if r.Header.Get(sessionIDHeader) == sdkID {
				deletes++
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	inbound := NewInboundHandler(next, tokenHashForAll([]string{"token-a"}))
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
	))
	request.Header.Set("Authorization", "Bearer token-a")
	request.Header.Set(sessionIDHeader, "pre-restart-session")
	response := httptest.NewRecorder()

	inbound.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", response.Code)
	}
	if len(inbound.sessions) != 0 || len(inbound.sdkSessions) != 0 {
		t.Fatalf("failed bootstrap retained bridge state: sessions=%d SDK sessions=%d",
			len(inbound.sessions), len(inbound.sdkSessions))
	}
	if deletes != 1 {
		t.Fatalf("SDK cleanup DELETE count=%d, want 1", deletes)
	}
}

func TestSessionRehydrationDistinguishedFromMismatch(t *testing.T) {
	newRef, refCalls := seqSessionRefs()
	var acceptAll http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	fixture := newSessionDiagFixture(t, &sessionIssuingHandler{next: acceptAll}, []string{"token-a", "token-b"}, newRef, nil)

	// Legal initialization without a session: created evidence.
	first := sessionPost(t, fixture.handler, sessionDiagInitialize, "token-a", "")
	if first.Code != http.StatusOK {
		t.Fatalf("initialize status=%d", first.Code)
	}
	sessionID := first.Header().Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatal("initialize omitted session ID")
	}

	// Valid credentials plus an unknown entry: it is rehydrated and bound to
	// the authenticated bearer.
	rehydrated := sessionPost(t, fixture.handler, sessionDiagControl, "token-a", "unknown-entry-id")
	if rehydrated.Code != http.StatusOK {
		t.Fatalf("unknown entry status=%d, want 200", rehydrated.Code)
	}
	if challenge := rehydrated.Header().Get("WWW-Authenticate"); challenge != "" {
		t.Fatalf("rehydrated challenge=%q, want no challenge", challenge)
	}
	if rehydrated.Header().Get(sessionIDHeader) != "" {
		t.Fatal("rehydrated response leaked a session reference header")
	}
	if rehydrated.Body.Len() != 0 {
		t.Fatalf("rehydrated notification body=%q, want empty", rehydrated.Body.String())
	}

	// Valid credentials plus an existing entry bound to a different bearer:
	// binding mismatch with the live reference in the server record only.
	mismatch := sessionPost(t, fixture.handler, sessionDiagControl, "token-b", sessionID)
	if mismatch.Code != http.StatusNotFound {
		t.Fatalf("mismatch status=%d, want 404", mismatch.Code)
	}
	if challenge := mismatch.Header().Get("WWW-Authenticate"); challenge != "" {
		t.Fatalf("mismatch challenge=%q, want no challenge", challenge)
	}
	if mismatch.Header().Get(sessionIDHeader) != "" {
		t.Fatal("mismatch response leaked a session reference header")
	}

	records := typedRecords(t, fixture.sink)
	if len(records) != 3 {
		t.Fatalf("records=%d, want 3", len(records))
	}
	created, rehydratedRec, mismatchRec := records[0], records[1], records[2]

	if recString(created, "failure_reason") != "none" || recString(created, "session_state") != "created" {
		t.Fatalf("created record reason=%q state=%q", recString(created, "failure_reason"), recString(created, "session_state"))
	}
	if recString(created, "session_ref_state") != "available" || !validDiagnosticRef(recString(created, "session_ref")) {
		t.Fatalf("created ref state=%q ref=%q", recString(created, "session_ref_state"), recString(created, "session_ref"))
	}
	createdRef := recString(created, "session_ref")
	rehydratedRef := recString(rehydratedRec, "session_ref")

	if recString(rehydratedRec, "failure_stage") != "none" || recString(rehydratedRec, "failure_reason") != "none" {
		t.Fatalf("rehydrated record failure=%s/%s", recString(rehydratedRec, "failure_stage"), recString(rehydratedRec, "failure_reason"))
	}
	if recString(rehydratedRec, "session_state") != "rehydrated" || recString(rehydratedRec, "binding_relation") != "not_evaluated" {
		t.Fatalf("rehydrated record state=%q binding=%q", recString(rehydratedRec, "session_state"), recString(rehydratedRec, "binding_relation"))
	}
	if recString(rehydratedRec, "session_ref_state") != "available" || !validDiagnosticRef(rehydratedRef) || rehydratedRef == createdRef {
		t.Fatalf("rehydrated ref state=%q ref=%q", recString(rehydratedRec, "session_ref_state"), rehydratedRef)
	}

	if recString(mismatchRec, "failure_stage") != "session" || recString(mismatchRec, "failure_reason") != "session_fingerprint_mismatch" {
		t.Fatalf("mismatch record failure=%s/%s", recString(mismatchRec, "failure_stage"), recString(mismatchRec, "failure_reason"))
	}
	if recString(mismatchRec, "session_state") != "existing" || recString(mismatchRec, "binding_relation") != "mismatch" {
		t.Fatalf("mismatch record state=%q binding=%q", recString(mismatchRec, "session_state"), recString(mismatchRec, "binding_relation"))
	}
	if recString(mismatchRec, "session_ref") != createdRef {
		t.Fatalf("mismatch record ref=%q, want the retained live reference %q", recString(mismatchRec, "session_ref"), createdRef)
	}

	// The live reference never leaves the server record: no response carries it.
	for name, response := range map[string]*httptest.ResponseRecorder{"mismatch": mismatch, "rehydrated": rehydrated, "created": first} {
		for key, values := range response.Header() {
			for _, value := range values {
				if strings.Contains(value, createdRef) || strings.Contains(value, rehydratedRef) {
					t.Fatalf("%s response header %q leaked session reference", name, key)
				}
			}
		}
		if strings.Contains(response.Body.String(), createdRef) || strings.Contains(response.Body.String(), rehydratedRef) {
			t.Fatalf("%s response body leaked session reference", name)
		}
	}
	if *refCalls != 2 {
		t.Fatalf("session reference drawn %d times, want once for creation and once for rehydration", *refCalls)
	}
}

func TestSessionRefLifecycleStabilityAndDeletion(t *testing.T) {
	newRef, _ := seqSessionRefs()
	acceptAll := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	fixture := newSessionDiagFixture(t, &sessionIssuingHandler{next: acceptAll}, []string{"token-a"}, newRef, nil)

	first := sessionPost(t, fixture.handler, sessionDiagInitialize, "token-a", "")
	sessionID := first.Header().Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatal("initialize omitted session ID")
	}

	// Repeated valid use keeps the same reference across record replacement.
	for i := range 3 {
		response := sessionPost(t, fixture.handler, sessionDiagControl, "token-a", sessionID)
		if response.Code != http.StatusNoContent {
			t.Fatalf("follow-up %d status=%d", i, response.Code)
		}
	}
	// DELETE removes the session together with its reference.
	del := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	del.Header.Set("Authorization", "Bearer token-a")
	del.Header.Set(sessionIDHeader, sessionID)
	delRecorder := httptest.NewRecorder()
	fixture.handler.ServeHTTP(delRecorder, del)
	if delRecorder.Code >= http.StatusBadRequest {
		t.Fatalf("delete status=%d", delRecorder.Code)
	}
	// A recreated session draws a fresh independent reference.
	sessionPost(t, fixture.handler, sessionDiagInitialize, "token-a", "")

	records := typedRecords(t, fixture.sink)
	createdRef := recString(records[0], "session_ref")
	for i, record := range records[1:4] {
		if got := recString(record, "session_ref"); got != createdRef {
			t.Fatalf("record %d ref=%q, want stable %q", i+1, got, createdRef)
		}
		if recString(record, "session_state") != "existing" || recString(record, "binding_relation") != "match" {
			t.Fatalf("record %d state=%q binding=%q, want existing/match",
				i+1, recString(record, "session_state"), recString(record, "binding_relation"))
		}
	}
	deleteRecord := records[4]
	if recString(deleteRecord, "session_state") != "existing" || recString(deleteRecord, "binding_relation") != "match" {
		t.Fatalf("delete record state=%q binding=%q", recString(deleteRecord, "session_state"), recString(deleteRecord, "binding_relation"))
	}
	if recString(deleteRecord, "session_ref") != createdRef {
		t.Fatalf("delete record ref=%q, want %q", recString(deleteRecord, "session_ref"), createdRef)
	}
	recreatedRef := recString(records[5], "session_ref")
	if recString(records[5], "session_state") != "created" || recreatedRef == createdRef {
		t.Fatalf("recreated record state=%q ref=%q must be created with an independent reference",
			recString(records[5], "session_state"), recreatedRef)
	}
}

func TestControlledClockReapingRehydratesSession(t *testing.T) {
	newRef, _ := seqSessionRefs()
	acceptAll := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := base
	fixture := newSessionDiagFixture(t, &sessionIssuingHandler{next: acceptAll}, []string{"token-a"}, newRef, func(o *InboundOptions) {
		o.SessionIdle = time.Minute
		o.Now = func() time.Time { return clock }
	})

	first := sessionPost(t, fixture.handler, sessionDiagInitialize, "token-a", "")
	sessionID := first.Header().Get(sessionIDHeader)
	clock = base.Add(2 * time.Minute)
	reaped := sessionPost(t, fixture.handler, sessionDiagControl, "token-a", sessionID)
	if reaped.Code != http.StatusOK {
		t.Fatalf("reaped session status=%d, want 200 after rehydration", reaped.Code)
	}
	if challenge := reaped.Header().Get("WWW-Authenticate"); challenge != "" {
		t.Fatalf("reaped challenge=%q, want no challenge", challenge)
	}

	records := typedRecords(t, fixture.sink)
	reapRecord := records[len(records)-1]
	if recString(reapRecord, "failure_stage") != "none" || recString(reapRecord, "failure_reason") != "none" {
		t.Fatalf("reaped record failure=%s/%s", recString(reapRecord, "failure_stage"), recString(reapRecord, "failure_reason"))
	}
	if recString(reapRecord, "session_state") != "rehydrated" || recString(reapRecord, "binding_relation") != "not_evaluated" {
		t.Fatalf("reaped record state=%q binding=%q", recString(reapRecord, "session_state"), recString(reapRecord, "binding_relation"))
	}
	if recString(reapRecord, "session_ref_state") != "available" || !validDiagnosticRef(recString(reapRecord, "session_ref")) {
		t.Fatalf("reaped session reference state=%q ref=%q", recString(reapRecord, "session_ref_state"), recString(reapRecord, "session_ref"))
	}
}

func TestSessionRefEntropyFailureExplicitUnavailable(t *testing.T) {
	calls := 0
	newRef := func() (string, error) {
		calls++
		return "", fmt.Errorf("injected entropy failure")
	}
	acceptAll := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	fixture := newSessionDiagFixture(t, &sessionIssuingHandler{next: acceptAll}, []string{"token-a"}, newRef, nil)

	first := sessionPost(t, fixture.handler, sessionDiagInitialize, "token-a", "")
	if first.Code != http.StatusNoContent {
		t.Fatalf("initialize status=%d under entropy failure", first.Code)
	}
	sessionID := first.Header().Get(sessionIDHeader)
	followUp := sessionPost(t, fixture.handler, sessionDiagControl, "token-a", sessionID)
	if followUp.Code != http.StatusNoContent {
		t.Fatalf("follow-up status=%d under entropy failure", followUp.Code)
	}

	records := typedRecords(t, fixture.sink)
	if len(records) != 2 {
		t.Fatalf("records=%d, want 2", len(records))
	}
	for i, record := range records {
		if recString(record, "session_ref_state") != "unavailable" {
			t.Fatalf("record %d session_ref_state=%q, want explicit unavailable", i, recString(record, "session_ref_state"))
		}
		if recHas(record, "session_ref") {
			t.Fatalf("record %d carried a reference despite entropy failure", i)
		}
	}
	if calls != 1 {
		t.Fatalf("entropy drawn %d times, want one non-retried attempt", calls)
	}
	if _, ok := first.Header()["Mcp-Session-Id"]; !ok {
		t.Fatal("entropy failure changed session issuance")
	}
}

func TestMissingSessionHeaderIsNeverARejection(t *testing.T) {
	newRef, _ := seqSessionRefs()
	// A stub inner handler that does not issue a session: headerless traffic
	// proceeds without a supplied session.
	fixture := newSessionDiagFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), []string{"token-a"}, newRef, nil)

	response := sessionPost(t, fixture.handler, sessionDiagInitialize, "token-a", "")
	if response.Code != http.StatusNoContent {
		t.Fatalf("headerless initialize status=%d", response.Code)
	}
	records := typedRecords(t, fixture.sink)
	if len(records) != 1 {
		t.Fatalf("records=%d, want 1", len(records))
	}
	record := records[0]
	if recString(record, "failure_reason") != "none" || recString(record, "failure_stage") != "none" {
		t.Fatalf("headerless record failure=%s/%s", recString(record, "failure_stage"), recString(record, "failure_reason"))
	}
	if recString(record, "session_state") != "not_supplied" || recString(record, "binding_relation") != "not_evaluated" {
		t.Fatalf("headerless record state=%q binding=%q", recString(record, "session_state"), recString(record, "binding_relation"))
	}
	if recString(record, "session_header") != "absent" {
		t.Fatalf("headerless record session_header=%q", recString(record, "session_header"))
	}
	// Missing-header reasons appear only on actual session-required branches
	// with an authoritative cause; none is established on this path.
	if strings.Contains(fixture.sink.String(), "session_header_missing") {
		t.Fatal("session_header_missing emitted without an authoritative protocol cause")
	}
}

func TestConcurrentProfilesStayIsolated(t *testing.T) {
	newRef, _ := seqSessionRefs()
	var issueMu sync.Mutex
	issued := 0
	issuer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issueMu.Lock()
		defer issueMu.Unlock()
		if r.Method == http.MethodPost && r.Header.Get(sessionIDHeader) == "" {
			issued++
			w.Header().Set(sessionIDHeader, fmt.Sprintf("session-%d", issued))
		}
		w.WriteHeader(http.StatusNoContent)
	})
	fixture := newSessionDiagFixture(t, issuer,
		[]string{"token-p", "token-q", "token-one", "token-two"}, newRef, nil)

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			token := []string{"token-p", "token-q"}[i%2]
			first := sessionPost(t, fixture.handler, sessionDiagInitialize, token, "")
			if first.Code != http.StatusNoContent {
				errs <- fmt.Errorf("worker %d initialize status=%d", i, first.Code)
				return
			}
			sessionID := first.Header().Get(sessionIDHeader)
			match := sessionPost(t, fixture.handler, sessionDiagControl, token, sessionID)
			if match.Code != http.StatusNoContent {
				errs <- fmt.Errorf("worker %d match status=%d", i, match.Code)
				return
			}
			foreign := sessionPost(t, fixture.handler, sessionDiagControl, "token-q", sessionID)
			if token == "token-q" {
				foreign = sessionPost(t, fixture.handler, sessionDiagControl, "token-p", sessionID)
			}
			if foreign.Code != http.StatusNotFound {
				errs <- fmt.Errorf("worker %d foreign status=%d", i, foreign.Code)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	records := typedRecords(t, fixture.sink)
	refs := map[string]bool{}
	for _, record := range records {
		state := recString(record, "session_state")
		if state == "created" || state == "existing" {
			ref := recString(record, "session_ref")
			if !validDiagnosticRef(ref) {
				t.Fatalf("record state=%q carries invalid ref %q", state, ref)
			}
			refs[ref] = true
		}
	}
	// Concurrent workers created independent sessions; every live reference is
	// distinct and no profile inherited another's session.
	if len(refs) != workers {
		t.Fatalf("distinct refs=%d, want %d", len(refs), workers)
	}
}

func TestStaticTokensCannotInheritAnotherSession(t *testing.T) {
	newRef, _ := seqSessionRefs()
	acceptAll := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	fixture := newSessionDiagFixture(t, &sessionIssuingHandler{next: acceptAll}, []string{"token-one", "token-two"}, newRef, nil)

	first := sessionPost(t, fixture.handler, sessionDiagInitialize, "token-one", "")
	if first.Code != http.StatusNoContent {
		t.Fatalf("token-one initialize status=%d", first.Code)
	}
	sessionID := first.Header().Get(sessionIDHeader)

	stolen := sessionPost(t, fixture.handler, sessionDiagControl, "token-two", sessionID)
	if stolen.Code != http.StatusNotFound {
		t.Fatalf("token-two with token-one session status=%d, want 404", stolen.Code)
	}
	fresh := sessionPost(t, fixture.handler, sessionDiagInitialize, "token-two", "")
	if fresh.Code != http.StatusNoContent {
		t.Fatalf("token-two fresh initialize status=%d", fresh.Code)
	}

	records := typedRecords(t, fixture.sink)
	if recString(records[1], "failure_reason") != "session_fingerprint_mismatch" {
		t.Fatalf("static inheritance record reason=%q", recString(records[1], "failure_reason"))
	}
	if recString(records[2], "session_state") != "created" {
		t.Fatalf("token-two fresh record state=%q", recString(records[2], "session_state"))
	}
}

// The composed OAuth slice: synthetic A/B bearer identities over an ephemeral
// local JWKS. Different valid principals share the test-defined authorization
// context, so B is accepted by OAuth but rejected for the retained binding.
func TestComposedOAuthSyntheticABSessionDiagnostics(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pub, err := jwk.FromRaw(&key.PublicKey)
	if err != nil {
		t.Fatalf("public jwk: %v", err)
	}
	if err := pub.Set(jwk.KeyIDKey, "test-kid"); err != nil {
		t.Fatalf("set kid: %v", err)
	}
	if err := pub.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("set alg: %v", err)
	}
	set := jwk.NewSet()
	if err := set.AddKey(pub); err != nil {
		t.Fatalf("add key: %v", err)
	}

	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	signToken := func(sub, scope string, issued, expiry time.Time) string {
		t.Helper()
		token := jwt.New()
		if err := token.Set(jwt.SubjectKey, sub); err != nil {
			t.Fatalf("sub: %v", err)
		}
		if err := token.Set(jwt.IssuerKey, "https://issuer.test"); err != nil {
			t.Fatalf("iss: %v", err)
		}
		if err := token.Set(jwt.AudienceKey, []string{"https://resource.test"}); err != nil {
			t.Fatalf("aud: %v", err)
		}
		if err := token.Set(jwt.IssuedAtKey, issued); err != nil {
			t.Fatalf("iat: %v", err)
		}
		if err := token.Set(jwt.ExpirationKey, expiry); err != nil {
			t.Fatalf("exp: %v", err)
		}
		if err := token.Set("scope", scope); err != nil {
			t.Fatalf("scope: %v", err)
		}
		signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, key))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return string(signed)
	}

	newRef, _ := seqSessionRefs()
	sink := &bytes.Buffer{}
	diag := observe.NewHTTPDiagnostics(observe.NewLogger(sink), observe.HTTPDiagnosticsOptions{
		Now:   func() time.Time { return base },
		NewID: func() string { return "0123456789abcdef0123456789abcdef" },
	})
	inbound := NewInboundHandler(NewStreamableHandler(NewMCPServer()), nil, InboundOptions{
		Logger:        observe.NewLogger(sink),
		Diagnostics:   diag,
		NewSessionRef: newRef,
	})
	authConfig := auth.Config{
		PocketIDIssuer:   "https://issuer.test",
		MCPResourceURL:   "https://resource.test",
		MCPRequiredScope: "mcp:read",
	}
	authMiddleware := auth.NewAuthMiddleware(authConfig, auth.StaticKeySet{Set: set}, auth.WithNow(func() time.Time { return base }))
	server := httptest.NewServer(diag.Wrap(authMiddleware(inbound)))
	defer server.Close()

	tokenA := signToken("alice", "mcp:read", base, base.Add(time.Hour))
	tokenB := signToken("bob", "mcp:read", base, base.Add(time.Hour))
	tokenExpiredA := signToken("alice", "mcp:read", base.Add(-2*time.Hour), base.Add(-time.Hour))
	tokenNoScope := signToken("alice", "other:scope", base, base.Add(time.Hour))
	issued := []string{tokenA, tokenB, tokenExpiredA, tokenNoScope}

	do := func(t *testing.T, method, body, token, sessionID string) *http.Response {
		t.Helper()
		var payload io.Reader
		if body != "" {
			payload = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, server.URL+"/mcp", payload)
		if err != nil {
			t.Fatal(err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = response.Body.Close() })
		_, _ = io.Copy(io.Discard, response.Body)
		return response
	}

	// A initializes and performs read-only control operations.
	response := do(t, http.MethodPost, sessionDiagInitialize, tokenA, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("A initialize status=%d", response.StatusCode)
	}
	sessionA := response.Header.Get("Mcp-Session-Id")
	if sessionA == "" {
		t.Fatal("A initialize omitted session ID")
	}
	if status := do(t, http.MethodPost, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, tokenA, sessionA).StatusCode; status != http.StatusAccepted {
		t.Fatalf("A initialized notification status=%d", status)
	}
	if status := do(t, http.MethodPost, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, tokenA, sessionA).StatusCode; status != http.StatusOK {
		t.Fatalf("A tools/list status=%d", status)
	}

	// Different valid principal B on A's retained session: OAuth accepts, the
	// existing-session bearer binding rejects.
	bMismatch := do(t, http.MethodPost, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`, tokenB, sessionA)
	if bMismatch.StatusCode != http.StatusNotFound {
		t.Fatalf("B on retained A session status=%d, want 404", bMismatch.StatusCode)
	}
	if challenge := bMismatch.Header.Get("WWW-Authenticate"); challenge != "" {
		t.Fatalf("B mismatch challenge=%q, want no challenge", challenge)
	}
	if bMismatch.Header.Get("Mcp-Session-Id") != "" {
		t.Fatal("mismatch response leaked a session reference header")
	}

	// A still works: the retained session was not disturbed by B's rejection.
	if status := do(t, http.MethodPost, `{"jsonrpc":"2.0","id":4,"method":"tools/list"}`, tokenA, sessionA).StatusCode; status != http.StatusOK {
		t.Fatalf("A post-mismatch tools/list status=%d", status)
	}

	// B's fresh initialization without the stale session succeeds.
	bFresh := do(t, http.MethodPost, sessionDiagInitialize, tokenB, "")
	if bFresh.StatusCode != http.StatusOK {
		t.Fatalf("B fresh initialize status=%d", bFresh.StatusCode)
	}
	sessionB := bFresh.Header.Get("Mcp-Session-Id")
	if sessionB == "" || sessionB == sessionA {
		t.Fatal("B fresh initialization did not create an independent session")
	}

	// Expired credentials remain enforced even though the session exists.
	if status := do(t, http.MethodPost, `{"jsonrpc":"2.0","id":5,"method":"tools/list"}`, tokenExpiredA, sessionA).StatusCode; status != http.StatusUnauthorized {
		t.Fatalf("expired token on live session status=%d, want 401", status)
	}
	// Insufficient scope remains enforced even though the session exists.
	if status := do(t, http.MethodPost, `{"jsonrpc":"2.0","id":6,"method":"tools/list"}`, tokenNoScope, sessionA).StatusCode; status != http.StatusForbidden {
		t.Fatalf("insufficient scope on live session status=%d, want 403", status)
	}

	records := typedRecords(t, sink)
	byReason := func(reason string) map[string]any {
		for _, record := range records {
			if recString(record, "failure_reason") == reason {
				return record
			}
		}
		return nil
	}

	createdA := byReason("none")
	if createdA == nil || recString(createdA, "session_state") != "created" {
		t.Fatalf("A created record missing: %+v", createdA)
	}
	refA := recString(createdA, "session_ref")
	if !validDiagnosticRef(refA) {
		t.Fatalf("A ref %q invalid", refA)
	}
	mismatch := byReason("session_fingerprint_mismatch")
	if mismatch == nil || recString(mismatch, "session_state") != "existing" || recString(mismatch, "binding_relation") != "mismatch" {
		t.Fatalf("mismatch record wrong: %+v", mismatch)
	}
	if recString(mismatch, "session_ref") != refA {
		t.Fatalf("mismatch ref=%q, want retained %q", recString(mismatch, "session_ref"), refA)
	}
	// Find B's created record (the second created record).
	createdCount := 0
	var refB string
	for _, record := range records {
		if recString(record, "session_state") == "created" {
			createdCount++
			refB = recString(record, "session_ref")
		}
	}
	if createdCount != 2 || refB == "" || refB == refA {
		t.Fatalf("B fresh session records created=%d refB=%q", createdCount, refB)
	}
	expired := byReason("jwt_invalid_other")
	if expired == nil || recString(expired, "failure_stage") != "outer_oauth" || recString(expired, "session_state") != "not_evaluated" {
		t.Fatalf("expired record wrong: %+v", expired)
	}
	scope := byReason("scope_insufficient")
	if scope == nil || scope["http_status"].(float64) != http.StatusForbidden || recString(scope, "session_state") != "not_evaluated" {
		t.Fatalf("scope record wrong: %+v", scope)
	}

	// Complete raw emitted bytes: no bearer token, no session ID value, no
	// claim identity may appear anywhere in the log lane.
	raw := sink.String()
	for _, secret := range append(issued, sessionA, sessionB, "alice", "bob", "token-") {
		if strings.Contains(raw, secret) {
			t.Fatalf("raw records leaked sensitive material of length %d", len(secret))
		}
	}
	for _, record := range records {
		if ref := recString(record, "session_ref"); ref != "" && !validDiagnosticRef(ref) {
			t.Fatalf("session_ref %q is not the allowlisted pseudonym shape", ref)
		}
	}
}
