package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jjohnson83853/mcp-bridge/internal/config"
	"github.com/jjohnson83853/mcp-bridge/internal/core"
)

type projectionOwnerKind uint8

const (
	projectionPublisher projectionOwnerKind = iota + 1
	projectionSession
)

// projectionOwner is process-local identity. Live request owners always pair
// the trusted SDK session ID with a unique epoch so reused session IDs cannot
// inherit an earlier projection.
type projectionOwner struct {
	kind      projectionOwnerKind
	sessionID string
	epoch     uint64
}

type logicalToolTarget struct {
	serverID string
	rawName  string
}

type projectionToolSpec struct {
	name       string
	binding    *core.Binding
	interested bool
}

type projectedTool struct {
	target        logicalToolTarget
	binding       *core.Binding
	inputSchema   json.RawMessage
	descriptor    *mcp.Tool
	descriptorKey []byte
}

// toolProjection is an immutable, name-sorted view. Its list, direct binding
// lookup and schema lookup are prepared together at the ownership boundary.
type toolProjection struct {
	owner             projectionOwner
	generation        uint64
	runtimeGeneration uint64
	tools             []*mcp.Tool
	byName            map[string]projectedTool
	interests         map[logicalToolTarget]struct{}
}

type unionTool struct {
	descriptor    *mcp.Tool
	descriptorKey []byte
	target        logicalToolTarget
	references    map[projectionOwner]struct{}
}

type registryDelta struct {
	remove []string
	add    []*mcp.Tool
}

type stagedProjectionReconciliation struct {
	projections        map[projectionOwner]*toolProjection
	union              map[string]*unionTool
	interestReferences map[logicalToolTarget]map[projectionOwner]struct{}
	delta              registryDelta
}

type projectionCoordinator struct {
	// barrier serializes runtime acceptance, complete projection swaps and
	// lease acquisition. publicationMu also serializes SDK registry writers,
	// whose calls deliberately run after barrier is released.
	barrier       sync.Mutex
	publicationMu sync.Mutex
	// testDuringProjectionPreparation pauses staged projection work for concurrency tests.
	testDuringProjectionPreparation func()
	// testBeforeRegistryAdd pauses immediately before SDK registration additions.
	testBeforeRegistryAdd func()
	// testAfterDirectCapture pauses after the admitted tuple owns its lease.
	testAfterDirectCapture func()

	nextEpoch          uint64
	acceptedKnown      bool
	acceptedGeneration uint64
	liveOwners         map[projectionOwner]struct{}
	projections        map[projectionOwner]*toolProjection
	union              map[string]*unionTool
	interestReferences map[logicalToolTarget]map[projectionOwner]struct{}
	server             *mcp.Server
}

func newProjectionCoordinator() *projectionCoordinator {
	return &projectionCoordinator{
		liveOwners:         make(map[projectionOwner]struct{}),
		projections:        make(map[projectionOwner]*toolProjection),
		union:              make(map[string]*unionTool),
		interestReferences: make(map[logicalToolTarget]map[projectionOwner]struct{}),
	}
}

func (r *Runtime) progressiveCoordinator() *projectionCoordinator {
	r.coordinatorOnce.Do(func() {
		if r.coordinator == nil {
			r.coordinator = newProjectionCoordinator()
		}
	})
	return r.coordinator
}

var (
	errRuntimeGenerationChanged   = errors.New("service: runtime generation changed while candidate was prepared")
	errAcceptedGenerationMismatch = errors.New("service: accepted generation does not match runtime")
)

// validateAcceptance checks the same generation invariants as accept before
// build persists the candidate. Candidate commits are serialized separately.
func (r *Runtime) validateAcceptance(previous *Assembly) error {
	c := r.progressiveCoordinator()
	c.barrier.Lock()
	defer c.barrier.Unlock()

	current := r.current.Load()
	if current != previous {
		return errRuntimeGenerationChanged
	}
	if current != nil && c.acceptedKnown && c.acceptedGeneration != current.Generation() {
		return errAcceptedGenerationMismatch
	}
	return nil
}

func (r *Runtime) accept(next, previous *Assembly) error {
	if next == nil {
		return errors.New("service: cannot accept an empty assembly")
	}
	c := r.progressiveCoordinator()
	c.barrier.Lock()
	defer c.barrier.Unlock()

	current := r.current.Load()
	if current != previous {
		return errRuntimeGenerationChanged
	}
	if current != nil && c.acceptedKnown && c.acceptedGeneration != current.Generation() {
		return errAcceptedGenerationMismatch
	}

	nextGeneration := uint64(1)
	if current != nil {
		nextGeneration = current.Generation() + 1
	}
	next.generation = nextGeneration
	c.acceptedKnown = true
	c.acceptedGeneration = nextGeneration
	r.current.Store(next)
	return nil
}

func (t *ToolSurface) projectionCoordinator() *projectionCoordinator {
	t.projectionOnce.Do(func() {
		if t.coordinator == nil {
			t.coordinator = t.runtime.progressiveCoordinator()
		}
		if t.publisherOwner.epoch == 0 {
			t.publisherOwner = t.coordinator.newPublisher()
		}
	})
	return t.coordinator
}

func (c *projectionCoordinator) newPublisher() projectionOwner {
	c.barrier.Lock()
	defer c.barrier.Unlock()
	c.nextEpoch++
	owner := projectionOwner{kind: projectionPublisher, epoch: c.nextEpoch}
	c.liveOwners[owner] = struct{}{}
	return owner
}

func (c *projectionCoordinator) newLiveOwner(sessionID string) (projectionOwner, error) {
	if sessionID == "" {
		return projectionOwner{}, errors.New("service: progressive owner needs a trusted session ID")
	}
	c.barrier.Lock()
	defer c.barrier.Unlock()
	c.nextEpoch++
	owner := projectionOwner{kind: projectionSession, sessionID: sessionID, epoch: c.nextEpoch}
	c.liveOwners[owner] = struct{}{}
	return owner, nil
}

func (c *projectionCoordinator) ownerProjection(owner projectionOwner) *toolProjection {
	c.barrier.Lock()
	defer c.barrier.Unlock()
	return c.projections[owner]
}

func (c *projectionCoordinator) accepted(generation uint64) bool {
	c.barrier.Lock()
	defer c.barrier.Unlock()
	return !c.acceptedKnown || c.acceptedGeneration == generation
}

func (c *projectionCoordinator) superseded(generation uint64) bool {
	c.barrier.Lock()
	defer c.barrier.Unlock()
	return c.acceptedKnown && c.acceptedGeneration > generation
}

func prepareProjection(owner projectionOwner, previous *toolProjection, runtimeGeneration uint64, specs []projectionToolSpec, interests map[logicalToolTarget]struct{}) (*toolProjection, error) {
	generation := uint64(1)
	if previous != nil {
		generation = previous.generation + 1
	}
	projection := &toolProjection{
		owner:             owner,
		generation:        generation,
		runtimeGeneration: runtimeGeneration,
		tools:             make([]*mcp.Tool, 0, len(specs)),
		byName:            make(map[string]projectedTool, len(specs)),
		interests:         cloneInterests(interests),
	}
	for _, spec := range specs {
		if spec.binding == nil {
			return nil, fmt.Errorf("service: progressive tool %q has no binding", spec.name)
		}
		if spec.name == "" {
			return nil, errors.New("service: progressive tool has an empty exposed name")
		}
		if _, exists := projection.byName[spec.name]; exists {
			return nil, fmt.Errorf("service: progressive projection repeats tool %q", spec.name)
		}
		binding := cloneProjectionBinding(spec.binding)
		descriptor, err := sdkToolDescriptor(spec.name, binding.Def)
		if err != nil {
			return nil, err
		}
		target := logicalToolTarget{serverID: binding.ServerID, rawName: binding.RawName}
		projection.byName[spec.name] = projectedTool{
			target:        target,
			binding:       binding,
			inputSchema:   binding.Def.InputSchema,
			descriptor:    descriptor,
			descriptorKey: marshalToolDescriptor(descriptor),
		}
		projection.tools = append(projection.tools, descriptor)
		if spec.interested {
			projection.interests[target] = struct{}{}
		}
	}
	sort.Slice(projection.tools, func(i, j int) bool {
		return projection.tools[i].Name < projection.tools[j].Name
	})
	return projection, nil
}

func cloneProjectionBinding(source *core.Binding) *core.Binding {
	binding := *source
	binding.Def.InputSchema = append(json.RawMessage(nil), source.Def.InputSchema...)
	binding.Def.OutputSchema = append(json.RawMessage(nil), source.Def.OutputSchema...)
	binding.Def.Metadata = source.Def.Metadata.Clone()
	return &binding
}

func cloneInterests(source map[logicalToolTarget]struct{}) map[logicalToolTarget]struct{} {
	interests := make(map[logicalToolTarget]struct{}, len(source))
	for target := range source {
		interests[target] = struct{}{}
	}
	return interests
}

func preparePinProjection(owner projectionOwner, previous *toolProjection, assembly *Assembly, catalogBindings map[logicalToolTarget]*core.Binding) (*toolProjection, error) {
	if assembly == nil {
		return nil, errors.New("service: progressive publication needs an assembly")
	}
	specs := make([]projectionToolSpec, 0, len(assembly.pins))
	pinnedTargets := make(map[logicalToolTarget]struct{}, len(assembly.pins))
	for i := range assembly.pins {
		pin := &assembly.pins[i]
		if pin.Binding == nil {
			continue
		}
		target := logicalToolTarget{serverID: pin.Binding.ServerID, rawName: pin.Binding.RawName}
		pinnedTargets[target] = struct{}{}
		specs = append(specs, projectionToolSpec{
			name:    config.WireToolName(pin.PinnedTool.ServerID, pin.PinnedTool.RawName),
			binding: pin.Binding,
		})
	}
	interests := make(map[logicalToolTarget]struct{})
	if previous != nil {
		for interest := range previous.interests {
			binding, present := catalogBindings[interest]
			if !present {
				continue
			}
			interests[interest] = struct{}{}
			if _, pinned := pinnedTargets[interest]; !pinned {
				if metadata, hasMetadata := assembly.toolMetadata[interest.serverID]; hasMetadata {
					toolPatch := metadata.tools[interest.rawName]
					if len(metadata.defaults) > 0 || len(toolPatch) > 0 {
						view, err := deriveToolMetadata(binding.Def, metadata.defaults, toolPatch)
						if err != nil {
							return nil, fmt.Errorf("service: progressive metadata for [server=%q,tool=%q]: %w",
								interest.serverID, interest.rawName, err)
						}
						effective := *binding
						effective.Def = view.Definition
						binding = &effective
					}
				}
				specs = append(specs, projectionToolSpec{
					name:    core.PublicName(interest.serverID, interest.rawName),
					binding: binding,
				})
			}
		}
	}
	return prepareProjection(owner, previous, assembly.Generation(), specs, interests)
}

func (t *ToolSurface) discoveryOwnerValid(owner projectionOwner, assembly *Assembly) bool {
	if (owner.kind != projectionSession && owner.kind != projectionPublisher) || assembly == nil {
		return false
	}
	coordinator := t.projectionCoordinator()
	coordinator.barrier.Lock()
	defer coordinator.barrier.Unlock()
	projection := coordinator.projections[owner]
	_, live := coordinator.liveOwners[owner]
	return t.runtime.current.Load() == assembly &&
		coordinator.acceptedKnown &&
		coordinator.acceptedGeneration == assembly.Generation() &&
		live &&
		projection != nil &&
		projection.owner == owner &&
		projection.runtimeGeneration == assembly.Generation()
}

func (t *ToolSurface) commitDiscovery(owner projectionOwner, assembly *Assembly, targets map[logicalToolTarget]struct{}) error {
	if assembly == nil {
		return errAcceptedGenerationMismatch
	}
	coordinator := t.projectionCoordinator()
	coordinator.publicationMu.Lock()
	defer coordinator.publicationMu.Unlock()
	if !t.discoveryOwnerValid(owner, assembly) {
		return errAcceptedGenerationMismatch
	}
	if owner.kind == projectionPublisher {
		return nil
	}
	if owner.kind != projectionSession {
		return errAcceptedGenerationMismatch
	}

	generation := assembly.Generation()
	coordinator.barrier.Lock()
	previous := coordinator.projections[owner]
	previousProjections := coordinator.projections
	previousUnion := coordinator.union
	previousInterestReferences := coordinator.interestReferences
	coordinator.barrier.Unlock()
	if len(targets) == 0 {
		return nil
	}
	if assembly.catalog == nil {
		return errors.New("service: progressive discovery needs a catalog")
	}

	bindings := assembly.catalog.Bindings()
	catalogBindings := make(map[logicalToolTarget]*core.Binding, len(bindings))
	for i := range bindings {
		binding := &bindings[i]
		target := logicalToolTarget{serverID: binding.ServerID, rawName: binding.RawName}
		catalogBindings[target] = binding
	}
	interests := cloneInterests(previous.interests)
	for target := range targets {
		if target.serverID == "" || target.rawName == "" {
			return errors.New("service: progressive discovery has an incomplete tool identity")
		}
		if _, present := catalogBindings[target]; !present {
			return fmt.Errorf("service: progressive discovery target [server=%q,tool=%q] is not in the accepted catalog", target.serverID, target.rawName)
		}
		interests[target] = struct{}{}
	}
	base := *previous
	base.interests = interests
	next, err := preparePinProjection(owner, &base, assembly, catalogBindings)
	if err != nil {
		return err
	}
	if next.owner != owner || next.runtimeGeneration != generation {
		return errAcceptedGenerationMismatch
	}
	if sameProjection(previous, next) {
		return nil
	}
	if err := validateProjectionDescriptors(changedProjectionDescriptors(previous, next)); err != nil {
		return err
	}

	staged, err := coordinator.stageProjectionReconciliation(
		[]projectionOwner{owner},
		previousProjections,
		previousUnion,
		previousInterestReferences,
		map[projectionOwner]*toolProjection{owner: next},
	)
	if err != nil {
		return err
	}
	if err := validateProjectionDescriptors(staged.delta.add); err != nil {
		return err
	}
	for _, name := range staged.delta.remove {
		if name == "" {
			return errors.New("service: progressive discovery produced an empty registry removal")
		}
	}

	coordinator.barrier.Lock()
	if t.runtime.current.Load() != assembly ||
		!coordinator.acceptedKnown ||
		coordinator.acceptedGeneration != generation ||
		coordinator.projections[owner] != previous {
		coordinator.barrier.Unlock()
		return errAcceptedGenerationMismatch
	}
	if _, live := coordinator.liveOwners[owner]; !live {
		coordinator.barrier.Unlock()
		return errAcceptedGenerationMismatch
	}
	coordinator.projections = staged.projections
	coordinator.union = staged.union
	coordinator.interestReferences = staged.interestReferences
	delta := staged.delta
	coordinator.barrier.Unlock()

	t.applyRegistryDelta(delta)
	return nil
}

// stageProjectionReconciliation builds replacement projection maps without holding the admission barrier.
func (c *projectionCoordinator) stageProjectionReconciliation(
	owners []projectionOwner,
	previousProjections map[projectionOwner]*toolProjection,
	previousUnion map[string]*unionTool,
	previousInterestReferences map[logicalToolTarget]map[projectionOwner]struct{},
	next map[projectionOwner]*toolProjection,
) (*stagedProjectionReconciliation, error) {
	staged := &projectionCoordinator{
		liveOwners:         make(map[projectionOwner]struct{}, len(owners)),
		projections:        make(map[projectionOwner]*toolProjection, len(previousProjections)),
		union:              make(map[string]*unionTool, len(previousUnion)),
		interestReferences: make(map[logicalToolTarget]map[projectionOwner]struct{}, len(previousInterestReferences)),
	}
	for _, owner := range owners {
		staged.liveOwners[owner] = struct{}{}
	}
	for owner, projection := range previousProjections {
		staged.projections[owner] = projection
	}
	for name, entry := range previousUnion {
		references := make(map[projectionOwner]struct{}, len(entry.references))
		for owner := range entry.references {
			references[owner] = struct{}{}
		}
		staged.union[name] = &unionTool{
			descriptor:    entry.descriptor,
			descriptorKey: entry.descriptorKey,
			target:        entry.target,
			references:    references,
		}
	}
	for interest, owners := range previousInterestReferences {
		references := make(map[projectionOwner]struct{}, len(owners))
		for owner := range owners {
			references[owner] = struct{}{}
		}
		staged.interestReferences[interest] = references
	}

	if c.testDuringProjectionPreparation != nil {
		c.testDuringProjectionPreparation()
	}
	for owner := range staged.liveOwners {
		if staged.projections[owner] == nil {
			continue
		}
		if _, err := staged.replaceProjectionLocked(owner, nil); err != nil {
			return nil, err
		}
	}
	for owner, projection := range next {
		if _, err := staged.replaceProjectionLocked(owner, projection); err != nil {
			return nil, err
		}
	}
	return &stagedProjectionReconciliation{
		projections:        staged.projections,
		union:              staged.union,
		interestReferences: staged.interestReferences,
		delta:              projectionRegistryDelta(previousUnion, staged.union),
	}, nil
}

func projectionRegistryDelta(previous, next map[string]*unionTool) registryDelta {
	removed := make(map[string]struct{})
	added := make(map[string]*mcp.Tool)
	for name, old := range previous {
		current, exists := next[name]
		if !exists {
			removed[name] = struct{}{}
			continue
		}
		hasNewOwner := false
		for owner := range current.references {
			if _, was := old.references[owner]; !was {
				hasNewOwner = true
				break
			}
		}
		if old.target != current.target || !sameToolDescriptorKey(old.descriptorKey, current.descriptorKey) || hasNewOwner {
			added[name] = current.descriptor
		}
	}
	for name, current := range next {
		if _, exists := previous[name]; !exists {
			added[name] = current.descriptor
		}
	}

	delta := registryDelta{
		remove: make([]string, 0, len(removed)),
		add:    make([]*mcp.Tool, 0, len(added)),
	}
	for name := range removed {
		delta.remove = append(delta.remove, name)
	}
	sort.Strings(delta.remove)
	addedNames := make([]string, 0, len(added))
	for name := range added {
		addedNames = append(addedNames, name)
	}
	sort.Strings(addedNames)
	for _, name := range addedNames {
		delta.add = append(delta.add, added[name])
	}
	return delta
}

func changedProjectionDescriptors(previous, next *toolProjection) []*mcp.Tool {
	if next == nil {
		return nil
	}
	changed := make([]*mcp.Tool, 0, len(next.tools))
	for _, descriptor := range next.tools {
		current, exists := projectedToolAt(previous, descriptor.Name)
		candidate := next.byName[descriptor.Name]
		if !exists || !sameProjectedTool(current, candidate) {
			changed = append(changed, descriptor)
		}
	}
	return changed
}

func projectedToolAt(projection *toolProjection, name string) (projectedTool, bool) {
	if projection == nil {
		return projectedTool{}, false
	}
	tool, ok := projection.byName[name]
	return tool, ok
}

func sameProjectedTool(left, right projectedTool) bool {
	return left.target == right.target &&
		canonicalJSON(left.inputSchema) == canonicalJSON(right.inputSchema) &&
		sameToolDescriptorKey(left.descriptorKey, right.descriptorKey)
}

func marshalToolDescriptor(descriptor *mcp.Tool) []byte {
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return nil
	}
	return encoded
}

func sameToolDescriptorKey(left, right []byte) bool {
	return left != nil && right != nil && bytes.Equal(left, right)
}

func canonicalJSON(raw json.RawMessage) string {
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return string(raw)
	}
	return compact.String()
}

func sameProjection(left, right *toolProjection) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.runtimeGeneration != right.runtimeGeneration ||
		len(left.byName) != len(right.byName) ||
		len(left.interests) != len(right.interests) {
		return false
	}
	for name, entry := range left.byName {
		other, ok := right.byName[name]
		if !ok || !sameProjectedTool(entry, other) {
			return false
		}
	}
	for interest := range left.interests {
		if _, ok := right.interests[interest]; !ok {
			return false
		}
	}
	return true
}

func (c *projectionCoordinator) replaceProjectionLocked(owner projectionOwner, next *toolProjection) (registryDelta, error) {
	if next != nil {
		if _, live := c.liveOwners[owner]; !live {
			return registryDelta{}, errors.New("service: progressive owner is no longer live")
		}
	}
	previous := c.projections[owner]
	if sameProjection(previous, next) {
		return registryDelta{}, nil
	}

	if next != nil {
		for name, candidate := range next.byName {
			if entry := c.union[name]; entry != nil {
				otherReferences := len(entry.references)
				if _, held := entry.references[owner]; held {
					otherReferences--
				}
				if otherReferences > 0 &&
					(entry.target != candidate.target || !sameToolDescriptorKey(entry.descriptorKey, candidate.descriptorKey)) {
					return registryDelta{}, fmt.Errorf("service: progressive name %q conflicts with another live owner", name)
				}
			}
		}
	}

	removed := make(map[string]struct{})
	if previous != nil {
		for name, oldTool := range previous.byName {
			newTool, remains := projectedTool{}, false
			if next != nil {
				newTool, remains = next.byName[name]
			}
			if remains && sameProjectedTool(oldTool, newTool) {
				continue
			}
			entry := c.union[name]
			if entry == nil {
				continue
			}
			delete(entry.references, owner)
			if len(entry.references) == 0 {
				delete(c.union, name)
				if !remains {
					removed[name] = struct{}{}
				}
			}
		}
		for interest := range previous.interests {
			if next != nil {
				if _, remains := next.interests[interest]; remains {
					continue
				}
			}
			references := c.interestReferences[interest]
			delete(references, owner)
			if len(references) == 0 {
				delete(c.interestReferences, interest)
			}
		}
	}

	added := make([]*mcp.Tool, 0)
	if next != nil {
		for _, descriptor := range next.tools {
			candidate := next.byName[descriptor.Name]
			oldTool, wasHeld := projectedToolAt(previous, descriptor.Name)
			if wasHeld && sameProjectedTool(oldTool, candidate) {
				continue
			}
			entry := c.union[descriptor.Name]
			if entry == nil {
				entry = &unionTool{
					descriptor:    candidate.descriptor,
					descriptorKey: candidate.descriptorKey,
					target:        candidate.target,
					references:    make(map[projectionOwner]struct{}),
				}
				c.union[descriptor.Name] = entry
			} else {
				entry.descriptor = candidate.descriptor
				entry.target = candidate.target
				entry.descriptorKey = candidate.descriptorKey
			}
			entry.references[owner] = struct{}{}
			// Re-register new owner references to use the SDK's standard
			// list-changed signal, even when another owner has this same name.
			added = append(added, candidate.descriptor)
		}
		for interest := range next.interests {
			if previous != nil {
				if _, alreadyHeld := previous.interests[interest]; alreadyHeld {
					continue
				}
			}
			references := c.interestReferences[interest]
			if references == nil {
				references = make(map[projectionOwner]struct{})
				c.interestReferences[interest] = references
			}
			references[owner] = struct{}{}
		}
	}

	if next == nil {
		delete(c.projections, owner)
	} else {
		c.projections[owner] = next
	}
	removedNames := make([]string, 0, len(removed))
	for name := range removed {
		removedNames = append(removedNames, name)
	}
	sort.Strings(removedNames)
	return registryDelta{remove: removedNames, add: added}, nil
}

func validationPanic(value any) (error, bool) {
	panicErr, ok := value.(error)
	if !ok {
		return nil, false
	}
	message := panicErr.Error()
	if !strings.HasPrefix(message, "AddTool ") {
		return nil, false
	}
	for _, detail := range []string{
		"missing input schema",
		"input schema is nil",
		"input schema must have type",
		"can't marshal input schema",
		"output schema is nil",
		"can't marshal output schema",
		"invalid parameter header annotations",
	} {
		if strings.Contains(message, detail) {
			return panicErr, true
		}
	}
	return nil, false
}

func validateProjectionDescriptors(descriptors []*mcp.Tool) (err error) {
	if len(descriptors) == 0 {
		return nil
	}
	validationServer := NewMCPServer()
	for _, descriptor := range descriptors {
		if err := validateProjectionDescriptor(validationServer, descriptor); err != nil {
			return err
		}
	}
	return nil
}

func validateProjectionDescriptor(server *mcp.Server, descriptor *mcp.Tool) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			validationErr, isValidationErr := validationPanic(recovered)
			if !isValidationErr {
				panic(recovered)
			}
			err = fmt.Errorf("service: SDK descriptor validation: %w", validationErr)
		}
	}()
	server.AddTool(descriptor, nil)
	return nil
}

func (c *projectionCoordinator) attachServer(server *mcp.Server) error {
	if server == nil {
		return errors.New("service: progressive registry needs an MCP server")
	}
	if c.server != nil && c.server != server {
		return errors.New("service: runtime already has a different progressive registry server")
	}
	c.server = server
	return nil
}

func (t *ToolSurface) newLiveOwner(sessionID string) (projectionOwner, error) {
	return t.projectionCoordinator().newLiveOwner(sessionID)
}

func (t *ToolSurface) publishOwnerProjection(owner projectionOwner, assembly *Assembly) error {
	if owner.kind != projectionSession || assembly == nil {
		return errors.New("service: invalid progressive owner publication")
	}
	return t.publishProjection(owner, assembly)
}

func (t *ToolSurface) retireLiveOwnerAtInboundLock(owner projectionOwner) bool {
	if owner.kind != projectionSession {
		return false
	}
	c := t.projectionCoordinator()
	c.barrier.Lock()
	defer c.barrier.Unlock()
	if _, live := c.liveOwners[owner]; !live {
		return false
	}
	delete(c.liveOwners, owner)
	return true
}

func (t *ToolSurface) cleanupRetiredOwner(owner projectionOwner) {
	if owner.kind != projectionSession {
		return
	}
	c := t.projectionCoordinator()
	c.publicationMu.Lock()
	defer c.publicationMu.Unlock()

	c.barrier.Lock()
	delta, err := c.replaceProjectionLocked(owner, nil)
	c.barrier.Unlock()
	if err == nil {
		t.applyRegistryDelta(delta)
	}
}

func (t *ToolSurface) retireLiveOwner(owner projectionOwner) bool {
	if !t.retireLiveOwnerAtInboundLock(owner) {
		return false
	}
	t.cleanupRetiredOwner(owner)
	return true
}

func (t *ToolSurface) publishProjection(owner projectionOwner, assembly *Assembly) error {
	c := t.projectionCoordinator()
	c.publicationMu.Lock()
	defer c.publicationMu.Unlock()

	generation := assembly.Generation()
	if !c.accepted(generation) {
		if c.superseded(generation) {
			return nil
		}
		return errors.New("service: progressive publication targets a non-accepted generation")
	}

	catalogBindings := make(map[logicalToolTarget]*core.Binding)
	if assembly.catalog != nil {
		bindings := assembly.catalog.Bindings()
		catalogBindings = make(map[logicalToolTarget]*core.Binding, len(bindings))
		for i := range bindings {
			binding := &bindings[i]
			catalogBindings[logicalToolTarget{serverID: binding.ServerID, rawName: binding.RawName}] = binding
		}
	}
	c.barrier.Lock()
	owners := make([]projectionOwner, 0, len(c.liveOwners))
	previous := make(map[projectionOwner]*toolProjection, len(c.liveOwners))
	for liveOwner := range c.liveOwners {
		previousProjection := c.projections[liveOwner]
		if liveOwner.kind == projectionSession && previousProjection == nil && liveOwner != owner {
			continue
		}
		owners = append(owners, liveOwner)
		previous[liveOwner] = previousProjection
	}
	previousProjections := c.projections
	previousUnion := c.union
	previousInterestReferences := c.interestReferences
	c.barrier.Unlock()

	next := make(map[projectionOwner]*toolProjection, len(owners))
	changedDescriptors := make(map[string]*mcp.Tool)
	needsReconcile := false
	for _, liveOwner := range owners {
		projection, err := preparePinProjection(liveOwner, previous[liveOwner], assembly, catalogBindings)
		if err != nil {
			return err
		}
		next[liveOwner] = projection
		if !sameProjection(previous[liveOwner], projection) {
			needsReconcile = true
		}
		for _, descriptor := range changedProjectionDescriptors(previous[liveOwner], projection) {
			changedDescriptors[descriptor.Name] = descriptor
		}
	}
	descriptorNames := make([]string, 0, len(changedDescriptors))
	for name := range changedDescriptors {
		descriptorNames = append(descriptorNames, name)
	}
	sort.Strings(descriptorNames)
	descriptors := make([]*mcp.Tool, 0, len(descriptorNames))
	for _, name := range descriptorNames {
		descriptors = append(descriptors, changedDescriptors[name])
	}
	if err := validateProjectionDescriptors(descriptors); err != nil {
		return err
	}

	var staged *stagedProjectionReconciliation
	if needsReconcile {
		var err error
		staged, err = c.stageProjectionReconciliation(
			owners,
			previousProjections,
			previousUnion,
			previousInterestReferences,
			next,
		)
		if err != nil {
			return err
		}
	}

	c.barrier.Lock()
	current := t.runtime.current.Load()
	if c.acceptedKnown && c.acceptedGeneration != generation {
		superseded := c.acceptedGeneration > generation
		c.barrier.Unlock()
		if superseded {
			return nil
		}
		return errors.New("service: progressive publication became stale before commit")
	}
	if current != nil && current.Generation() != generation {
		superseded := c.acceptedKnown && c.acceptedGeneration > generation
		c.barrier.Unlock()
		if superseded {
			return nil
		}
		return errors.New("service: progressive publication targets a superseded runtime")
	}
	if _, live := c.liveOwners[owner]; !live {
		c.barrier.Unlock()
		return errors.New("service: progressive owner is no longer live")
	}
	for _, liveOwner := range owners {
		if _, live := c.liveOwners[liveOwner]; !live || c.projections[liveOwner] != previous[liveOwner] {
			c.barrier.Unlock()
			return errors.New("service: progressive owner changed during preparation")
		}
	}
	if !c.acceptedKnown {
		c.acceptedKnown = true
		c.acceptedGeneration = generation
	}
	var delta registryDelta
	if staged != nil {
		c.projections = staged.projections
		c.union = staged.union
		c.interestReferences = staged.interestReferences
		delta = staged.delta
	}
	c.barrier.Unlock()

	t.applyRegistryDelta(delta)
	return nil
}

func (t *ToolSurface) applyRegistryDelta(delta registryDelta) {
	coordinator := t.projectionCoordinator()
	server := coordinator.server
	if server == nil {
		return
	}
	for _, name := range delta.remove {
		server.RemoveTools(name)
	}
	if len(delta.add) > 0 && coordinator.testBeforeRegistryAdd != nil {
		coordinator.testBeforeRegistryAdd()
	}
	for _, descriptor := range delta.add {
		server.AddTool(descriptor, t.pinnedToolHandler(descriptor.Name, descriptor))
	}
}
