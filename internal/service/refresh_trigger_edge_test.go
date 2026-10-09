package service

import (
	"context"
	"testing"
	"time"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

func TestRefresherTriggerRunsPassWithPollingDisabled(t *testing.T) {
	refresher := NewRefresher(NewRuntime(), func() *config.Config { return &config.Config{} }, RefreshDeps{ReloadDeps: ReloadDeps{
		Discover: func(config.Server) ([]core.ToolDefinition, error) { return nil, nil },
		Now:      time.Now,
	}}, RefresherOptions{Interval: -time.Second})
	if refresher.ticks() != nil {
		t.Fatal("disabled polling exposed a tick channel")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { refresher.Run(ctx); close(done) }()
	refresher.Trigger()
	deadline := time.NewTimer(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for refresher.Passes() == 0 {
		select {
		case <-ticker.C:
		case <-deadline.C:
			cancel()
			t.Fatal("trigger did not complete a refresh pass")
		}
	}
	if got := refresher.Passes(); got != 1 {
		cancel()
		t.Fatalf("completed passes = %d, want 1", got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("refresher did not exit after cancellation")
	}
}
