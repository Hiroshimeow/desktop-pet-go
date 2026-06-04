package animation

import "time"

type InterruptPolicy string

const (
	InterruptNone      InterruptPolicy = "none"
	InterruptSoft      InterruptPolicy = "soft"
	InterruptHard      InterruptPolicy = "hard"
	InterruptAfterLoop InterruptPolicy = "after_loop"
	InterruptAfterEnd  InterruptPolicy = "after_end"
)

type PlaybackRequest struct {
	Clip            ClipDefinition
	InterruptPolicy InterruptPolicy
	ForcedFor       time.Duration
}

type PlayerSnapshot struct {
	ClipName      string
	FrameIndex    int
	Playing       bool
	Finished      bool
	LoopCompleted bool
	Elapsed       time.Duration
}

type AnimationPlayer struct {
	current       ClipDefinition
	hasCurrent    bool
	frameIndex    int
	frameAccum    time.Duration
	elapsed       time.Duration
	forcedLeft    time.Duration
	finished      bool
	loopCompleted bool
	pending       *PlaybackRequest
}

func NewAnimationPlayer() *AnimationPlayer {
	return &AnimationPlayer{}
}

func (p *AnimationPlayer) Play(req PlaybackRequest) bool {
	if !validPlayableClip(req.Clip) {
		return false
	}
	policy := req.InterruptPolicy
	if policy == "" {
		policy = InterruptHard
	}
	if !p.hasCurrent || p.finished || policy == InterruptHard {
		p.start(req)
		return true
	}
	switch policy {
	case InterruptNone:
		return false
	case InterruptSoft:
		if p.canSoftInterrupt() {
			p.start(req)
			return true
		}
		return false
	case InterruptAfterLoop:
		p.queue(req)
		return false
	case InterruptAfterEnd:
		if p.current.Loop {
			return false
		}
		p.queue(req)
		return false
	default:
		p.start(req)
		return true
	}
}

func (p *AnimationPlayer) Tick(dt time.Duration) PlayerSnapshot {
	if dt <= 0 || !p.hasCurrent || p.finished {
		return p.Snapshot()
	}
	p.elapsed += dt
	if p.forcedLeft > 0 {
		p.forcedLeft -= dt
		if p.forcedLeft < 0 {
			p.forcedLeft = 0
		}
	}
	step := frameDuration(p.current.FPS)
	p.frameAccum += dt
	for p.frameAccum >= step && !p.finished {
		p.frameAccum -= step
		p.advanceFrame()
	}
	if p.current.DurationMS > 0 && p.elapsed >= time.Duration(p.current.DurationMS)*time.Millisecond {
		p.finishIfOneShot()
	}
	if !p.current.Loop && p.frameIndex == p.current.Frames-1 && p.forcedLeft == 0 {
		p.finishIfOneShot()
	}
	p.applyPendingIfReady()
	return p.Snapshot()
}

func (p *AnimationPlayer) Snapshot() PlayerSnapshot {
	if !p.hasCurrent {
		return PlayerSnapshot{}
	}
	return PlayerSnapshot{
		ClipName:      p.current.Name,
		FrameIndex:    p.frameIndex,
		Playing:       !p.finished,
		Finished:      p.finished,
		LoopCompleted: p.loopCompleted,
		Elapsed:       p.elapsed,
	}
}

func (p *AnimationPlayer) CurrentClip() (ClipDefinition, bool) {
	if !p.hasCurrent {
		return ClipDefinition{}, false
	}
	return p.current, true
}

func (p *AnimationPlayer) start(req PlaybackRequest) {
	p.current = req.Clip
	p.hasCurrent = true
	p.frameIndex = 0
	p.frameAccum = 0
	p.elapsed = 0
	p.forcedLeft = req.ForcedFor
	p.finished = false
	p.loopCompleted = false
	p.pending = nil
}

func (p *AnimationPlayer) queue(req PlaybackRequest) {
	copy := req
	p.pending = &copy
}

func (p *AnimationPlayer) advanceFrame() {
	frames := p.current.Frames
	if frames <= 1 {
		if !p.current.Loop {
			p.finishIfOneShot()
		}
		return
	}
	if p.frameIndex+1 >= frames {
		p.loopCompleted = true
		if p.current.Loop {
			p.frameIndex = 0
			return
		}
		p.frameIndex = frames - 1
		p.finishIfOneShot()
		return
	}
	p.frameIndex++
}

func (p *AnimationPlayer) finishIfOneShot() {
	if p.current.Loop || p.forcedLeft > 0 {
		return
	}
	p.finished = true
}

func (p *AnimationPlayer) applyPendingIfReady() {
	if p.pending == nil {
		return
	}
	policy := p.pending.InterruptPolicy
	if policy == InterruptAfterLoop && p.loopCompleted {
		req := *p.pending
		p.start(req)
		return
	}
	if policy == InterruptAfterEnd && p.finished {
		req := *p.pending
		p.start(req)
	}
}

func (p *AnimationPlayer) canSoftInterrupt() bool {
	if p.forcedLeft > 0 {
		return false
	}
	return !p.current.Loop || p.loopCompleted
}

func validPlayableClip(clip ClipDefinition) bool {
	return clip.Name != "" && clip.FPS > 0 && clip.Frames > 0
}

func frameDuration(fps int) time.Duration {
	if fps <= 0 {
		return time.Second
	}
	return time.Second / time.Duration(fps)
}
