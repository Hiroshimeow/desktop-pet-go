package main

import (
	"math"
	"math/rand"
	"time"

	"desktop-pet-lite-go/internal/input"
	"desktop-pet-lite-go/internal/pet"
	"desktop-pet-lite-go/internal/runtimev2"
)

type Pet struct {
	InstanceID string
	Name       string
	Manifest   PetManifest
	Store      *SpriteStore

	X             float64
	Y             float64
	TargetX       float64
	HasRoamTarget bool
	Facing        int
	Animation     string
	Frame         int
	NextThink     time.Time
	DragMode      bool
	V2            *runtimev2.Engine
}

func NewPet(instanceID string, name string, manifest PetManifest, store *SpriteStore, screenW, screenH, frameW, frameH int) *Pet {
	margin := manifest.Motion.ScreenMargin
	if margin <= 0 {
		margin = 28
	}
	x := float64(margin + rand.Intn(max(1, screenW-frameW-margin*2)))
	y := float64(max(0, screenH-frameH-40-rand.Intn(80)))
	p := &Pet{InstanceID: instanceID, Name: name, Manifest: manifest, Store: store, X: x, Y: y, TargetX: x, Facing: 1, Animation: manifest.DefaultAnimation, NextThink: time.Now().Add(randomIdleDuration(manifest))}
	definition, err := compileLegacyManifestV2(manifest)
	if err == nil {
		p.V2 = runtimev2.NewEngine(definition, runtimev2.EngineConfig{
			Gesture: input.GestureConfig{DragThresholdPx: 10, HoldRepeat: 650 * time.Millisecond},
			Brain: pet.BrainConfig{
				ReactionDuration:      1200 * time.Millisecond,
				DragHoldRepeat:        650 * time.Millisecond,
				SlowDistanceThreshold: float64(manifest.Motion.WalkDistanceThreshold),
			},
		})
		p.syncV2State()
	}
	return p
}

func (p *Pet) TriggerAction(actionName string) {
	p.handleV2Action(actionName)
}

func (p *Pet) TriggerIntent(intent pet.Intent) {
	if p == nil || p.V2 == nil {
		return
	}
	p.syncFromV2(p.V2.PlayIntent(intent, time.Now()))
}

func (p *Pet) StartDrag() {
	p.DragMode = true
	p.handleV2Action("drag_start")
}

func (p *Pet) EndDrag() {
	p.DragMode = false
	p.handleV2Action("drag_end")
	p.NextThink = time.Now().Add(randomIdleDuration(p.Manifest))
}

func (p *Pet) UpdateDragEmotion() {
	if !p.DragMode {
		return
	}
	p.handleV2Action("drag_hold")
}

func (p *Pet) Update(dt float64, screenW, frameW int) {
	p.updateV2(dt, screenW, frameW)
}

func randomIdleDuration(m PetManifest) time.Duration {
	minMS := m.Motion.MinIdleMS
	maxMS := m.Motion.MaxIdleMS
	if minMS <= 0 {
		minMS = 1800
	}
	if maxMS < minMS {
		maxMS = minMS + 2000
	}
	return time.Duration(minMS+rand.Intn(maxMS-minMS+1)) * time.Millisecond
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (p *Pet) frameCountForCurrentAnimation(anim AnimationDef) int {
	if p.Store != nil {
		if strip, ok := p.Store.Animations[p.Animation]; ok && strip.Frames > 0 {
			return strip.Frames
		}
	}
	return frameCountOf(p.Manifest, anim)
}

func (p *Pet) syncV2State() {
	if p == nil || p.V2 == nil {
		return
	}
	p.V2.SetState(runtimev2.State{
		X:               p.X,
		TargetX:         p.TargetX,
		HomeX:           p.X,
		HasRoamTarget:   p.HasRoamTarget && math.Abs(p.TargetX-p.X) > 2,
		HasReturnTarget: false,
		Dragged:         p.DragMode,
	})
}

func (p *Pet) syncFromV2(result runtimev2.StepResult) {
	if !result.Resolved && result.Player.ClipName == "" {
		return
	}
	if result.Player.ClipName != "" {
		p.Animation = result.Player.ClipName
		p.Frame = result.Player.FrameIndex
	}
}

func (p *Pet) handleV2Action(actionName string) {
	if p == nil || p.V2 == nil {
		return
	}
	now := time.Now()
	p.syncV2State()
	switch actionName {
	case "left_click":
		p.syncFromV2(p.V2.HandleRaw(input.RawEvent{Kind: input.RawPointerDown, Button: input.ButtonLeft, X: p.X, Y: p.Y, At: now}))
		p.syncFromV2(p.V2.HandleRaw(input.RawEvent{Kind: input.RawPointerUp, Button: input.ButtonLeft, X: p.X, Y: p.Y, At: now.Add(time.Millisecond)}))
	case "right_click":
		p.syncFromV2(p.V2.HandleRaw(input.RawEvent{Kind: input.RawRightClick, Button: input.ButtonRight, X: p.X, Y: p.Y, At: now}))
	case "drag_start":
		p.syncFromV2(p.V2.HandleRaw(input.RawEvent{Kind: input.RawPointerDown, Button: input.ButtonLeft, X: p.X, Y: p.Y, At: now}))
		p.syncFromV2(p.V2.HandleRaw(input.RawEvent{Kind: input.RawPointerMove, Button: input.ButtonLeft, X: p.X + 10, Y: p.Y, At: now.Add(time.Millisecond)}))
	case "drag_hold":
		p.syncFromV2(p.V2.Tick(now, 0))
	case "drag_end":
		p.syncFromV2(p.V2.HandleRaw(input.RawEvent{Kind: input.RawPointerUp, Button: input.ButtonLeft, X: p.X, Y: p.Y, At: now}))
	}
}

func (p *Pet) updateV2(dt float64, screenW, frameW int) {
	if p == nil || p.V2 == nil {
		return
	}
	now := time.Now()
	if now.After(p.NextThink) && !p.DragMode {
		p.chooseNextTargetV2(screenW, frameW)
	}
	p.syncV2State()
	result := p.V2.Tick(now, time.Duration(dt*float64(time.Second)))
	p.syncFromV2(result)
	if def, ok := p.Manifest.Animations[p.Animation]; ok && def.Locomotion {
		p.updateHorizontalMovementV2(dt, screenW, frameW, def)
	}
}

func (p *Pet) chooseNextTargetV2(screenW, frameW int) {
	chance := p.Manifest.Motion.AutoRoamChance
	if chance <= 0 {
		chance = 35
	}
	if rand.Intn(100) < chance {
		margin := p.Manifest.Motion.ScreenMargin
		if margin <= 0 {
			margin = 28
		}
		minX := margin
		maxX := max(minX+1, screenW-frameW-margin)
		p.TargetX = float64(minX + rand.Intn(max(1, maxX-minX)))
		if math.Abs(p.TargetX-p.X) < 80 {
			p.TargetX = math.Min(float64(maxX), p.X+180)
		}
		if p.TargetX >= p.X {
			p.Facing = 1
		} else {
			p.Facing = -1
		}
		p.HasRoamTarget = true
		p.NextThink = time.Now().Add(6 * time.Second)
		return
	}
	p.HasRoamTarget = false
	p.TargetX = p.X
	p.NextThink = time.Now().Add(randomIdleDuration(p.Manifest))
}

func (p *Pet) updateHorizontalMovementV2(dt float64, screenW, frameW int, anim AnimationDef) {
	dx := p.TargetX - p.X
	if math.Abs(dx) <= 2 {
		p.X = p.TargetX
		p.TargetX = p.X
		p.HasRoamTarget = false
		p.NextThink = time.Now().Add(randomIdleDuration(p.Manifest))
		return
	}
	if dx > 0 {
		p.Facing = 1
	} else {
		p.Facing = -1
	}
	speed := anim.SpeedPxS
	if speed <= 0 {
		speed = 35
	}
	step := speed * dt * float64(p.Facing)
	if math.Abs(step) > math.Abs(dx) {
		step = dx
	}
	p.X += step
	p.X = math.Max(0, math.Min(p.X, float64(screenW-frameW)))
}
