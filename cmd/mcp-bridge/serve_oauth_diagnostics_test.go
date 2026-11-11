package main

// Outer OAuth diagnostics wire tests. Every POST /mcp response must carry a
// response-owned Mcp-Diagnostic-Id that joins to exactly one http_complete
// record carrying the oauth_diag_v1 semantic fields, while no bearer token,
// claim value, session identity, or caller-supplied sentinel ever reaches the
// captured terminal record. The tests reuse the composed oauthTestHarness;
// no production types are referenced, all new diagnostic fields are decoded
// dynamically from the captured JSON records.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// diagnosticRecordLimit is the complete-record byte bound from the approved
// diagnostics contract (compact core plus optional increments, newline
// included).
const diagnosticRecordLimit = 4096

// diagPost performs one POST /mcp with a verbatim Authorization value, caller
// header canaries, and an optional query string. It mirrors oauthPost's wire
// shape so every case exercises the same outer edge; an empty authorization
// sends no Authorization header at all.
func diagPost(t *testing.T, harness *oauthTestHarness, authorization, query string, headers map[string]string, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, harness.httpURL+"/mcp"+query, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("POST %s/mcp: %v", harness.httpURL, err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

// diagCanaries derives three distinct 32-character lowercase hexadecimal
// caller-supplied canaries: a public X-Request-ID, an internal request id
// header, and a spoofed diagnostic id header. An owned diagnostic identity
// must never equal any of them.
func diagCanaries(index int) (publicID, internalID, diagnosticID string) {
	return fmt.Sprintf("%032x", index*3+1), fmt.Sprintf("%032x", index*3+2), fmt.Sprintf("%032x", index*3+3)
}

// diagCompletionsFor returns the captured http_complete records whose
// request_id equals the response-owned diagnostic id. Filtering by the owned
// identity keeps readiness probes and unrelated harness traffic out of every
// count and content assertion.
func diagCompletionsFor(t *testing.T, harness *oauthTestHarness, diagnosticID string) []map[string]any {
	t.Helper()
	var matched []map[string]any
	for _, record := range filterRecords(decodeCapturedRecords(t, harness.stdout.String()+harness.stderr.String()), "http_complete") {
		if record["request_id"] == diagnosticID {
			matched = append(matched, record)
		}
	}
	return matched
}

// requireDiagnosticID asserts the response carries a fresh 32-character
// lowercase hexadecimal Mcp-Diagnostic-Id that none of the caller-supplied
// canaries chose, and returns it. This is the fail-fast gate: before the
// diagnostics feature exists the header is absent and the test aborts here
// instead of waiting for a log record that can never arrive.
func requireDiagnosticID(t *testing.T, response *http.Response, canaries ...string) string {
	t.Helper()
	diagnosticID := response.Header.Get("Mcp-Diagnostic-Id")
	if !internalIDPattern.MatchString(diagnosticID) {
		t.Fatalf("response Mcp-Diagnostic-Id = %q; want a 32-character lowercase hexadecimal owned diagnostic identity", diagnosticID)
	}
	for _, canary := range canaries {
		if diagnosticID == canary {
			t.Fatalf("response Mcp-Diagnostic-Id equals a caller-supplied canary; the owned identity must not be chosen by request headers")
		}
	}
	return diagnosticID
}

// diagSecretVariants expands one supplied value with its simple wire
// encodings so the leak checks cover raw and lightly encoded forms.
func diagSecretVariants(value string) []string {
	return []string{value, url.QueryEscape(value)}
}

// waitForDiagnosticCompletion drains the synchronized stdout capture until the
// response-owned completion record is observable, bounded by waitFor's
// deadline; no sleeps and no aggregated counts across readiness traffic.
func waitForDiagnosticCompletion(t *testing.T, harness *oauthTestHarness, what, diagnosticID string) []map[string]any {
	t.Helper()
	waitFor(t, what, func() bool {
		return len(diagCompletionsFor(t, harness, diagnosticID)) > 0
	})
	return diagCompletionsFor(t, harness, diagnosticID)
}

// requireOuterOAuthRecord asserts the shared semantic shape of one captured
// http_complete record joined by the response-owned diagnostic id: the
// physical wire fields, the oauth_diag_v1 schema, the outer rejection stage
// with the case's finite reason, the actual credential/scope states, and the
// untouched session/upstream states.
func requireOuterOAuthRecord(t *testing.T, record map[string]any, status int, reason, credentialState, scopeState string) {
	t.Helper()
	for key, want := range map[string]any{
		"http_status":      float64(status),
		"http_method":      "POST",
		"route":            "mcp",
		"diag_schema":      "oauth_diag_v1",
		"failure_stage":    "outer_oauth",
		"failure_reason":   reason,
		"credential_state": credentialState,
		"scope_state":      scopeState,
		"session_state":    "not_evaluated",
		"upstream_state":   "not_attempted",
	} {
		if got := record[key]; got != want {
			t.Errorf("http_complete %s = %v; want %v", key, got, want)
		}
	}
}

// TestServeOAuthDiagnosticRejections walks the outer OAuth rejection wire:
// each refusal keeps the existing status and byte-exact challenge, carries a
// response-owned Mcp-Diagnostic-Id, and joins to exactly one http_complete
// record with the outer_oauth failure stage and its finite failure reason.
// Caller-supplied id canaries must never select the owned identity, and no
// token or claim value may appear in the captured record.
func TestServeOAuthDiagnosticRejections(t *testing.T) {
	harness := startOAuthHarness(t)
	harness.awaitLive(t)
	harness.awaitReady(t)

	now := time.Now().UTC()
	wrongAudience := harness.oauthToken(t, now, map[string]any{"aud": "https://audience-elsewhere.example.com"})
	unscoped := harness.oauthToken(t, now, map[string]any{"scope": "openid email"})
	insufficientScopeChallenge := `Bearer error="insufficient_scope", scope="` + oauthScope + `"`

	for index, tc := range []struct {
		name             string
		bearerToken      string // sent through oauthPost as "Bearer <token>"; empty sends no header
		rawAuthorization string // sent verbatim through diagPost when nonempty
		reason           string
		credentialState  string
		scopeState       string
		status           int
		challenge        string
	}{
		{name: "MissingCredential", reason: "credentials_missing", credentialState: "missing", scopeState: "not_evaluated", status: http.StatusUnauthorized, challenge: oauthChallenge},
		{name: "WrongScheme", rawAuthorization: "Basic ZGlhZ25vc3RpYzpzZW5zaXRpdmU=", reason: "bearer_malformed", credentialState: "rejected", scopeState: "not_evaluated", status: http.StatusUnauthorized, challenge: oauthChallenge},
		{name: "InvalidJWT", bearerToken: "aa.bb.cc", reason: "jwt_invalid_other", credentialState: "rejected", scopeState: "not_evaluated", status: http.StatusUnauthorized, challenge: oauthChallenge},
		{name: "WrongVerifiedAudience", rawAuthorization: "Bearer " + wrongAudience, reason: "jwt_audience_invalid", credentialState: "rejected", scopeState: "not_evaluated", status: http.StatusUnauthorized, challenge: oauthChallenge},
		{name: "InsufficientScope", bearerToken: unscoped, reason: "scope_insufficient", credentialState: "accepted", scopeState: "rejected", status: http.StatusForbidden, challenge: insufficientScopeChallenge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publicID, internalCanary, diagnosticCanary := diagCanaries(index + 1)
			spoofed := map[string]string{
				"X-Request-ID":          publicID,
				"X-Internal-Request-Id": internalCanary,
				"Mcp-Diagnostic-Id":     diagnosticCanary,
			}
			var response *http.Response
			if tc.rawAuthorization != "" {
				response = diagPost(t, harness, tc.rawAuthorization, "", spoofed, `{}`)
			} else {
				response = oauthPost(t, harness.httpURL, tc.bearerToken, `{}`)
			}

			// Existing wire shape first: the exact retained status and the
			// byte-exact challenge, before any log dependency.
			if response.StatusCode != tc.status {
				t.Fatalf("%s: status = %d; want %d", tc.name, response.StatusCode, tc.status)
			}
			if got := response.Header.Get("WWW-Authenticate"); got != tc.challenge {
				t.Fatalf("%s: WWW-Authenticate = %q; want %q", tc.name, got, tc.challenge)
			}

			// Fail fast on the response-owned identity; a missing header must
			// abort here rather than hang waiting for an impossible log line.
			diagnosticID := requireDiagnosticID(t, response, publicID, internalCanary, diagnosticCanary)

			record := waitForDiagnosticCompletion(t, harness, "rejection http_complete owned by diagnostic id", diagnosticID)
			if len(record) != 1 {
				t.Fatalf("rejection completion count for owned diagnostic id = %d; want exactly 1", len(record))
			}
			requireOuterOAuthRecord(t, record[0], tc.status, tc.reason, tc.credentialState, tc.scopeState)

			raw, err := json.Marshal(record[0])
			if err != nil {
				t.Fatalf("re-encode rejection diagnostic record: %v", err)
			}
			if len(raw) > diagnosticRecordLimit {
				t.Errorf("rejection diagnostic record is %d bytes; want <= %d", len(raw), diagnosticRecordLimit)
			}
			for canaryIndex, canary := range []string{publicID, internalCanary, diagnosticCanary} {
				if strings.Contains(string(raw), canary) {
					t.Errorf("rejection diagnostic record leaks caller-supplied canary index %d", canaryIndex)
				}
			}
		})
	}
}

// TestServeOAuthDiagnosticAcceptedControl pins the accepted side of the
// diagnostics contract: a valid initialize keeps the existing 200/result/
// session behavior, its response-owned Mcp-Diagnostic-Id joins to exactly one
// http_complete record with no false rejection, the record stays within its
// byte bound, and no synthetic token, claim value, session identity, or
// caller-supplied header/query sentinel — raw or simply encoded — reaches the
// captured terminal bytes.
func TestServeOAuthDiagnosticAcceptedControl(t *testing.T) {
	harness := startOAuthHarness(t)
	harness.awaitLive(t)
	harness.awaitReady(t)

	now := time.Now().UTC()
	token := harness.oauthToken(t, now, nil)

	const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"diagnostic-wire","version":"1"}}}`
	const headerSentinel = "diag-header-sentinel-3mf"
	const querySentinel = "diag-query-sentinel-7qz"
	publicID, internalCanary, diagnosticCanary := diagCanaries(97)

	response := diagPost(t, harness, "Bearer "+token, "?probe="+querySentinel, map[string]string{
		"X-Request-ID":          publicID,
		"X-Internal-Request-Id": internalCanary,
		"Mcp-Diagnostic-Id":     diagnosticCanary,
		"X-Client-Trace":        headerSentinel,
	}, initializeBody)

	// Existing public behavior: 200 with a JSON-RPC result and an issued
	// session, untouched by the diagnostics extension.
	if response.StatusCode != http.StatusOK {
		payload, _ := readAll(response)
		t.Fatalf("accepted initialize: status = %d; want 200 (body=%s)", response.StatusCode, payload)
	}
	result := oauthJSONBody(t, response)
	if result["jsonrpc"] != "2.0" {
		t.Fatalf("initialize response = %v; want jsonrpc 2.0", result)
	}
	if _, ok := result["result"]; !ok {
		t.Fatalf("initialize response lacks result: %v", result)
	}
	sessionID := response.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("initialize omitted session identity")
	}
	diagnosticID := requireDiagnosticID(t, response, publicID, internalCanary, diagnosticCanary)

	record := waitForDiagnosticCompletion(t, harness, "accepted http_complete owned by diagnostic id", diagnosticID)
	if len(record) != 1 {
		t.Fatalf("accepted completion count for owned diagnostic id = %d; want exactly 1", len(record))
	}
	for key, want := range map[string]any{
		"http_status":      float64(http.StatusOK),
		"http_method":      "POST",
		"route":            "mcp",
		"diag_schema":      "oauth_diag_v1",
		"failure_stage":    "none",
		"failure_reason":   "none",
		"credential_state": "accepted",
		"scope_state":      "accepted",
		"upstream_state":   "not_attempted",
	} {
		if got := record[0][key]; got != want {
			t.Errorf("accepted http_complete %s = %v; want %v", key, got, want)
		}
	}

	raw, err := json.Marshal(record[0])
	if err != nil {
		t.Fatalf("re-encode accepted diagnostic record: %v", err)
	}
	if len(raw) > diagnosticRecordLimit {
		t.Errorf("accepted diagnostic record is %d bytes; want <= %d", len(raw), diagnosticRecordLimit)
	}
	// Supplied credentials, claims, identities, and caller header/query
	// sentinels never appear in the terminal record, raw or simply encoded.
	// Failure messages name the category only; values are never printed.
	forbidden := map[string]string{
		"bearer token":         token,
		"subject claim":        oauthTestSubject,
		"email claim":          oauthTestSubject + "@example.com",
		"issuer claim":         oauthIssuer,
		"resource audience":    oauthResource,
		"query sentinel":       querySentinel,
		"header sentinel":      headerSentinel,
		"public id canary":     publicID,
		"internal id canary":   internalCanary,
		"diagnostic id canary": diagnosticCanary,
		"session identity":     sessionID,
	}
	for label, value := range forbidden {
		for variantIndex, variant := range diagSecretVariants(value) {
			if strings.Contains(string(raw), variant) {
				t.Errorf("accepted diagnostic record leaks %s (variant %d)", label, variantIndex)
			}
		}
	}
}
