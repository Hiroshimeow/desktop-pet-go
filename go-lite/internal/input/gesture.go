package input

import (
	"math"
	"time"
)

type RawEventKind string

const (
	RawPointerDown RawEventKind = "pointer_down"
	RawPointerUp   RawEventKind = "pointer_up"
	RawPointerMove RawEventKind = "pointer_move"
	RawRightClick  RawEventKind = "right_click"
	RawDoubleClick RawEventKind = "double_click"
	RawCancel      RawEventKind = "cancel"
)

type Button string

const (
	ButtonNone  Button = ""
	ButtonLeft  Button = "left"
	ButtonRight Button = "right"
)

type RawEvent struct {
	Kind   RawEventKind
	Button Button
	X      float64
	Y      float64
	At     time.Time
}

type GestureKind string

const (
	GestureClick       GestureKind = "click"
	GestureDragStart   GestureKind = "drag_start"
	GestureDragHold    GestureKind = "drag_hold"
	GestureDragEnd     GestureKind = "drag_end"
	GestureRightClick  GestureKind = "right_click"
	GestureDoubleClick GestureKind = "double_click"
	GestureCancel      GestureKind = "cancel"
)

type GestureEvent struct {
	Kind GestureKind
	X    float64
	Y    float64
	DX   float64
	DY   float64
	At   time.Time
}

type GestureConfig struct {
	DragThresholdPx float64
	HoldRepeat      time.Duration
}

type GestureMapper struct {
	config     GestureConfig
	leftDown   bool
	dragging   bool
	downX      float64
	downY      float64
	lastX      float64
	lastY      float64
	downAt     time.Time
	nextHoldAt time.Time
}

func NewGestureMapper(config GestureConfig) *GestureMapper {
	if config.DragThresholdPx <= 0 {
		config.DragThresholdPx = 6
	}
	if config.HoldRepeat <= 0 {
		config.HoldRepeat = 650 * time.Millisecond
	}
	return &GestureMapper{config: config}
}

func (m *GestureMapper) Handle(raw RawEvent) []GestureEvent {
	now := raw.At
	if now.IsZero() {
		now = time.Now()
	}
	switch raw.Kind {
	case RawPointerDown:
		if raw.Button != ButtonLeft {
			return nil
		}
		m.leftDown = true
		m.dragging = false
		m.downX = raw.X
		m.downY = raw.Y
		m.lastX = raw.X
		m.lastY = raw.Y
		m.downAt = now
		m.nextHoldAt = now.Add(m.config.HoldRepeat)
		return nil
	case RawPointerMove:
		return m.handleMove(raw.X, raw.Y, now)
	case RawPointerUp:
		return m.handleUp(raw.X, raw.Y, now)
	case RawRightClick:
		return []GestureEvent{{Kind: GestureRightClick, X: raw.X, Y: raw.Y, At: now}}
	case RawDoubleClick:
		m.resetLeft()
		return []GestureEvent{{Kind: GestureDoubleClick, X: raw.X, Y: raw.Y, At: now}}
	case RawCancel:
		wasActive := m.leftDown || m.dragging
		m.resetLeft()
		if wasActive {
			return []GestureEvent{{Kind: GestureCancel, X: raw.X, Y: raw.Y, At: now}}
		}
	}
	return nil
}

func (m *GestureMapper) Tick(now time.Time) []GestureEvent {
	if now.IsZero() {
		now = time.Now()
	}
	if !m.leftDown || !m.dragging || now.Before(m.nextHoldAt) {
		return nil
	}
	out := []GestureEvent{{Kind: GestureDragHold, X: m.lastX, Y: m.lastY, DX: m.lastX - m.downX, DY: m.lastY - m.downY, At: now}}
	m.nextHoldAt = now.Add(m.config.HoldRepeat)
	return out
}

func (m *GestureMapper) handleMove(x, y float64, now time.Time) []GestureEvent {
	if !m.leftDown {
		return nil
	}
	m.lastX = x
	m.lastY = y
	dx := x - m.downX
	dy := y - m.downY
	if !m.dragging && distance(dx, dy) >= m.config.DragThresholdPx {
		m.dragging = true
		m.nextHoldAt = now.Add(m.config.HoldRepeat)
		return []GestureEvent{{Kind: GestureDragStart, X: x, Y: y, DX: dx, DY: dy, At: now}}
	}
	return nil
}

func (m *GestureMapper) handleUp(x, y float64, now time.Time) []GestureEvent {
	if !m.leftDown {
		return nil
	}
	dx := x - m.downX
	dy := y - m.downY
	wasDragging := m.dragging
	m.resetLeft()
	if wasDragging {
		return []GestureEvent{{Kind: GestureDragEnd, X: x, Y: y, DX: dx, DY: dy, At: now}}
	}
	return []GestureEvent{{Kind: GestureClick, X: x, Y: y, DX: dx, DY: dy, At: now}}
}

func (m *GestureMapper) resetLeft() {
	m.leftDown = false
	m.dragging = false
	m.downX = 0
	m.downY = 0
	m.lastX = 0
	m.lastY = 0
	m.downAt = time.Time{}
	m.nextHoldAt = time.Time{}
}

func distance(dx, dy float64) float64 {
	return math.Hypot(dx, dy)
}
