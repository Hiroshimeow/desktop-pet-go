package pet

import (
	"math"
	"time"
)

type Intent string

const (
	IntentIdle           Intent = "idle"
	IntentLeftClick      Intent = "left_click"
	IntentRightClick     Intent = "right_click"
	IntentDragStart      Intent = "drag_start"
	IntentDragHold       Intent = "drag_hold"
	IntentDragEnd        Intent = "drag_end"
	IntentLocomotionSlow Intent = "locomotion_slow"
	IntentLocomotionFast Intent = "locomotion_fast"
	IntentVoiceListening Intent = "voice_listening"
	IntentVoiceThinking  Intent = "voice_thinking"
	IntentVoiceSpeaking  Intent = "voice_speaking"
	IntentVoiceUnknown   Intent = "voice_unknown"
	IntentVoiceError     Intent = "voice_error"
)

type BrainState string

const (
	StateIdle           BrainState = "idle"
	StateRoaming        BrainState = "roaming"
	StateForcedReaction BrainState = "forced_reaction"
	StateDragged        BrainState = "dragged"
	StateReturning      BrainState = "returning"
)

type EventKind string

const (
	EventTick       EventKind = "tick"
	EventLeftClick  EventKind = "left_click"
	EventRightClick EventKind = "right_click"
	EventDragStart  EventKind = "drag_start"
	EventDragHold   EventKind = "drag_hold"
	EventDragEnd    EventKind = "drag_end"
)

type BrainConfig struct {
	ReactionDuration      time.Duration
	DragHoldRepeat        time.Duration
	SlowDistanceThreshold float64
}

type Context struct {
	Now             time.Time
	X               float64
	TargetX         float64
	HomeX           float64
	Dragged         bool
	HasRoamTarget   bool
	HasReturnTarget bool
}

type Event struct {
	Kind EventKind
}

type Decision struct {
	Intent        Intent
	State         BrainState
	Changed       bool
	ShouldResolve bool
}

type Brain struct {
	config               BrainConfig
	state                BrainState
	currentIntent        Intent
	forcedUntil          time.Time
	nextDragHoldIntentAt time.Time
}

func NewBrain(config BrainConfig) *Brain {
	if config.ReactionDuration <= 0 {
		config.ReactionDuration = 1200 * time.Millisecond
	}
	if config.DragHoldRepeat <= 0 {
		config.DragHoldRepeat = 650 * time.Millisecond
	}
	if config.SlowDistanceThreshold <= 0 {
		config.SlowDistanceThreshold = 420
	}
	return &Brain{
		config:        config,
		state:         StateIdle,
		currentIntent: IntentIdle,
	}
}

func (b *Brain) Decide(event Event, ctx Context) Decision {
	now := ctx.Now
	if now.IsZero() {
		now = time.Now()
	}
	previousState := b.state
	previousIntent := b.currentIntent
	resolve := false

	switch event.Kind {
	case EventLeftClick:
		b.enterForcedReaction(IntentLeftClick, now)
		resolve = true
	case EventRightClick:
		b.enterForcedReaction(IntentRightClick, now)
		resolve = true
	case EventDragStart:
		b.state = StateDragged
		b.currentIntent = IntentDragStart
		b.nextDragHoldIntentAt = now.Add(b.config.DragHoldRepeat)
		resolve = true
	case EventDragHold:
		if b.state != StateDragged {
			b.state = StateDragged
		}
		if !now.Before(b.nextDragHoldIntentAt) {
			b.currentIntent = IntentDragHold
			b.nextDragHoldIntentAt = now.Add(b.config.DragHoldRepeat)
			resolve = true
		}
	case EventDragEnd:
		b.enterForcedReaction(IntentDragEnd, now)
		resolve = true
	default:
		resolve = b.decideFromContext(now, ctx)
	}

	changed := previousState != b.state || previousIntent != b.currentIntent
	return Decision{
		Intent:        b.currentIntent,
		State:         b.state,
		Changed:       changed,
		ShouldResolve: resolve || changed,
	}
}

func (b *Brain) State() BrainState { return b.state }

func (b *Brain) CurrentIntent() Intent { return b.currentIntent }

func (b *Brain) enterForcedReaction(intent Intent, now time.Time) {
	b.state = StateForcedReaction
	b.currentIntent = intent
	b.forcedUntil = now.Add(b.config.ReactionDuration)
}

func (b *Brain) decideFromContext(now time.Time, ctx Context) bool {
	if b.state == StateDragged || ctx.Dragged {
		if !now.Before(b.nextDragHoldIntentAt) {
			b.state = StateDragged
			b.currentIntent = IntentDragHold
			b.nextDragHoldIntentAt = now.Add(b.config.DragHoldRepeat)
			return true
		}
		b.state = StateDragged
		return false
	}
	if b.state == StateForcedReaction && now.Before(b.forcedUntil) {
		return false
	}
	if ctx.HasReturnTarget {
		return b.setLocomotion(StateReturning, ctx.X, ctx.HomeX)
	}
	if ctx.HasRoamTarget {
		return b.setLocomotion(StateRoaming, ctx.X, ctx.TargetX)
	}
	return b.setIntent(StateIdle, IntentIdle)
}

func (b *Brain) setLocomotion(state BrainState, x float64, targetX float64) bool {
	intent := IntentLocomotionSlow
	if math.Abs(targetX-x) > b.config.SlowDistanceThreshold {
		intent = IntentLocomotionFast
	}
	return b.setIntent(state, intent)
}

func (b *Brain) setIntent(state BrainState, intent Intent) bool {
	changed := b.state != state || b.currentIntent != intent
	b.state = state
	b.currentIntent = intent
	return changed
}
