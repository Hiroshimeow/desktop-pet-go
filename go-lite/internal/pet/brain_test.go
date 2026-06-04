package pet

import (
	"testing"
	"time"
)

func TestBrainInitialTickEmitsIdleIntent(t *testing.T) {
	brain := NewBrain(BrainConfig{})
	decision := brain.Decide(Event{Kind: EventTick}, Context{Now: time.Unix(100, 0)})
	assertDecision(t, decision, StateIdle, IntentIdle)
	if decision.ShouldResolve {
		t.Fatal("initial unchanged idle tick should not require resolve")
	}
}

func TestBrainLeftClickEmitsReactionIntentWithoutClipName(t *testing.T) {
	brain := NewBrain(BrainConfig{})
	decision := brain.Decide(Event{Kind: EventLeftClick}, Context{Now: time.Unix(100, 0)})
	assertDecision(t, decision, StateForcedReaction, IntentLeftClick)
	if !decision.ShouldResolve {
		t.Fatal("click reaction should require resolve")
	}
}

func TestBrainRightClickEmitsReactionIntent(t *testing.T) {
	brain := NewBrain(BrainConfig{})
	decision := brain.Decide(Event{Kind: EventRightClick}, Context{Now: time.Unix(100, 0)})
	assertDecision(t, decision, StateForcedReaction, IntentRightClick)
}

func TestBrainDragLifecycleEmitsDragIntents(t *testing.T) {
	now := time.Unix(100, 0)
	brain := NewBrain(BrainConfig{DragHoldRepeat: 500 * time.Millisecond})
	start := brain.Decide(Event{Kind: EventDragStart}, Context{Now: now})
	assertDecision(t, start, StateDragged, IntentDragStart)
	if !start.ShouldResolve {
		t.Fatal("drag_start should resolve")
	}
	earlyHold := brain.Decide(Event{Kind: EventDragHold}, Context{Now: now.Add(100 * time.Millisecond), Dragged: true})
	assertDecision(t, earlyHold, StateDragged, IntentDragStart)
	if earlyHold.ShouldResolve {
		t.Fatal("early drag_hold before repeat interval should not resolve")
	}
	hold := brain.Decide(Event{Kind: EventDragHold}, Context{Now: now.Add(500 * time.Millisecond), Dragged: true})
	assertDecision(t, hold, StateDragged, IntentDragHold)
	if !hold.ShouldResolve {
		t.Fatal("drag_hold at repeat interval should resolve")
	}
	end := brain.Decide(Event{Kind: EventDragEnd}, Context{Now: now.Add(time.Second)})
	assertDecision(t, end, StateForcedReaction, IntentDragEnd)
}

func TestBrainForcedReactionHoldsUntilDurationExpires(t *testing.T) {
	now := time.Unix(100, 0)
	brain := NewBrain(BrainConfig{ReactionDuration: time.Second})
	brain.Decide(Event{Kind: EventLeftClick}, Context{Now: now})
	during := brain.Decide(Event{Kind: EventTick}, Context{Now: now.Add(500 * time.Millisecond), HasRoamTarget: true, X: 0, TargetX: 1000})
	assertDecision(t, during, StateForcedReaction, IntentLeftClick)
	if during.ShouldResolve {
		t.Fatal("forced reaction should not resolve to locomotion before duration expires")
	}
	after := brain.Decide(Event{Kind: EventTick}, Context{Now: now.Add(time.Second), HasRoamTarget: true, X: 0, TargetX: 1000})
	assertDecision(t, after, StateRoaming, IntentLocomotionFast)
	if !after.ShouldResolve {
		t.Fatal("expired forced reaction should resolve locomotion")
	}
}

func TestBrainDistanceBasedLocomotionIntent(t *testing.T) {
	brain := NewBrain(BrainConfig{SlowDistanceThreshold: 100})
	slow := brain.Decide(Event{Kind: EventTick}, Context{Now: time.Unix(100, 0), HasRoamTarget: true, X: 0, TargetX: 50})
	assertDecision(t, slow, StateRoaming, IntentLocomotionSlow)
	fast := brain.Decide(Event{Kind: EventTick}, Context{Now: time.Unix(101, 0), HasRoamTarget: true, X: 0, TargetX: 150})
	assertDecision(t, fast, StateRoaming, IntentLocomotionFast)
}

func TestBrainReturnTargetUsesReturningState(t *testing.T) {
	brain := NewBrain(BrainConfig{SlowDistanceThreshold: 100})
	decision := brain.Decide(Event{Kind: EventTick}, Context{Now: time.Unix(100, 0), HasReturnTarget: true, X: 300, HomeX: 0})
	assertDecision(t, decision, StateReturning, IntentLocomotionFast)
}

func TestBrainReturnsToIdleWithoutTargets(t *testing.T) {
	brain := NewBrain(BrainConfig{})
	brain.Decide(Event{Kind: EventTick}, Context{Now: time.Unix(100, 0), HasRoamTarget: true, X: 0, TargetX: 10})
	decision := brain.Decide(Event{Kind: EventTick}, Context{Now: time.Unix(101, 0)})
	assertDecision(t, decision, StateIdle, IntentIdle)
}

func assertDecision(t *testing.T, decision Decision, state BrainState, intent Intent) {
	t.Helper()
	if decision.State != state {
		t.Fatalf("state = %q, want %q", decision.State, state)
	}
	if decision.Intent != intent {
		t.Fatalf("intent = %q, want %q", decision.Intent, intent)
	}
}
