package runtimev2

import (
	"time"

	"desktop-pet-lite-go/internal/animation"
	"desktop-pet-lite-go/internal/input"
	"desktop-pet-lite-go/internal/pet"
)

type EngineConfig struct {
	Gesture               input.GestureConfig
	Brain                 pet.BrainConfig
	Cooldown              time.Duration
	RecentLimit           int
	RecentPenalty         int
	SlowDistanceThreshold float64
}

type Engine struct {
	definition *animation.Definition
	mapper     *input.GestureMapper
	brain      *pet.Brain
	player     *animation.AnimationPlayer
	config     EngineConfig
	state      State
}

type State struct {
	X               float64
	TargetX         float64
	HomeX           float64
	HasRoamTarget   bool
	HasReturnTarget bool
	Dragged         bool
}

type StepResult struct {
	Gestures     []input.GestureEvent
	Decision     pet.Decision
	ResolvedClip string
	Player       animation.PlayerSnapshot
	Resolved     bool
}

func NewEngine(definition *animation.Definition, config EngineConfig) *Engine {
	if config.Cooldown <= 0 {
		config.Cooldown = 500 * time.Millisecond
	}
	if config.RecentLimit <= 0 {
		config.RecentLimit = 3
	}
	if config.RecentPenalty <= 0 {
		config.RecentPenalty = 10
	}
	if config.Brain.SlowDistanceThreshold <= 0 && config.SlowDistanceThreshold > 0 {
		config.Brain.SlowDistanceThreshold = config.SlowDistanceThreshold
	}
	return &Engine{
		definition: definition,
		mapper:     input.NewGestureMapper(config.Gesture),
		brain:      pet.NewBrain(config.Brain),
		player:     animation.NewAnimationPlayer(),
		config:     config,
	}
}

func (e *Engine) SetState(state State) {
	e.state = state
}

func (e *Engine) State() State { return e.state }

func (e *Engine) HandleRaw(raw input.RawEvent) StepResult {
	gestures := e.mapper.Handle(raw)
	result := StepResult{Gestures: gestures, Player: e.player.Snapshot()}
	for _, gesture := range gestures {
		result = e.applyGesture(gesture)
	}
	result.Gestures = gestures
	return result
}

func (e *Engine) Tick(now time.Time, dt time.Duration) StepResult {
	gestures := e.mapper.Tick(now)
	result := StepResult{Gestures: gestures}
	for _, gesture := range gestures {
		result = e.applyGesture(gesture)
	}
	if len(gestures) == 0 {
		decision := e.brain.Decide(pet.Event{Kind: pet.EventTick}, e.brainContext(now))
		result.Decision = decision
		if decision.ShouldResolve {
			result = e.resolveDecision(result, decision, now)
		}
	}
	result.Player = e.player.Tick(dt)
	return result
}

func (e *Engine) applyGesture(gesture input.GestureEvent) StepResult {
	now := gesture.At
	var event pet.Event
	switch gesture.Kind {
	case input.GestureClick:
		event = pet.Event{Kind: pet.EventLeftClick}
	case input.GestureRightClick:
		event = pet.Event{Kind: pet.EventRightClick}
	case input.GestureDoubleClick:
		event = pet.Event{Kind: pet.EventLeftClick}
	case input.GestureDragStart:
		e.state.Dragged = true
		event = pet.Event{Kind: pet.EventDragStart}
	case input.GestureDragHold:
		e.state.Dragged = true
		event = pet.Event{Kind: pet.EventDragHold}
	case input.GestureDragEnd:
		e.state.Dragged = false
		event = pet.Event{Kind: pet.EventDragEnd}
	case input.GestureCancel:
		e.state.Dragged = false
		event = pet.Event{Kind: pet.EventDragEnd}
	default:
		return StepResult{Player: e.player.Snapshot()}
	}
	decision := e.brain.Decide(event, e.brainContext(now))
	return e.resolveDecision(StepResult{Decision: decision}, decision, now)
}

func (e *Engine) resolveDecision(result StepResult, decision pet.Decision, now time.Time) StepResult {
	if e.definition == nil || e.definition.Resolver == nil || decision.Intent == "" {
		result.Player = e.player.Snapshot()
		return result
	}
	clip, ok := e.definition.Resolver.ResolveWithContext(string(decision.Intent), animation.ResolveContext{
		Now:               now,
		Cooldown:          e.config.Cooldown,
		RecentLimit:       e.config.RecentLimit,
		RecentPenalty:     e.config.RecentPenalty,
		RecordSelection:   true,
		AllowCooldownOnly: true,
	}, nil)
	if !ok {
		result.Player = e.player.Snapshot()
		return result
	}
	clipDef, ok := e.definition.Clips[clip.Name]
	if !ok {
		result.Player = e.player.Snapshot()
		return result
	}
	e.player.Play(animation.PlaybackRequest{Clip: clipDef, InterruptPolicy: policyForIntent(decision.Intent)})
	result.Resolved = true
	result.ResolvedClip = clip.Name
	result.Player = e.player.Snapshot()
	return result
}

func (e *Engine) brainContext(now time.Time) pet.Context {
	return pet.Context{
		Now:             now,
		X:               e.state.X,
		TargetX:         e.state.TargetX,
		HomeX:           e.state.HomeX,
		Dragged:         e.state.Dragged,
		HasRoamTarget:   e.state.HasRoamTarget,
		HasReturnTarget: e.state.HasReturnTarget,
	}
}

func policyForIntent(intent pet.Intent) animation.InterruptPolicy {
	switch intent {
	case pet.IntentDragStart, pet.IntentDragHold, pet.IntentDragEnd:
		return animation.InterruptHard
	case pet.IntentLeftClick, pet.IntentRightClick:
		return animation.InterruptSoft
	case pet.IntentLocomotionSlow, pet.IntentLocomotionFast:
		return animation.InterruptAfterEnd
	default:
		return animation.InterruptSoft
	}
}
