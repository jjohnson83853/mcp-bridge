package observe

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This file exercises the 64 generated facade types and the owner writer
// methods as a consumer-behavior matrix. Fakes are composed from recording
// facet structs so presence and absence of every capability, delegate value
// and error propagation are observed, not inferred.

// --- recording facets ---

type baseFacet struct {
	header    http.Header
	code      int
	wroteCode int
	body      bytes.Buffer
	writeErr  error
	headerN   int
}

func newBaseFacet() baseFacet { return baseFacet{header: http.Header{}} }

func (b *baseFacet) Header() http.Header { return b.header }

func (b *baseFacet) Write(p []byte) (int, error) {
	b.body.Write(p)
	return len(p), b.writeErr
}

func (b *baseFacet) WriteHeader(status int) {
	if b.wroteCode != 0 {
		return
	}
	b.wroteCode++
	b.code = status
}

type flushFacet struct{ flushes int }

func (f *flushFacet) Flush() { f.flushes++ }

type flushErrorFacet struct {
	flushes int
	err     error
}

func (f *flushErrorFacet) Flush() { f.flushes++ }

func (f *flushErrorFacet) FlushError() error {
	f.flushes++
	return f.err
}

type hijackFacet struct {
	conn   net.Conn
	buf    *bufio.ReadWriter
	err    error
	called int
}

func (h *hijackFacet) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.called++
	return h.conn, h.buf, h.err
}

type pushFacet struct {
	err    error
	called int
	target string
}

func (p *pushFacet) Push(target string, _ *http.PushOptions) error {
	p.called++
	p.target = target
	return p.err
}

type notifyFacet struct{ ch chan bool }

func (n *notifyFacet) CloseNotify() <-chan bool { return n.ch }

type readerFromFacet struct {
	n   int64
	err error
}

func (r *readerFromFacet) ReadFrom(src io.Reader) (int64, error) {
	_, _ = io.Copy(io.Discard, src)
	return r.n, r.err
}

type stringWriterFacet struct {
	written string
	n       int
	err     error
}

func (s *stringWriterFacet) WriteString(str string) (int, error) {
	s.written += str
	n := len(str)
	if s.n != 0 {
		n = s.n
	}
	return n, s.err
}

// composedWriter selects one of 64 concrete matrix writer types whose static
// method set exposes exactly the capabilities selected by mask, mirroring how
// the generator composes the 64 facades from probed capabilities. Go method
// sets cannot vary at runtime, so each mask needs its own type.
func composedWriter(mask uint8) http.ResponseWriter {
	switch mask {
	case 0:
		return &matrixWriter000000{}
	case 1:
		return &matrixWriter000001{}
	case 2:
		return &matrixWriter000010{}
	case 3:
		return &matrixWriter000011{}
	case 4:
		return &matrixWriter000100{}
	case 5:
		return &matrixWriter000101{}
	case 6:
		return &matrixWriter000110{}
	case 7:
		return &matrixWriter000111{}
	case 8:
		return &matrixWriter001000{}
	case 9:
		return &matrixWriter001001{}
	case 10:
		return &matrixWriter001010{}
	case 11:
		return &matrixWriter001011{}
	case 12:
		return &matrixWriter001100{}
	case 13:
		return &matrixWriter001101{}
	case 14:
		return &matrixWriter001110{}
	case 15:
		return &matrixWriter001111{}
	case 16:
		return &matrixWriter010000{}
	case 17:
		return &matrixWriter010001{}
	case 18:
		return &matrixWriter010010{}
	case 19:
		return &matrixWriter010011{}
	case 20:
		return &matrixWriter010100{}
	case 21:
		return &matrixWriter010101{}
	case 22:
		return &matrixWriter010110{}
	case 23:
		return &matrixWriter010111{}
	case 24:
		return &matrixWriter011000{}
	case 25:
		return &matrixWriter011001{}
	case 26:
		return &matrixWriter011010{}
	case 27:
		return &matrixWriter011011{}
	case 28:
		return &matrixWriter011100{}
	case 29:
		return &matrixWriter011101{}
	case 30:
		return &matrixWriter011110{}
	case 31:
		return &matrixWriter011111{}
	case 32:
		return &matrixWriter100000{}
	case 33:
		return &matrixWriter100001{}
	case 34:
		return &matrixWriter100010{}
	case 35:
		return &matrixWriter100011{}
	case 36:
		return &matrixWriter100100{}
	case 37:
		return &matrixWriter100101{}
	case 38:
		return &matrixWriter100110{}
	case 39:
		return &matrixWriter100111{}
	case 40:
		return &matrixWriter101000{}
	case 41:
		return &matrixWriter101001{}
	case 42:
		return &matrixWriter101010{}
	case 43:
		return &matrixWriter101011{}
	case 44:
		return &matrixWriter101100{}
	case 45:
		return &matrixWriter101101{}
	case 46:
		return &matrixWriter101110{}
	case 47:
		return &matrixWriter101111{}
	case 48:
		return &matrixWriter110000{}
	case 49:
		return &matrixWriter110001{}
	case 50:
		return &matrixWriter110010{}
	case 51:
		return &matrixWriter110011{}
	case 52:
		return &matrixWriter110100{}
	case 53:
		return &matrixWriter110101{}
	case 54:
		return &matrixWriter110110{}
	case 55:
		return &matrixWriter110111{}
	case 56:
		return &matrixWriter111000{}
	case 57:
		return &matrixWriter111001{}
	case 58:
		return &matrixWriter111010{}
	case 59:
		return &matrixWriter111011{}
	case 60:
		return &matrixWriter111100{}
	case 61:
		return &matrixWriter111101{}
	case 62:
		return &matrixWriter111110{}
	case 63:
		return &matrixWriter111111{}
	}
	return nil
}

// Self-contained recording capability structs. Their zero values are wired
// lazily so the 64 matrix writer types need no per-instance construction.
type capFlusher struct{ flushes int }

func (c *capFlusher) Flush() { c.flushes++ }

type capHijacker struct{}

func (c *capHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, _ := net.Pipe()
	buf := bufio.NewReadWriter(bufio.NewReader(strings.NewReader("")), bufio.NewWriter(io.Discard))
	return conn, buf, nil
}

type capPusher struct{}

func (c *capPusher) Push(string, *http.PushOptions) error { return nil }

type capCloseNotifier struct{}

func (c *capCloseNotifier) CloseNotify() <-chan bool { return make(chan bool) }

type capReaderFrom struct{}

func (c *capReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	_, _ = io.Copy(io.Discard, r)
	return 0, nil
}

type capStringWriter struct{}

func (c *capStringWriter) WriteString(s string) (int, error) { return len(s), nil }

// matrixCore provides the mandatory http.ResponseWriter surface.
type matrixCore struct{ baseFacet }

func (w *matrixCore) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

type matrixWriter000000 struct{ matrixCore }
type matrixWriter000001 struct {
	matrixCore
	capFlusher
}
type matrixWriter000010 struct {
	matrixCore
	capHijacker
}
type matrixWriter000011 struct {
	matrixCore
	capFlusher
	capHijacker
}
type matrixWriter000100 struct {
	matrixCore
	capPusher
}
type matrixWriter000101 struct {
	matrixCore
	capFlusher
	capPusher
}
type matrixWriter000110 struct {
	matrixCore
	capHijacker
	capPusher
}
type matrixWriter000111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
}
type matrixWriter001000 struct {
	matrixCore
	capCloseNotifier
}
type matrixWriter001001 struct {
	matrixCore
	capFlusher
	capCloseNotifier
}
type matrixWriter001010 struct {
	matrixCore
	capHijacker
	capCloseNotifier
}
type matrixWriter001011 struct {
	matrixCore
	capFlusher
	capHijacker
	capCloseNotifier
}
type matrixWriter001100 struct {
	matrixCore
	capPusher
	capCloseNotifier
}
type matrixWriter001101 struct {
	matrixCore
	capFlusher
	capPusher
	capCloseNotifier
}
type matrixWriter001110 struct {
	matrixCore
	capHijacker
	capPusher
	capCloseNotifier
}
type matrixWriter001111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
	capCloseNotifier
}
type matrixWriter010000 struct {
	matrixCore
	capReaderFrom
}
type matrixWriter010001 struct {
	matrixCore
	capFlusher
	capReaderFrom
}
type matrixWriter010010 struct {
	matrixCore
	capHijacker
	capReaderFrom
}
type matrixWriter010011 struct {
	matrixCore
	capFlusher
	capHijacker
	capReaderFrom
}
type matrixWriter010100 struct {
	matrixCore
	capPusher
	capReaderFrom
}
type matrixWriter010101 struct {
	matrixCore
	capFlusher
	capPusher
	capReaderFrom
}
type matrixWriter010110 struct {
	matrixCore
	capHijacker
	capPusher
	capReaderFrom
}
type matrixWriter010111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
	capReaderFrom
}
type matrixWriter011000 struct {
	matrixCore
	capCloseNotifier
	capReaderFrom
}
type matrixWriter011001 struct {
	matrixCore
	capFlusher
	capCloseNotifier
	capReaderFrom
}
type matrixWriter011010 struct {
	matrixCore
	capHijacker
	capCloseNotifier
	capReaderFrom
}
type matrixWriter011011 struct {
	matrixCore
	capFlusher
	capHijacker
	capCloseNotifier
	capReaderFrom
}
type matrixWriter011100 struct {
	matrixCore
	capPusher
	capCloseNotifier
	capReaderFrom
}
type matrixWriter011101 struct {
	matrixCore
	capFlusher
	capPusher
	capCloseNotifier
	capReaderFrom
}
type matrixWriter011110 struct {
	matrixCore
	capHijacker
	capPusher
	capCloseNotifier
	capReaderFrom
}
type matrixWriter011111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
	capCloseNotifier
	capReaderFrom
}
type matrixWriter100000 struct {
	matrixCore
	capStringWriter
}
type matrixWriter100001 struct {
	matrixCore
	capFlusher
	capStringWriter
}
type matrixWriter100010 struct {
	matrixCore
	capHijacker
	capStringWriter
}
type matrixWriter100011 struct {
	matrixCore
	capFlusher
	capHijacker
	capStringWriter
}
type matrixWriter100100 struct {
	matrixCore
	capPusher
	capStringWriter
}
type matrixWriter100101 struct {
	matrixCore
	capFlusher
	capPusher
	capStringWriter
}
type matrixWriter100110 struct {
	matrixCore
	capHijacker
	capPusher
	capStringWriter
}
type matrixWriter100111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
	capStringWriter
}
type matrixWriter101000 struct {
	matrixCore
	capCloseNotifier
	capStringWriter
}
type matrixWriter101001 struct {
	matrixCore
	capFlusher
	capCloseNotifier
	capStringWriter
}
type matrixWriter101010 struct {
	matrixCore
	capHijacker
	capCloseNotifier
	capStringWriter
}
type matrixWriter101011 struct {
	matrixCore
	capFlusher
	capHijacker
	capCloseNotifier
	capStringWriter
}
type matrixWriter101100 struct {
	matrixCore
	capPusher
	capCloseNotifier
	capStringWriter
}
type matrixWriter101101 struct {
	matrixCore
	capFlusher
	capPusher
	capCloseNotifier
	capStringWriter
}
type matrixWriter101110 struct {
	matrixCore
	capHijacker
	capPusher
	capCloseNotifier
	capStringWriter
}
type matrixWriter101111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
	capCloseNotifier
	capStringWriter
}
type matrixWriter110000 struct {
	matrixCore
	capReaderFrom
	capStringWriter
}
type matrixWriter110001 struct {
	matrixCore
	capFlusher
	capReaderFrom
	capStringWriter
}
type matrixWriter110010 struct {
	matrixCore
	capHijacker
	capReaderFrom
	capStringWriter
}
type matrixWriter110011 struct {
	matrixCore
	capFlusher
	capHijacker
	capReaderFrom
	capStringWriter
}
type matrixWriter110100 struct {
	matrixCore
	capPusher
	capReaderFrom
	capStringWriter
}
type matrixWriter110101 struct {
	matrixCore
	capFlusher
	capPusher
	capReaderFrom
	capStringWriter
}
type matrixWriter110110 struct {
	matrixCore
	capHijacker
	capPusher
	capReaderFrom
	capStringWriter
}
type matrixWriter110111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
	capReaderFrom
	capStringWriter
}
type matrixWriter111000 struct {
	matrixCore
	capCloseNotifier
	capReaderFrom
	capStringWriter
}
type matrixWriter111001 struct {
	matrixCore
	capFlusher
	capCloseNotifier
	capReaderFrom
	capStringWriter
}
type matrixWriter111010 struct {
	matrixCore
	capHijacker
	capCloseNotifier
	capReaderFrom
	capStringWriter
}
type matrixWriter111011 struct {
	matrixCore
	capFlusher
	capHijacker
	capCloseNotifier
	capReaderFrom
	capStringWriter
}
type matrixWriter111100 struct {
	matrixCore
	capPusher
	capCloseNotifier
	capReaderFrom
	capStringWriter
}
type matrixWriter111101 struct {
	matrixCore
	capFlusher
	capPusher
	capCloseNotifier
	capReaderFrom
	capStringWriter
}
type matrixWriter111110 struct {
	matrixCore
	capHijacker
	capPusher
	capCloseNotifier
	capReaderFrom
	capStringWriter
}
type matrixWriter111111 struct {
	matrixCore
	capFlusher
	capHijacker
	capPusher
	capCloseNotifier
	capReaderFrom
	capStringWriter
}

type baseWriter struct {
	baseFacet
	mask        uint8
	flush       flushFacet
	flushErr    flushErrorFacet
	hijack      hijackFacet
	push        pushFacet
	notify      notifyFacet
	readerFrom  readerFromFacet
	stringWrite stringWriterFacet
}

func (w *baseWriter) flushFacetEnabled() bool   { return w.mask&httpCapFlusher != 0 }
func (w *baseWriter) flushErrorEnabled() bool   { return w.mask&httpCapFlusher != 0 }
func (w *baseWriter) hijackEnabled() bool       { return w.mask&httpCapHijacker != 0 }
func (w *baseWriter) pushEnabled() bool         { return w.mask&httpCapPusher != 0 }
func (w *baseWriter) notifyEnabled() bool       { return w.mask&httpCapCloseNotifier != 0 }
func (w *baseWriter) readerFromEnabled() bool   { return w.mask&httpCapReaderFrom != 0 }
func (w *baseWriter) stringWriterEnabled() bool { return w.mask&httpCapStringWriter != 0 }

func (w *baseWriter) Flush() {
	if w.flushFacetEnabled() {
		w.flush.Flush()
	}
}

func (w *baseWriter) FlushError() error {
	if w.flushErrorEnabled() {
		return w.flushErr.FlushError()
	}
	return http.ErrNotSupported
}

func (w *baseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.hijackEnabled() {
		return w.hijack.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (w *baseWriter) Push(target string, opts *http.PushOptions) error {
	if w.pushEnabled() {
		return w.push.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (w *baseWriter) CloseNotify() <-chan bool {
	if w.notifyEnabled() {
		return w.notify.CloseNotify()
	}
	return make(chan bool)
}

func (w *baseWriter) ReadFrom(r io.Reader) (int64, error) {
	if w.readerFromEnabled() {
		return w.readerFrom.ReadFrom(r)
	}
	n, err := io.Copy(w.bodyWriter(), r)
	return n, err
}

func (w *baseWriter) WriteString(s string) (int, error) {
	if w.stringWriterEnabled() {
		return w.stringWrite.WriteString(s)
	}
	return w.Write([]byte(s))
}

func (b *baseFacet) bodyWriter() io.Writer { return &b.body }

func TestFacadeCapabilityMatrix(t *testing.T) {
	for mask := range 64 {
		mask := mask
		t.Run(maskName(uint8(mask)), func(t *testing.T) {
			logger := NewLogger(&bytes.Buffer{})
			diag := NewHTTPDiagnostics(logger, HTTPDiagnosticsOptions{})
			handler := diag.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Presence assertions inside the handler.
				_, isFlusher := w.(http.Flusher)
				if isFlusher != (uint8(mask)&httpCapFlusher != 0) {
					t.Errorf("Flusher presence = %v; want %v", isFlusher, uint8(mask)&httpCapFlusher != 0)
				}
				_, isHijacker := w.(http.Hijacker)
				if isHijacker != (uint8(mask)&httpCapHijacker != 0) {
					t.Errorf("Hijacker presence = %v; want %v", isHijacker, uint8(mask)&httpCapHijacker != 0)
				}
				_, isPusher := w.(http.Pusher)
				if isPusher != (uint8(mask)&httpCapPusher != 0) {
					t.Errorf("Pusher presence = %v; want %v", isPusher, uint8(mask)&httpCapPusher != 0)
				}
				_, isNotifier := w.(http.CloseNotifier)
				if isNotifier != (uint8(mask)&httpCapCloseNotifier != 0) {
					t.Errorf("CloseNotifier presence = %v; want %v", isNotifier, uint8(mask)&httpCapCloseNotifier != 0)
				}
				_, isReaderFrom := w.(io.ReaderFrom)
				if isReaderFrom != (uint8(mask)&httpCapReaderFrom != 0) {
					t.Errorf("ReaderFrom presence = %v; want %v", isReaderFrom, uint8(mask)&httpCapReaderFrom != 0)
				}
				_, isStringWriter := w.(io.StringWriter)
				if isStringWriter != (uint8(mask)&httpCapStringWriter != 0) {
					t.Errorf("StringWriter presence = %v; want %v", isStringWriter, uint8(mask)&httpCapStringWriter != 0)
				}

				// Exercise the supported surface with concrete effects.
				if _, err := w.Write([]byte("hello")); err != nil {
					t.Errorf("Write: %v", err)
				}
				if isFlusher {
					w.(http.Flusher).Flush()
				}
				if isHijacker {
					conn, buf, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("Hijack: %v", err)
					} else if conn == nil || buf == nil {
						t.Error("Hijack returned nil conn or buffer")
					}
				}
				if isPusher {
					if err := w.(http.Pusher).Push("/x", nil); err != nil {
						t.Errorf("Push: %v", err)
					}
				}
				if isNotifier {
					if w.(http.CloseNotifier).CloseNotify() == nil {
						t.Error("CloseNotify returned nil channel")
					}
				}
				if isReaderFrom {
					if _, err := w.(io.ReaderFrom).ReadFrom(strings.NewReader("payload")); err != nil {
						t.Errorf("ReadFrom: %v", err)
					}
				}
				if isStringWriter {
					if _, err := w.(io.StringWriter).WriteString("str"); err != nil {
						t.Errorf("WriteString: %v", err)
					}
				}
			}))
			r := httptest.NewRequest(http.MethodGet, "/livez", nil)
			handler.ServeHTTP(composedWriter(uint8(mask)), r)
		})
	}
}

func maskName(mask uint8) string {
	const digits = "01"
	name := make([]byte, 6)
	for i := 0; i < 6; i++ {
		if mask&(1<<i) != 0 {
			name[5-i] = '1'
		} else {
			name[5-i] = '0'
		}
	}
	_ = digits
	return string(name)
}

// TestOwnerFlushErrorSemantics pins error-aware flush behavior at the owner
// level: success establishes implicit 200, failure records evidence without
// inventing a status, and unsupported control returns ErrNotSupported with no
// phantom status.
func TestOwnerFlushErrorSemantics(t *testing.T) {
	t.Run("SuccessEstablishesImplicitOK", func(t *testing.T) {
		fe := &flushErrorFacet{}
		owner := &httpOwner{
			writer:    &baseWriter{baseFacet: newBaseFacet()},
			delegates: httpDelegates{flushError: fe},
		}
		if err := owner.flushError(); err != nil {
			t.Fatalf("flushError = %v; want nil", err)
		}
		if !owner.statusObserved || owner.status != http.StatusOK {
			t.Fatalf("observed status = %d/%v; want 200", owner.status, owner.statusObserved)
		}
	})
	t.Run("FailedErrorAwareFlushNoPhantomStatus", func(t *testing.T) {
		sentinel := errors.New("flush boom")
		fe := &flushErrorFacet{err: sentinel}
		owner := &httpOwner{
			writer:    &baseWriter{baseFacet: newBaseFacet()},
			delegates: httpDelegates{flushError: fe},
		}
		err := owner.flushError()
		if !errors.Is(err, sentinel) {
			t.Fatalf("flushError = %v; want sentinel", err)
		}
		if owner.statusObserved {
			t.Fatal("failed error-aware flush must not invent a status")
		}
		if owner.writeFailure == HTTPWriteNone {
			t.Fatal("failed flush must record write-failure evidence")
		}
	})
	t.Run("UnsupportedReturnsErrNotSupported", func(t *testing.T) {
		owner := &httpOwner{
			writer: &baseWriter{baseFacet: newBaseFacet()},
		}
		err := owner.flushError()
		if !errors.Is(err, http.ErrNotSupported) {
			t.Fatalf("flushError = %v; want ErrNotSupported", err)
		}
		if owner.statusObserved {
			t.Fatal("unsupported flush must not commit a status")
		}
	})
}

// TestOwnerReadFromAndHijackObservation pins count/status/hijack semantics.
func TestOwnerReadFromAndHijackObservation(t *testing.T) {
	t.Run("PositiveReadFromImplicitOK", func(t *testing.T) {
		rf := &readerFromFacet{n: 5}
		owner := &httpOwner{
			writer:    &baseWriter{baseFacet: newBaseFacet()},
			delegates: httpDelegates{readerFrom: rf},
		}
		if _, err := owner.readFrom(strings.NewReader("12345")); err != nil {
			t.Fatalf("readFrom: %v", err)
		}
		if owner.status != http.StatusOK || owner.bytes != 5 {
			t.Fatalf("status/bytes = %d/%d; want 200/5", owner.status, owner.bytes)
		}
	})
	t.Run("ZeroReadFromNoInventedStatus", func(t *testing.T) {
		rf := &readerFromFacet{}
		owner := &httpOwner{
			writer:    &baseWriter{baseFacet: newBaseFacet()},
			delegates: httpDelegates{readerFrom: rf},
		}
		if _, err := owner.readFrom(strings.NewReader("")); err != nil {
			t.Fatalf("readFrom: %v", err)
		}
		if owner.statusObserved {
			t.Fatal("zero ReadFrom must not invent a status")
		}
	})
	t.Run("HijackWithoutObservedStatusStaysUnavailable", func(t *testing.T) {
		hj := &hijackFacet{}
		owner := &httpOwner{
			writer:    &baseWriter{baseFacet: newBaseFacet()},
			delegates: httpDelegates{hijacker: hj},
		}
		if _, _, err := owner.hijack(); err != nil {
			t.Fatalf("hijack: %v", err)
		}
		if !owner.hijacked {
			t.Fatal("hijack flag not set")
		}
		if owner.statusObserved {
			t.Fatal("hijack without observed status must stay unobserved")
		}
	})
}
