package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"

	"github.com/jjohnson83853/mcp-bridge/internal/observe"
	"github.com/jjohnson83853/mcp-bridge/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type promptFetcher interface {
	GetPrompt(context.Context, string, string, map[string]string) (*mcp.GetPromptResult, error)
}

type PromptSurface struct {
	mu          sync.Mutex
	server      *mcp.Server
	runtime     *Runtime
	definitions map[string]storage.Prompt
	logf        func(string)
	metrics     *observe.Metrics
}

// SetMetrics attaches the metrics surface the publication reports its count to.
// Wiring happens once before serving; the gauge carries no prompt or server
// label (R-27/AC-29).
func (p *PromptSurface) SetMetrics(metrics *observe.Metrics) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metrics = metrics
}

func NewPromptSurface(runtime *Runtime, logf ...func(string)) *PromptSurface {
	var log func(string)
	if len(logf) > 0 {
		log = logf[0]
	}
	return &PromptSurface{runtime: runtime, definitions: make(map[string]storage.Prompt), logf: log}
}

func (p *PromptSurface) Register(server *mcp.Server) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.server = server
}

// Published reconciles the inbound SDK prompt registry with one immutable publication.
func (p *PromptSurface) Published(assembly *Assembly) {
	if assembly == nil {
		return
	}
	published := assembly.Prompts()
	next := make(map[string]storage.Prompt, len(published))
	for _, prompt := range published {
		if _, exists := next[prompt.PublicName]; exists {
			p.report("duplicate public name")
			return
		}
		next[prompt.PublicName] = prompt
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.server == nil {
		return
	}
	valid := make(map[string]storage.Prompt, len(next))
	for name, definition := range next {
		if len(definition.Arguments) > 0 {
			var args []*mcp.PromptArgument
			if err := json.Unmarshal(definition.Arguments, &args); err != nil {
				p.report("invalid stored argument schema")
				continue
			}
		}
		valid[name] = definition
	}
	next = make(map[string]storage.Prompt, len(valid))
	for name, definition := range valid {
		next[name] = definition
	}
	for name, previous := range p.definitions {
		current, exists := next[name]
		if !exists {
			p.server.RemovePrompts(name)
			continue
		}
		if reflect.DeepEqual(previous, current) {
			delete(next, name)
		}
	}
	for name, definition := range next {
		prompt := &mcp.Prompt{Name: name, Title: definition.Title, Description: promptDescription(definition)}
		if len(definition.Arguments) > 0 && json.Unmarshal(definition.Arguments, &prompt.Arguments) != nil {
			continue
		}
		captured := definition
		p.server.AddPrompt(prompt, func(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return p.get(ctx, request, captured)
		})
	}
	p.definitions = valid
	if p.metrics != nil {
		p.metrics.SetCatalogPrompts(len(valid))
	}
}

// promptDescription keeps the upstream description and adds the human-readable
// provenance a client needs to tell two colliding public names apart. Only the
// server id is added: the raw upstream name stays out of the wire definition
// and remains private routing provenance (R-25/AC-27).
func promptDescription(definition storage.Prompt) string {
	provenance := "via " + definition.ServerID
	if definition.Description == "" {
		return provenance
	}
	return definition.Description + " (" + provenance + ")"
}

func (p *PromptSurface) report(reason string) {
	if p.logf != nil {
		p.logf("prompt publication rejected: " + reason)
	}
}

func (p *PromptSurface) get(ctx context.Context, request *mcp.GetPromptRequest, definition storage.Prompt) (*mcp.GetPromptResult, error) {
	if request == nil || request.Params == nil {
		return nil, errors.New("invalid prompt request")
	}
	assembly := p.runtime.Current()
	if assembly == nil {
		return nil, errors.New("prompt unavailable")
	}
	server, exists := assembly.Server(definition.ServerID)
	if !exists {
		return nil, errors.New("prompt unavailable")
	}
	fetcher, exists := server.Client.(promptFetcher)
	if !exists {
		return nil, errors.New("prompt unavailable")
	}
	result, err := fetcher.GetPrompt(ctx, definition.ServerID, definition.RawName, request.Params.Arguments)
	if err != nil {
		return nil, errors.New("prompt unavailable")
	}
	return result, nil
}
