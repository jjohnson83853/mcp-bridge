package observe

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestConcurrentEvidenceByIDLinksAllOperations exercises the trusted BYD
// contribution seam with N concurrent contributors on one owner, the shape a
// concurrent JSON-RPC batch produces. All contributions must survive: the
// terminal record must report operation_relation "multiple" with every
// distinct server-minted operation observed, never a lost-update "single".
func TestConcurrentEvidenceByIDLinksAllOperations(t *testing.T) {
	const ops = 16
	const iterations = 5
	now := fixedNow()
	for iteration := range iterations {
		sink := &bytes.Buffer{}
		diag := newTestDiagnostics(sink, now, fixedID(testID))
		handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := HTTPRequestID(r.Context())
			if requestID == "" {
				t.Error("no owned request identity available to the BYD seam")
				return
			}
			ready := make(chan struct{})
			var contributors sync.WaitGroup
			var release sync.WaitGroup
			for i := range ops {
				contributors.Add(1)
				go func(i int) {
					defer contributors.Done()
					<-ready
					ContributeOperationEvidenceByID(requestID, HTTPOperationEvidence{
						ID:       fmt.Sprintf("%032x", i+1),
						Upstream: "succeeded",
					})
				}(i)
			}
			release.Add(1)
			go func() {
				defer release.Done()
				contributors.Wait()
			}()
			close(ready)
			release.Wait()
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))
		record := decodeTerminal(t, sink)
		if record["operation_relation"] != "multiple" {
			t.Fatalf("iteration %d: operation_relation = %v; want multiple (all %d concurrent contributions must survive)",
				iteration, record["operation_relation"], ops)
		}
		if record["operation_id_state"] != "not_applicable" {
			t.Errorf("iteration %d: operation_id_state = %v; want not_applicable", iteration, record["operation_id_state"])
		}
		if _, present := record["operation_id"]; present {
			t.Errorf("iteration %d: operation_id emitted for a multiple relation", iteration)
		}
		if record["upstream_state"] != "succeeded" {
			t.Errorf("iteration %d: upstream_state = %v; want succeeded", iteration, record["upstream_state"])
		}
	}
}
