package observe

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// This file implements the capability-preserving response observation: the
// shared owner methods, the one-time capability probe and the delegate
// helpers the generated facades forward to. The facades themselves are
// checked-in generated code (httpwriter_facades_generated.go).

// Fixed six-capability mask bits: F=Flusher, H=Hijacker, P=Pusher,
// C=CloseNotifier, R=ReaderFrom, S=StringWriter.
const (
	httpCapFlusher uint8 = 1 << iota
	httpCapHijacker
	httpCapPusher
	httpCapCloseNotifier
	httpCapReaderFrom
	httpCapStringWriter
)

// httpOwnerInit is the one-time initialization of a facade-embedded owner.
// It is built once per claimed request and copied into the single facade
// allocation; no separate base or state wrapper is allocated.
type httpOwnerInit struct {
	diagnostics   *HTTPDiagnostics
	writer        http.ResponseWriter
	delegates     httpDelegates
	started       time.Time
	requestID     string
	method        string
	route         string
	pathClass     string
	sessionHeader string
}

// httpDelegates caches the underlying writer's optional capabilities probed
// once at claim time.
type httpDelegates struct {
	flusher       http.Flusher
	hijacker      http.Hijacker
	pusher        http.Pusher
	closeNotifier http.CloseNotifier
	readerFrom    io.ReaderFrom
	stringWriter  io.StringWriter
	// flushError is the native error-aware flushing adapter; it carries no
	// mask bit and never makes the facade advertise http.Flusher.
	flushError interface{ FlushError() error }
}

// probeCapabilities reads the underlying writer's six optional capabilities
// plus the error-aware flush adapter exactly once.
func probeCapabilities(w http.ResponseWriter) (uint8, httpDelegates) {
	var mask uint8
	var delegates httpDelegates
	if flusher, ok := w.(http.Flusher); ok {
		mask |= httpCapFlusher
		delegates.flusher = flusher
	}
	if flushError, ok := w.(interface{ FlushError() error }); ok {
		delegates.flushError = flushError
	}
	if hijacker, ok := w.(http.Hijacker); ok {
		mask |= httpCapHijacker
		delegates.hijacker = hijacker
	}
	if pusher, ok := w.(http.Pusher); ok {
		mask |= httpCapPusher
		delegates.pusher = pusher
	}
	if notifier, ok := w.(http.CloseNotifier); ok {
		mask |= httpCapCloseNotifier
		delegates.closeNotifier = notifier
	}
	if readerFrom, ok := w.(io.ReaderFrom); ok {
		mask |= httpCapReaderFrom
		delegates.readerFrom = readerFrom
	}
	if stringWriter, ok := w.(io.StringWriter); ok {
		mask |= httpCapStringWriter
		delegates.stringWriter = stringWriter
	}
	return mask, delegates
}

// init initializes the facade-embedded owner in place.
func (o *httpOwner) init(state httpOwnerInit) {
	o.diagnostics = state.diagnostics
	o.writer = state.writer
	o.delegates = state.delegates
	o.started = state.started
	o.requestID = state.requestID
	o.method = state.method
	o.route = state.route
	o.pathClass = state.pathClass
	o.sessionHeader = state.sessionHeader
	o.writeFailure = HTTPWriteNone
	if validHexID(state.requestID) {
		o.correlationState = CorrelationAvailable
	} else {
		o.correlationState = CorrelationUnavailable
	}
}

// States of the terminal correlation snapshot.
const (
	CorrelationAvailable   = "available"
	CorrelationUnavailable = "unavailable"
	StatusObserved         = "observed"
	StatusUnobserved       = "unavailable"
)

// httpOwner is the sole bounded diagnostic state of one claimed request. It
// is embedded by value in exactly one generated facade; the carrier method
// lets a lost context marker be recovered through the writer chain.
type httpOwner struct {
	diagnostics      *HTTPDiagnostics
	writer           http.ResponseWriter
	delegates        httpDelegates
	started          time.Time
	requestID        string
	correlationState string
	method           string
	route            string
	pathClass        string
	sessionHeader    string

	// Response observation.
	status         int
	statusObserved bool
	bytes          int64
	writeErr       error
	writeFailure   string
	hijacked       bool

	finished bool

	// Contributions (closed evidence only).
	failureSet         bool
	failureStage       string
	failureReason      string
	authMode           string
	credentialState    string
	scopeState         string
	sessionState       string
	bindingRelation    string
	sessionRefState    string
	sessionRef         string
	challengeKind      string
	challengeTarget    string
	metadataRegistered string
	routeOutcomeSet    bool
	routeOutcome       string
	operationRelation  string
	operationIDState   string
	operationID        string
	upstreamState      string
	errorDetailState   string
	errorDetail        *HTTPDiagnosticErrorDetail
	legacyStage        string
	legacyReason       string
	legacyOperation    string
}

func (o *httpOwner) header() http.Header { return o.writer.Header() }

func (o *httpOwner) writeHeader(status int) {
	if o.statusObserved {
		// The first status is the response's status; forwarding a second one
		// would only log a superfluous call.
		return
	}
	o.statusObserved = true
	o.status = status
	o.writer.WriteHeader(status)
}

// recordWriteFailure stores the first observed delivery failure as a closed
// class; the raw error is kept only for internal errors.Is classification.
func (o *httpOwner) recordWriteFailure(err error) {
	if o.writeErr == nil && err != nil {
		o.writeErr = err
		o.writeFailure = classifyWriteFailure(err)
	}
}

func (o *httpOwner) write(p []byte) (int, error) {
	if !o.statusObserved {
		// Record the implicit 200 exactly as net/http would commit it; do not
		// forward a second header write.
		o.statusObserved = true
		o.status = http.StatusOK
	}
	n, err := o.writer.Write(p)
	o.bytes += int64(n)
	o.recordWriteFailure(err)
	return n, err
}

func (o *httpOwner) flush() {
	if !o.statusObserved {
		o.writeHeader(http.StatusOK)
	}
	o.delegates.flusher.Flush()
}

// flushError delegates native error-aware flushing first, otherwise supported
// ordinary flushing; with neither it returns ErrNotSupported without any
// committed status, false success or response side effect. A failed error-aware
// flush without observed commitment never invents a 200.
func (o *httpOwner) flushError() error {
	if o.delegates.flushError != nil {
		err := o.delegates.flushError.FlushError()
		if err == nil {
			if !o.statusObserved {
				o.statusObserved = true
				o.status = http.StatusOK
			}
			return nil
		}
		o.recordWriteFailure(err)
		return err
	}
	if o.delegates.flusher != nil {
		o.flush()
		return nil
	}
	return http.ErrNotSupported
}

func (o *httpOwner) hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := o.delegates.hijacker.Hijack()
	if err == nil {
		// Hijacked connection bytes are directly written and unobservable;
		// the bucket stays unavailable and any unobserved status stays null.
		o.hijacked = true
	}
	return conn, buffer, err
}

func (o *httpOwner) push(target string, options *http.PushOptions) error {
	return o.delegates.pusher.Push(target, options)
}

func (o *httpOwner) closeNotify() <-chan bool {
	return o.delegates.closeNotifier.CloseNotify()
}

// readFrom delegates optimized copying exactly once and observes the actual
// returned count and error. A positive count establishes the implicit 200; a
// zero count without other commitment stays unavailable.
func (o *httpOwner) readFrom(reader io.Reader) (int64, error) {
	n, err := o.delegates.readerFrom.ReadFrom(reader)
	if n > 0 {
		if !o.statusObserved {
			o.statusObserved = true
			o.status = http.StatusOK
		}
		o.bytes += n
	}
	o.recordWriteFailure(err)
	return n, err
}

func (o *httpOwner) writeString(s string) (int, error) {
	if !o.statusObserved {
		o.statusObserved = true
		o.status = http.StatusOK
	}
	n, err := o.delegates.stringWriter.WriteString(s)
	o.bytes += int64(n)
	o.recordWriteFailure(err)
	return n, err
}

// classifyWriteFailure maps a retained delivery error into its closed class.
// Classification is typed only: the raw error text is never parsed, retained
// in the record, or emitted.
func classifyWriteFailure(err error) string {
	switch {
	case err == nil:
		return HTTPWriteNone
	case errors.Is(err, net.ErrClosed):
		return HTTPWriteDisconnected
	case errors.Is(err, context.Canceled):
		return HTTPWriteCancelled
	case errors.Is(err, context.DeadlineExceeded):
		return HTTPWriteDeadline
	default:
		return HTTPWriteUnknown
	}
}

// ownerFromContext returns the request's owner from the private typed
// context marker, or nil when the marker is absent.
func ownerFromContext(ctx context.Context) *httpOwner {
	if ctx == nil {
		return nil
	}
	owner, _ := ctx.Value(httpCompletionContextKey{}).(*httpOwner)
	return owner
}

func withOwnerContext(ctx context.Context, owner *httpOwner) context.Context {
	return context.WithValue(ctx, httpCompletionContextKey{}, owner)
}

// httpCompletionContextKey is the private typed context marker. External
// packages cannot construct it, so request headers can never supply identity.
type httpCompletionContextKey struct{}

// httpOwnerCarrier is the private writer-carrier interface. The generated
// facades promote completionOwner, allowing recovery of the same owner when
// the context marker was lost between layers.
type httpOwnerCarrier interface {
	completionOwner() *httpOwner
}

// ownerFromCarrier recovers the owner from the writer chain.
func ownerFromCarrier(w http.ResponseWriter) *httpOwner {
	if carrier, ok := w.(httpOwnerCarrier); ok {
		return carrier.completionOwner()
	}
	return nil
}
