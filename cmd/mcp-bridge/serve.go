package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/auth"
	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/service"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
)

// readHeaderTimeout bounds the header phase of every listener; shutdownTimeout
// bounds the listener drain once the run context ends (plan §4.12).
const (
	readHeaderTimeout = 15 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// serve is the composition root: it turns one operator document into the
// serving bridge of plan §4.2 — the inbound edge with its bounds and tokens,
// the local catalog, the operations listener, the refresh/reload loop, and the
// reload signal edge.
func serve(ctx context.Context, path string, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := observe.NewLogger(stdout)
	holder := newConfigHolder()
	load := func() (*config.Config, error) {
		candidate, err := config.ParseFile(path)
		if err != nil {
			return nil, err
		}
		settings, err := candidate.Command()
		if err != nil {
			return nil, err
		}
		if published := holder.publishedSettings(); published != nil {
			if field := restartOnlyField(*published, settings); field != "" {
				// The restart-only guard is a typed record (spec §3.1 E), so
				// operatorLog attributes it on reload: the candidate path, the
				// full dotted key, the restart_required class, and a static
				// reason — never a setting value. The rejected reload keeps
				// the working generation staged.
				return nil, &config.Diagnostics{Records: []config.Diagnostic{{
					Path:     path,
					Key:      field,
					Code:     "restart_required",
					Property: "setting",
					Shape:    "immutable",
					Reason:   "setting can only change across a restart",
				}}}
			}
		}
		holder.stage(candidate, settings)
		return candidate, nil
	}
	candidate, err := load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	settings := holder.settings()
	authCfg := auth.NewConfigFromEnv()
	diagnostics := observe.NewHTTPDiagnostics(logger, observe.HTTPDiagnosticsOptions{
		Now: diagnosticNowFromContext(ctx),
		NewID: diagnosticNewIDFromContext(ctx),
		Provenance: buildDiagnosticProvenance(
			authCfg, len(candidate.ClientTokenHashes()) > 0,
		),
	})
	metrics := observe.NewMetrics(configuredServerIDs(candidate))

	store, err := storage.Open(settings.StorageDirectory)
	if err != nil {
		return fmt.Errorf("state directory %q: %w", settings.StorageDirectory, err)
	}
	defer store.Close()

	jwksCache, err := auth.NewJWKSCache(ctx, authCfg.PocketIDJWKSURL)
	if err != nil {
		return fmt.Errorf("jwks cache: %w", err)
	}

	runtime := service.NewRuntime()
	readiness := service.NewReadinessState(true, true, true, false)
	checks := newLocalChecks(readiness, true)

	model, provider, err := newEmbeddingProvider(candidate, settings.Embedding)
	if err != nil {
		return fmt.Errorf("embedding provider: %w", err)
	}
	surface, err := service.NewToolSurface(runtime, service.ToolSurfaceConfig{
		Search:   searchSettings(settings.Search),
		Provider: provider,
		Model:    model,
		Logger:   logger,
	})
	if err != nil {
		return fmt.Errorf("tool surface: %w", err)
	}
	server := service.NewMCPServer()
	if err := surface.Register(server); err != nil {
		return fmt.Errorf("tool surface: %w", err)
	}
	promptSurface := service.NewPromptSurface(runtime, func(message string) { operatorLog(logger)("%s", message) })
	promptSurface.SetMetrics(metrics)
	promptSurface.Register(server)

	registry := newUpstreamRegistry(holder)
	published := publishObserver(holder, checks, readiness, metrics)
	persist := func(snapshot *storage.Snapshot) error {
		if err := store.Save(snapshot); err != nil {
			checks.storage(false)
			return err
		}
		checks.storage(true)
		return nil
	}
	persistPrompts := func(snapshot *storage.PromptSnapshot) error {
		if err := store.SavePrompts(snapshot); err != nil {
			checks.storage(false)
			return err
		}
		checks.storage(true)
		return nil
	}
	inbound := service.NewInboundHandler(service.NewStreamableHandler(server), candidate.ClientTokenHashes(), service.InboundOptions{
		Logger:          logger,
		Metrics:         metrics,
		Diagnostics:     diagnostics,
		MaxRequestBytes: settings.MaxRequestBytes,
		MaxSessions:     settings.MaxSessions,
		SessionIdle:     settings.SessionIdle,
		AllowedHosts:    settings.AllowedHosts,
		AllowedOrigins:  settings.AllowedOrigins,
		Bound:           holder.bounds,
		Tokens: func() [][32]byte {
			assembly := runtime.Current()
			if assembly == nil {
				return nil
			}
			return assembly.ClientTokenHashes()
		},
	})

	inboundMux := http.NewServeMux()
	inboundMux.Handle("/.well-known/oauth-protected-resource", auth.NewProtectedResourceHandler(authCfg))
	inboundMux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{RouteOutcome: "health"})
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("live\n"))
	})
	inboundMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		observe.ContributeHTTP(r.Context(), observe.HTTPDecision{RouteOutcome: "health"})
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	inboundMux.Handle("/readyz", service.NewOperationsHandler(readiness, nil))
	authMiddleware := auth.NewAuthMiddleware(authCfg, jwksCache, auth.WithStaticFallback(true), auth.WithNow(diagnosticNowFromContext(ctx)))
	inboundMux.Handle("/mcp", authMiddleware(inbound))
	inboundMux.Handle("/", inbound)

	reloads := make(chan struct{}, 1)
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go pumpReloads(ctx, hup, reloads)

	loop, err := service.NewLoop(runtime, service.LoopOptions{
		Load:    load,
		Durable: durableLoader(store),
		Deps: service.RefreshDeps{ReloadDeps: service.ReloadDeps{
			Discover:            registry.discover,
			DiscoverPrompts:     registry.discoverPrompts,
			NewClient:           registry.newClient,
			Persist:             persist,
			LoadPrompts:         promptLoader(store),
			PersistPrompts:      persistPrompts,
			Embed:               embedder(provider, model, settings.Embedding),
			Close:               registry.close,
			Now:                 time.Now,
			ModelIdentityDigest: model.Digest(),
		}},
		Interval:      settings.RefreshInterval,
		Signals:       reloads,
		Metrics:       func(serverID, status, reason string) { metrics.RecordRefresh(serverID, status, reason) },
		ReloadMetrics: func(outcome, reason string) { metrics.RecordReload(outcome, orNone(reason)) },
		Published: func(assembly *service.Assembly) {
			promptSurface.Published(assembly)
			surface.Published(assembly)
			published(assembly)
		},
		Logf: operatorLog(logger),
	})
	if err != nil {
		return fmt.Errorf("catalog loop: %w", err)
	}

	inboundListener, err := net.Listen("tcp", settings.HTTPListen)
	if err != nil {
		return fmt.Errorf("inbound listener %q: %w", settings.HTTPListen, err)
	}
	defer inboundListener.Close()
	operationsListener, err := net.Listen("tcp", settings.OperationsListen)
	if err != nil {
		return fmt.Errorf("operations listener %q: %w", settings.OperationsListen, err)
	}
	defer operationsListener.Close()

	inboundServer := &http.Server{Handler: diagnostics.Wrap(inboundMux), ReadHeaderTimeout: readHeaderTimeout}
	operationsServer := &http.Server{
		Handler: service.RateLimit(
			service.NewOperationsHandler(readiness, metrics.Handler()),
			settings.OpsRequestsPerSecond,
			settings.OpsBurst,
		),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	loopErr := make(chan error, 1)
	go func() { loopErr <- loop.Start(ctx) }()
	listenerErr := make(chan error, 2)
	for _, running := range []struct {
		server   *http.Server
		listener net.Listener
	}{
		{inboundServer, inboundListener},
		{operationsServer, operationsListener},
	} {
		go func(running struct {
			server   *http.Server
			listener net.Listener
		}) {
			listenerErr <- serveListener(running.server, running.listener)
		}(running)
	}

	var failure error
	drained := false
	select {
	case failure = <-loopErr:
		drained = true
	case failure = <-listenerErr:
	case <-ctx.Done():
	}

	// Drain the listeners and the loop concurrently under one budget: an open
	// bridged stream can keep Shutdown waiting, and serializing the two would
	// spend the whole budget on the listener before the loop is observed.
	drainCtx, cancelDrain := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelDrain()
	var draining sync.WaitGroup
	draining.Add(1)
	go func() {
		defer draining.Done()
		var servers sync.WaitGroup
		for _, current := range []*http.Server{inboundServer, operationsServer} {
			servers.Add(1)
			go func(server *http.Server) {
				defer servers.Done()
				// A bridged session holds a long-lived stream, so waiting for the
				// listener to go idle would never finish: Close stops accepting and
				// releases those streams first, and Shutdown then drains whatever
				// request was still in flight.
				_ = server.Close()
				_ = server.Shutdown(drainCtx)
			}(current)
		}
		servers.Wait()
	}()
	if !drained {
		draining.Add(1)
		go func() {
			defer draining.Done()
			select {
			case err := <-loopErr:
				failure = firstError(failure, err)
			case <-drainCtx.Done():
				failure = firstError(failure, errors.New("the catalog loop did not drain"))
			}
		}()
	}
	draining.Wait()
	return failure
}

// serveListener serves one listener and reports whether it stopped for a reason
// other than the process shutting it down.
func serveListener(server *http.Server, listener net.Listener) error {
	err := server.Serve(listener)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// firstError reports the first non-nil error so the shutdown path never hides
// the failure that ended the process.
func firstError(first, second error) error {
	if first != nil {
		return first
	}
	return second
}
