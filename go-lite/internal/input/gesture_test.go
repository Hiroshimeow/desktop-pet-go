package input

import (
	"testing"
	"time"
)

func TestGestureMapperMapsClickBelowDragThreshold(t *testing.T) {
	now := time.Unix(100, 0)
	mapper := NewGestureMapper(GestureConfig{DragThresholdPx: 10})
	if got := mapper.Handle(RawEvent{Kind: RawPointerDown, Button: ButtonLeft, X: 10, Y: 10, At: now}); len(got) != 0 {
		t.Fatalf("pointer down gestures = %v, want none", got)
	}
	if got := mapper.Handle(RawEvent{Kind: RawPointerMove, X: 15, Y: 15, At: now.Add(10 * time.Millisecond)}); len(got) != 0 {
		t.Fatalf("small move gestures = %v, want none", got)
	}
	got := mapper.Handle(RawEvent{Kind: RawPointerUp, Button: ButtonLeft, X: 15, Y: 15, At: now.Add(20 * time.Millisecond)})
	assertSingleGesture(t, got, GestureClick)
}

func TestGestureMapperStartsDragAtThreshold(t *testing.T) {
	now := time.Unix(100, 0)
	mapper := NewGestureMapper(GestureConfig{DragThresholdPx: 10, HoldRepeat: time.Second})
	mapper.Handle(RawEvent{Kind: RawPointerDown, Button: ButtonLeft, X: 0, Y: 0, At: now})
	got := mapper.Handle(RawEvent{Kind: RawPointerMove, X: 10, Y: 0, At: now.Add(10 * time.Millisecond)})
	event := assertSingleGesture(t, got, GestureDragStart)
	if event.DX != 10 || event.DY != 0 {
		t.Fatalf("drag delta = (%v,%v), want (10,0)", event.DX, event.DY)
	}
}

func TestGestureMapperEmitsDragHoldOnTick(t *testing.T) {
	now := time.Unix(100, 0)
	mapper := NewGestureMapper(GestureConfig{DragThresholdPx: 5, HoldRepeat: 500 * time.Millisecond})
	mapper.Handle(RawEvent{Kind: RawPointerDown, Button: ButtonLeft, X: 0, Y: 0, At: now})
	mapper.Handle(RawEvent{Kind: RawPointerMove, X: 6, Y: 0, At: now.Add(10 * time.Millisecond)})
	if got := mapper.Tick(now.Add(400 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("early tick gestures = %v, want none", got)
	}
	got := mapper.Tick(now.Add(510 * time.Millisecond))
	assertSingleGesture(t, got, GestureDragHold)
	if got := mapper.Tick(now.Add(900 * time.Millisecond)); len(got) != 0 {
		t.Fatalf("second early tick gestures = %v, want none", got)
	}
	got = mapper.Tick(now.Add(1100 * time.Millisecond))
	assertSingleGesture(t, got, GestureDragHold)
}

func TestGestureMapperEndsDragOnPointerUp(t *testing.T) {
	now := time.Unix(100, 0)
	mapper := NewGestureMapper(GestureConfig{DragThresholdPx: 5})
	mapper.Handle(RawEvent{Kind: RawPointerDown, Button: ButtonLeft, X: 1, Y: 1, At: now})
	mapper.Handle(RawEvent{Kind: RawPointerMove, X: 7, Y: 1, At: now.Add(10 * time.Millisecond)})
	got := mapper.Handle(RawEvent{Kind: RawPointerUp, Button: ButtonLeft, X: 9, Y: 2, At: now.Add(20 * time.Millisecond)})
	event := assertSingleGesture(t, got, GestureDragEnd)
	if event.DX != 8 || event.DY != 1 {
		t.Fatalf("drag end delta = (%v,%v), want (8,1)", event.DX, event.DY)
	}
	if got := mapper.Tick(now.Add(time.Second)); len(got) != 0 {
		t.Fatalf("tick after drag end gestures = %v, want none", got)
	}
}

func TestGestureMapperMapsRightAndDoubleClick(t *testing.T) {
	now := time.Unix(100, 0)
	mapper := NewGestureMapper(GestureConfig{})
	assertSingleGesture(t, mapper.Handle(RawEvent{Kind: RawRightClick, Button: ButtonRight, X: 1, Y: 2, At: now}), GestureRightClick)
	assertSingleGesture(t, mapper.Handle(RawEvent{Kind: RawDoubleClick, Button: ButtonLeft, X: 3, Y: 4, At: now}), GestureDoubleClick)
}

func TestGestureMapperCancelResetsActiveDrag(t *testing.T) {
	now := time.Unix(100, 0)
	mapper := NewGestureMapper(GestureConfig{DragThresholdPx: 5})
	mapper.Handle(RawEvent{Kind: RawPointerDown, Button: ButtonLeft, X: 0, Y: 0, At: now})
	mapper.Handle(RawEvent{Kind: RawPointerMove, X: 10, Y: 0, At: now.Add(time.Millisecond)})
	assertSingleGesture(t, mapper.Handle(RawEvent{Kind: RawCancel, X: 10, Y: 0, At: now.Add(2 * time.Millisecond)}), GestureCancel)
	if got := mapper.Handle(RawEvent{Kind: RawPointerUp, Button: ButtonLeft, X: 10, Y: 0, At: now.Add(3 * time.Millisecond)}); len(got) != 0 {
		t.Fatalf("pointer up after cancel gestures = %v, want none", got)
	}
}

func TestGestureMapperIgnoresMoveWithoutPointerDown(t *testing.T) {
	mapper := NewGestureMapper(GestureConfig{})
	if got := mapper.Handle(RawEvent{Kind: RawPointerMove, X: 10, Y: 10, At: time.Unix(100, 0)}); len(got) != 0 {
		t.Fatalf("move without down gestures = %v, want none", got)
	}
}

func assertSingleGesture(t *testing.T, got []GestureEvent, kind GestureKind) GestureEvent {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("gesture count = %d, want 1 (%s): %v", len(got), kind, got)
	}
	if got[0].Kind != kind {
		t.Fatalf("gesture kind = %q, want %q", got[0].Kind, kind)
	}
	return got[0]
}
