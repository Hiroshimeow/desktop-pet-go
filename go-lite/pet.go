package main

import (
	"math"
	"math/rand"
	"time"
)

type Pet struct {
	InstanceID string
	Name       string
	Manifest   PetManifest
	Store      *SpriteStore

	X               float64
	Y               float64
	TargetX         float64
	Facing          int
	Animation       string
	Frame           int
	FrameAccum      float64
	NextThink       time.Time
	ForcedTill      time.Time
	NextDragEmotion time.Time
	DragMode        bool
}

func NewPet(instanceID string, name string, manifest PetManifest, store *SpriteStore, screenW, screenH, frameW, frameH int) *Pet {
	margin := manifest.Motion.ScreenMargin
	if margin <= 0 {
		margin = 28
	}
	x := float64(margin + rand.Intn(max(1, screenW-frameW-margin*2)))
	y := float64(max(0, screenH-frameH-40-rand.Intn(80)))
	return &Pet{InstanceID: instanceID, Name: name, Manifest: manifest, Store: store, X: x, Y: y, TargetX: x, Facing: 1, Animation: manifest.DefaultAnimation, NextThink: time.Now().Add(randomIdleDuration(manifest))}
}

func (p *Pet) Force(animation string, d time.Duration) {
	if _, ok := p.Manifest.Animations[animation]; !ok {
		animation = p.Manifest.DefaultAnimation
	}
	p.Animation = animation
	p.Frame = 0
	p.FrameAccum = 0
	p.ForcedTill = time.Now().Add(d)
}

func (p *Pet) TriggerAction(actionName string) {
	action, ok := p.Manifest.Interactions[actionName]
	if !ok {
		return
	}
	animation := action.Animation
	if animation == "" && len(action.Random) > 0 {
		animation = action.Random[rand.Intn(len(action.Random))]
	}
	d := time.Duration(action.DurationMS) * time.Millisecond
	if d <= 0 {
		d = 1200 * time.Millisecond
	}
	p.Force(animation, d)
}

func (p *Pet) StartDrag() {
	p.DragMode = true
	p.NextDragEmotion = time.Now()
	p.TriggerAction("drag_start")
}

func (p *Pet) EndDrag() {
	p.DragMode = false
	p.TriggerAction("drag_end")
	p.NextThink = time.Now().Add(randomIdleDuration(p.Manifest))
}

func (p *Pet) UpdateDragEmotion() {
	if !p.DragMode {
		return
	}
	action, ok := p.Manifest.Interactions["drag_hold"]
	if !ok {
		return
	}
	now := time.Now()
	if now.Before(p.NextDragEmotion) {
		return
	}
	if len(action.Random) > 0 {
		p.Force(action.Random[rand.Intn(len(action.Random))], 900*time.Millisecond)
	}
	interval := action.IntervalMS
	if interval <= 0 {
		interval = 650
	}
	p.NextDragEmotion = now.Add(time.Duration(interval) * time.Millisecond)
}

func (p *Pet) Update(dt float64, screenW, frameW int) {
	now := time.Now()
	anim := p.Manifest.Animations[p.Animation]
	if now.After(p.ForcedTill) {
		if anim.Locomotion {
			p.updateHorizontalMovement(dt, screenW, frameW)
		} else if now.After(p.NextThink) {
			p.chooseNext(screenW, frameW)
		}
	}
	p.advanceFrame(dt)
}

func (p *Pet) updateHorizontalMovement(dt float64, screenW, frameW int) {
	dx := p.TargetX - p.X
	if math.Abs(dx) <= 2 {
		p.X = p.TargetX
		p.Animation = p.Manifest.DefaultAnimation
		p.Frame = 0
		p.FrameAccum = 0
		p.NextThink = time.Now().Add(randomIdleDuration(p.Manifest))
		return
	}
	if dx > 0 {
		p.Facing = 1
	} else {
		p.Facing = -1
	}
	threshold := float64(p.Manifest.Motion.WalkDistanceThreshold)
	if threshold <= 0 {
		threshold = 420
	}
	remaining := math.Abs(dx)
	moveAnim := p.chooseMoveAnimation(remaining, threshold)
	if p.Animation != moveAnim {
		p.setAnimation(moveAnim)
	}
	anim := p.Manifest.Animations[p.Animation]
	step := anim.SpeedPxS * dt * float64(p.Facing)
	if math.Abs(step) > math.Abs(dx) {
		step = dx
	}
	p.X += step
	p.X = math.Max(0, math.Min(p.X, float64(screenW-frameW)))
}

func (p *Pet) chooseNext(screenW, frameW int) {
	chance := p.Manifest.Motion.AutoRoamChance
	if chance <= 0 {
		chance = 35
	}
	if rand.Intn(100) < chance && hasAnim(p.Manifest, "walk") {
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
		p.setAnimation(p.chooseMoveAnimation(math.Abs(p.TargetX-p.X), float64(p.Manifest.Motion.WalkDistanceThreshold)))
		p.NextThink = time.Now().Add(6 * time.Second)
		return
	}
	states := filterExisting(p.Manifest, []string{"idle", "sleepy", "sit_idle", "thinking", "wave", "happy"})
	if len(states) == 0 {
		states = []string{p.Manifest.DefaultAnimation}
	}
	name := states[rand.Intn(len(states))]
	p.setAnimation(name)
	d := durationOf(p.Manifest.Animations[name])
	p.ForcedTill = time.Now().Add(time.Duration(d) * time.Millisecond)
	p.NextThink = p.ForcedTill
}

func (p *Pet) chooseMoveAnimation(remaining, threshold float64) string {
	base := "walk"
	if remaining > threshold && hasAnim(p.Manifest, "run") {
		base = "run"
	}
	if p.Facing < 0 {
		if hasAnim(p.Manifest, base+"_left") {
			return base + "_left"
		}
	} else {
		if hasAnim(p.Manifest, base+"_right") {
			return base + "_right"
		}
	}
	if hasAnim(p.Manifest, base) {
		return base
	}
	if hasAnim(p.Manifest, "walk") {
		return "walk"
	}
	return p.Manifest.DefaultAnimation
}

func (p *Pet) setAnimation(name string) {
	if _, ok := p.Manifest.Animations[name]; !ok {
		name = p.Manifest.DefaultAnimation
	}
	if p.Animation == name {
		return
	}
	p.Animation = name
	p.Frame = 0
	p.FrameAccum = 0
}

func (p *Pet) advanceFrame(dt float64) {
	anim := p.Manifest.Animations[p.Animation]
	p.FrameAccum += dt
	step := 1.0 / float64(anim.FPS)
	for p.FrameAccum >= step {
		p.FrameAccum -= step
		p.Frame = (p.Frame + 1) % p.frameCountForCurrentAnimation(anim)
	}
}

func hasAnim(m PetManifest, name string) bool { _, ok := m.Animations[name]; return ok }

func filterExisting(m PetManifest, names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if hasAnim(m, name) {
			out = append(out, name)
		}
	}
	return out
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
