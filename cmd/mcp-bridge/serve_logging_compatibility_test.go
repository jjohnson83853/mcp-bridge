package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// The writer observes actual logger attempts, not guaranteed delivery. Notification
// follows capture, so metrics checks never use response EOF as a logging barrier.
type compatibilityLogWriter struct {
	mu           sync.Mutex
	buf          bytes.Buffer
	terminals    int
	updates      chan struct{}
	failTerminal bool
}

func newCompatibilityLogWriter(fail bool) *compatibilityLogWriter {
	return &compatibilityLogWriter{updates: make(chan struct{}, 32), failTerminal: fail}
}

func (w *compatibilityLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.buf.Write(p)
	var record map[string]any
	terminal := json.Unmarshal(p, &record) == nil && record["event"] == "http_complete"
	if terminal {
		w.terminals++
		select {
		case w.updates <- struct{}{}:
		default:
		}
		if w.failTerminal {
			return 0, errors.New("synthetic stdout delivery failure")
		}
	}
	return len(p), nil
}

func (w *compatibilityLogWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *compatibilityLogWriter) waitTerminals(t *testing.T, count int) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		w.mu.Lock()
		observed := w.terminals
		w.mu.Unlock()
		if observed >= count {
			return
		}
		select {
		case <-w.updates:
		case <-timer.C:
			t.Fatalf("actual HTTP terminal attempts = %d; waiting for %d", observed, count)
		}
	}
}

const compatibilityInitialize = `{"jsonrpc":"2.0","id":"compatibility-rpc-secret","method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"compatibility-client-secret","version":"1"}}}`

func TestServeLoggingCompatibility(t *testing.T) {
	t.Run("ChunkedRequestLimit", testCompatibilityChunkedRequestLimit)
	t.Run("Metrics", testCompatibilityMetrics)
	t.Run("FailingStdout", testCompatibilityFailingStdout)
}

func compatibilityRequest(t *testing.T, endpoint, method, token, session, body, publicID string) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build compatibility request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if session != "" {
		request.Header.Set("Mcp-Session-Id", session)
		request.Header.Set("Mcp-Protocol-Version", "2025-11-25")
	}
	if publicID != "" {
		request.Header.Set("X-Request-ID", publicID)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("compatibility request: %v", err)
	}
	content, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatalf("read compatibility response: %v", readErr)
	}
	return response, content
}

func compatibilityRecords(t *testing.T, captured string) []map[string]any {
	t.Helper()
	var records []map[string]any
	decoder := json.NewDecoder(strings.NewReader(captured))
	for {
		var record map[string]any
		err := decoder.Decode(&record)
		if err == io.EOF {
			return records
		}
		if err != nil {
			t.Fatalf("decode attempted structured logs: %v", err)
		}
		records = append(records, record)
	}
}

func compatibilityNoSecrets(t *testing.T, captured string, additional ...string) {
	t.Helper()
	for _, secret := range append(additional, serveClientToken, "compatibility-rpc-secret", "compatibility-client-secret") {
		if secret != "" && strings.Contains(captured, secret) {
			t.Error("attempted structured stdout contains a synthetic client/session secret")
		}
	}
}

func testCompatibilityChunkedRequestLimit(t *testing.T) {
	writer := newCompatibilityLogWriter(false)
	_, endpoint, _, stop := startHTTPLoggingBridgeWithWriter(t, "", writer)
	const limit = 1048576 // Existing writeBridgeConfig max_request_bytes.
	const publicID = "dddddddddddddddddddddddddddddddd"
	prefix := strings.TrimSuffix(compatibilityInitialize, "}") + `,"chunked-body-secret":"`
	suffix := `"}`
	payload := prefix + strings.Repeat("x", limit+16-len(prefix)-len(suffix)) + suffix
	if !json.Valid([]byte(payload)) || len(payload) != limit+16 {
		t.Fatal("chunked fixture must be valid initialize JSON exactly 16 bytes past the configured limit")
	}
	request, err := http.NewRequest(http.MethodPost, endpoint+"/mcp", io.NopCloser(strings.NewReader(payload)))
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = -1
	request.TransferEncoding = []string{"chunked"}
	request.Header.Set("Authorization", "Bearer "+serveClientToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("X-Request-ID", publicID)
	connection, err := net.DialTimeout("tcp", request.URL.Host, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := request.Write(connection); err != nil {
		t.Fatalf("write actual chunked initialize: %v", err)
	}
	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		t.Fatalf("read chunked response: %v", err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		t.Fatalf("read chunked error body: %v", readErr)
	}
	// The pinned SDK preserves *http.MaxBytesError and emits 413 from
	// streamableServerConn.servePOST; ordinary read failures alone use 400.
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("chunked SDK typed-limit status = %d; want existing 413", response.StatusCode)
	}
	if len(body) == 0 {
		t.Fatal("chunked SDK typed-limit response body is empty")
	}
	if response.Header.Get("Content-Type") != "text/plain; charset=utf-8" || response.Header.Get("WWW-Authenticate") != "" || response.Header.Get("X-Request-ID") != publicID {
		t.Fatal("chunked read-error public content-type/challenge/request-ID changed")
	}
	t.Logf("public prerequisites passed: valid chunked initialize, ContentLength=-1, 16-byte residual; SDK typed-limit 413/nonempty-body/header contract; response.Close=%v", response.Close)
	if !response.Close {
		t.Error("streamed request limit lost native close-on-limit behavior: response.Close=false")
	} else if _, err := reader.ReadByte(); err != io.EOF {
		t.Errorf("native close-on-limit wire result = %v; want EOF", err)
	}
	stop()
	compatibilityNoSecrets(t, writer.String(), publicID, "chunked-body-secret", strings.Repeat("x", 64))
	assertHTTPLoggingTerminal(t, writer.String(), response.StatusCode, "POST", "mcp", "request_limits", "request_too_large", publicID, "le_1k", true)
}

func compatibilityMetrics(t *testing.T, endpoint string) map[string]*dto.MetricFamily {
	t.Helper()
	response, err := (&http.Client{Timeout: 5 * time.Second}).Get(endpoint + "/metrics")
	if err != nil {
		t.Fatalf("real operations metrics GET: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("real operations metrics status = %d; want 200 under existing limiter", response.StatusCode)
	}
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(response.Body)
	if err != nil {
		t.Fatalf("parse metrics exposition: %v", err)
	}
	return families
}

func compatibilityRequestCounters(t *testing.T, families map[string]*dto.MetricFamily) map[string]float64 {
	t.Helper()
	counts := map[string]float64{}
	family := families["bridge_requests_total"]
	if family == nil {
		return counts
	}
	if family.GetType() != dto.MetricType_COUNTER {
		t.Error("bridge_requests_total must remain a counter")
	}
	for _, metric := range family.Metric {
		labels := map[string]string{}
		for _, pair := range metric.Label {
			labels[pair.GetName()] = pair.GetValue()
		}
		if len(labels) != 3 || labels["server"] != "_bridge" || !slices.Contains([]string{"request", "initialize", "initialized", "tools_list", "tools_call", "resources_list", "resources_read", "ping", "prompts_list", "prompts_get", "sse_stream"}, labels["operation"]) || !slices.Contains([]string{"success", "failure"}, labels["outcome"]) {
			t.Errorf("inbound metrics label contract changed: %v", labels)
		}
		counts[labels["operation"]+"/"+labels["outcome"]] += metric.GetCounter().GetValue()
	}
	return counts
}

func testCompatibilityMetrics(t *testing.T) {
	writer := newCompatibilityLogWriter(false)
	_, endpoint, operations, stop := startHTTPLoggingBridgeWithWriter(t, "", writer)
	before := compatibilityRequestCounters(t, compatibilityMetrics(t, operations))
	response, _ := compatibilityRequest(t, endpoint+"/mcp", "POST", "metrics-bad-token-secret", "", `{}`, "")
	if response.StatusCode != http.StatusUnauthorized || response.Header.Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("metric failure fixture did not preserve public 401/Bearer")
	}
	writer.waitTerminals(t, 1)
	response, _ = compatibilityRequest(t, endpoint+"/metrics-path-secret", "POST", serveClientToken, "", `{}`, "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unmatched metric fixture status = %d; want 404", response.StatusCode)
	}
	writer.waitTerminals(t, 2)
	response, body := compatibilityRequest(t, endpoint+"/mcp", "POST", serveClientToken, "", compatibilityInitialize, "")
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), `"result"`) || !strings.Contains(string(body), `"compatibility-rpc-secret"`) {
		t.Fatal("metrics initialize prerequisite did not return a matching successful result")
	}
	session := response.Header.Get("Mcp-Session-Id")
	if session == "" {
		t.Fatal("metrics initialize omitted real session identity")
	}
	writer.waitTerminals(t, 3)
	response, _ = compatibilityRequest(t, endpoint+"/mcp", "POST", serveClientToken, session, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, "")
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized notification status = %d; want 202", response.StatusCode)
	}
	writer.waitTerminals(t, 4)
	for index, requestBody := range []string{
		`{"jsonrpc":"2.0","id":21,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":22,"method":"prompts/get","params":{"name":"metrics-prompt-secret","arguments":{}}}`,
	} {
		response, _ = compatibilityRequest(t, endpoint+"/mcp", "POST", serveClientToken, session, requestBody, "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("prompt metric fixture status = %d; want protocol-level 200", response.StatusCode)
		}
		writer.waitTerminals(t, 5+index)
	}
	// Exactly two real scrapes total; synchronization is on actual terminal
	// attempts after request metrics, not repeated scraping or client EOF.
	families := compatibilityMetrics(t, operations)
	after := compatibilityRequestCounters(t, families)
	wantDelta := map[string]float64{"request/failure": 1, "initialize/success": 1, "initialized/success": 1, "prompts_list/success": 1, "prompts_get/success": 1}
	for _, operation := range []string{"request", "initialize", "initialized", "prompts_list", "prompts_get"} {
		for _, outcome := range []string{"success", "failure"} {
			key := operation + "/" + outcome
			if got := after[key] - before[key]; got != wantDelta[key] {
				t.Errorf("bridge_requests_total delta %s = %v; want %v (404 must remain uncounted)", key, got, wantDelta[key])
			}
		}
	}
	for name, contract := range map[string]struct {
		kind   dto.MetricType
		labels string
	}{
		"bridge_requests_total":        {dto.MetricType_COUNTER, "operation,outcome,server"},
		"bridge_catalog_tools":         {dto.MetricType_GAUGE, "server"},
		"bridge_catalog_prompts":       {dto.MetricType_GAUGE, ""},
		"bridge_catalog_age_seconds":   {dto.MetricType_GAUGE, "server"},
		"bridge_catalog_refresh_total": {dto.MetricType_COUNTER, "outcome,reason,server"},
		"bridge_readiness":             {dto.MetricType_GAUGE, "server"},
	} {
		family := families[name]
		if family == nil || family.GetType() != contract.kind || len(family.Metric) == 0 {
			t.Errorf("metric family %s missing or changed type", name)
			continue
		}
		for _, metric := range family.Metric {
			var keys []string
			for _, pair := range metric.Label {
				keys = append(keys, pair.GetName())
			}
			slices.Sort(keys)
			if strings.Join(keys, ",") != contract.labels {
				t.Errorf("metric family %s labels = %v; want %s", name, keys, contract.labels)
			}
		}
	}
	response, _ = compatibilityRequest(t, endpoint+"/mcp", "DELETE", serveClientToken, session, "", "")
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("metrics session deletion status = %d", response.StatusCode)
	}
	writer.waitTerminals(t, 7)
	stop()
	compatibilityNoSecrets(t, writer.String(), session, "metrics-bad-token-secret", "metrics-path-secret", "metrics-prompt-secret")
	t.Log("real operations metrics regression: two 200 scrapes, family types/labels and controlled request deltas checked after actual HTTP terminal capture")
}

func testCompatibilityFailingStdout(t *testing.T) {
	writer := newCompatibilityLogWriter(true)
	_, endpoint, _, stop := startHTTPLoggingBridgeWithWriter(t, "", writer)
	const publicID = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	var session string
	for index, token := range []string{"stdout-bad-token-secret", serveClientToken, "stdout-bad-token-secret"} {
		body := `{}`
		wantStatus := http.StatusUnauthorized
		wantChallenge := "Bearer"
		if index == 1 {
			body, wantStatus, wantChallenge = compatibilityInitialize, http.StatusOK, ""
		}
		response, content := compatibilityRequest(t, endpoint+"/mcp", "POST", token, "", body, publicID)
		if response.StatusCode != wantStatus || response.Header.Get("WWW-Authenticate") != wantChallenge || response.Header.Get("X-Request-ID") != publicID {
			t.Fatalf("stdout failure changed public response %d: status=%d challenge=%q", index, response.StatusCode, response.Header.Get("WWW-Authenticate"))
		}
		if index == 1 {
			if !strings.Contains(string(content), `"result"`) || !strings.Contains(string(content), `"compatibility-rpc-secret"`) || strings.Contains(string(content), `"error"`) {
				t.Fatal("stdout failure changed real initialize result")
			}
			session = response.Header.Get("Mcp-Session-Id")
		}
	}
	t.Log("public prerequisites passed despite injected stdout failure: 401/Bearer, real successful initialize/200, next 401/Bearer; public IDs unchanged")
	stop()
	captured := writer.String()
	compatibilityNoSecrets(t, captured, publicID, session, "stdout-bad-token-secret")
	terminals := 0
	counts := map[float64]int{}
	identities := map[string]bool{}
	for _, record := range compatibilityRecords(t, captured) {
		if record["event"] != "http_complete" {
			continue
		}
		terminals++
		status, ok := record["http_status"].(float64)
		if !ok {
			t.Error("attempted terminal lacks observed HTTP status")
		}
		counts[status]++
		identity, _ := record["request_id"].(string)
		if identity == "" || identities[identity] {
			t.Error("terminal writer retried/duplicated a request or lost its independent identity")
		}
		identities[identity] = true
		if record["stage"] != "client_auth" && record["stage"] != "none" {
			t.Error("stdout delivery failure changed the observed request stage")
		}
	}
	if terminals != 3 || counts[401] != 2 || counts[200] != 1 {
		t.Errorf("failed stdout terminal attempts = %d (401=%d, 200=%d); want exactly one per request: 3 (401=2, 200=1)", terminals, counts[401], counts[200])
	}
	t.Log("stdout capture represents attempted bytes only, not delivered logs; shutdown completed normally")
}
