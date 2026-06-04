package animation

import (
	"testing"
	"time"
)

func TestAnimationPlayerAdvancesLoopingFrames(t *testing.T) {
	player := NewAnimationPlayer()
	if !player.Play(PlaybackRequest{Clip: testClip("walk", 4, 3, true)}) {
		t.Fatal("Play() = false")
	}
	snap := player.Tick(250 * time.Millisecond)
	if snap.FrameIndex != 1 {
		t.Fatalf("frame after 250ms = %d, want 1", snap.FrameIndex)
	}
	snap = player.Tick(500 * time.Millisecond)
	if snap.FrameIndex != 0 {
		t.Fatalf("frame after loop = %d, want 0", snap.FrameIndex)
	}
	if !snap.LoopCompleted {
		t.Fatal("LoopCompleted = false")
	}
	if !snap.Playing || snap.Finished {
		t.Fatalf("looping clip should still play: %+v", snap)
	}
}

func TestAnimationPlayerFinishesOneShotAtLastFrame(t *testing.T) {
	player := NewAnimationPlayer()
	player.Play(PlaybackRequest{Clip: testClip("wave", 4, 3, false)})
	snap := player.Tick(750 * time.Millisecond)
	if snap.FrameIndex != 2 {
		t.Fatalf("one-shot final frame = %d, want 2", snap.FrameIndex)
	}
	if !snap.Finished || snap.Playing {
		t.Fatalf("one-shot should be finished: %+v", snap)
	}
}

func TestAnimationPlayerFinishesOneShotByDuration(t *testing.T) {
	player := NewAnimationPlayer()
	clip := testClip("pose", 10, 10, false)
	clip.DurationMS = 250
	player.Play(PlaybackRequest{Clip: clip})
	snap := player.Tick(250 * time.Millisecond)
	if !snap.Finished {
		t.Fatalf("duration-ended clip should be finished: %+v", snap)
	}
}

func TestAnimationPlayerHardInterruptStartsImmediately(t *testing.T) {
	player := NewAnimationPlayer()
	player.Play(PlaybackRequest{Clip: testClip("walk", 4, 4, true)})
	player.Tick(250 * time.Millisecond)
	if !player.Play(PlaybackRequest{Clip: testClip("wave", 4, 4, false), InterruptPolicy: InterruptHard}) {
		t.Fatal("hard interrupt Play() = false")
	}
	snap := player.Snapshot()
	if snap.ClipName != "wave" || snap.FrameIndex != 0 {
		t.Fatalf("hard interrupt snapshot = %+v, want wave frame 0", snap)
	}
}

func TestAnimationPlayerSoftInterruptRespectsForcedWindow(t *testing.T) {
	player := NewAnimationPlayer()
	player.Play(PlaybackRequest{Clip: testClip("reaction", 4, 4, false), ForcedFor: time.Second})
	if player.Play(PlaybackRequest{Clip: testClip("idle", 4, 4, true), InterruptPolicy: InterruptSoft}) {
		t.Fatal("soft interrupt during forced window = true, want false")
	}
	snap := player.Snapshot()
	if snap.ClipName != "reaction" {
		t.Fatalf("current clip = %q, want reaction", snap.ClipName)
	}
	player.Tick(time.Second)
	if !player.Snapshot().Finished {
		t.Fatalf("forced one-shot should finish after forced window expires: %+v", player.Snapshot())
	}
	if !player.Play(PlaybackRequest{Clip: testClip("idle", 4, 4, true), InterruptPolicy: InterruptSoft}) {
		t.Fatal("soft interrupt after forced window = false, want true")
	}
	if player.Snapshot().ClipName != "idle" {
		t.Fatalf("current clip = %q, want idle", player.Snapshot().ClipName)
	}
}

func TestAnimationPlayerInterruptNoneKeepsCurrentClip(t *testing.T) {
	player := NewAnimationPlayer()
	player.Play(PlaybackRequest{Clip: testClip("walk", 4, 4, true)})
	if player.Play(PlaybackRequest{Clip: testClip("wave", 4, 4, false), InterruptPolicy: InterruptNone}) {
		t.Fatal("InterruptNone Play() = true, want false")
	}
	if player.Snapshot().ClipName != "walk" {
		t.Fatalf("current clip = %q, want walk", player.Snapshot().ClipName)
	}
}

func TestAnimationPlayerAfterLoopStartsPendingAtLoopBoundary(t *testing.T) {
	player := NewAnimationPlayer()
	player.Play(PlaybackRequest{Clip: testClip("walk", 4, 2, true)})
	if player.Play(PlaybackRequest{Clip: testClip("wave", 4, 2, false), InterruptPolicy: InterruptAfterLoop}) {
		t.Fatal("queued after_loop Play() = true, want false")
	}
	snap := player.Tick(500 * time.Millisecond)
	if snap.ClipName != "wave" || snap.FrameIndex != 0 {
		t.Fatalf("after_loop snapshot = %+v, want wave frame 0", snap)
	}
}

func TestAnimationPlayerAfterEndStartsPendingAfterOneShotFinishes(t *testing.T) {
	player := NewAnimationPlayer()
	player.Play(PlaybackRequest{Clip: testClip("wave", 4, 2, false)})
	if player.Play(PlaybackRequest{Clip: testClip("idle", 4, 2, true), InterruptPolicy: InterruptAfterEnd}) {
		t.Fatal("queued after_end Play() = true, want false")
	}
	snap := player.Tick(500 * time.Millisecond)
	if snap.ClipName != "idle" || snap.FrameIndex != 0 || !snap.Playing {
		t.Fatalf("after_end snapshot = %+v, want idle playing", snap)
	}
}

func TestAnimationPlayerAfterEndDoesNotQueueBehindLoopingClip(t *testing.T) {
	player := NewAnimationPlayer()
	player.Play(PlaybackRequest{Clip: testClip("idle", 4, 2, true)})
	if player.Play(PlaybackRequest{Clip: testClip("wave", 4, 2, false), InterruptPolicy: InterruptAfterEnd}) {
		t.Fatal("after_end behind loop Play() = true, want false")
	}
	player.Tick(time.Second)
	if player.Snapshot().ClipName != "idle" {
		t.Fatalf("current clip = %q, want idle", player.Snapshot().ClipName)
	}
}

func TestAnimationPlayerRejectsInvalidClip(t *testing.T) {
	player := NewAnimationPlayer()
	if player.Play(PlaybackRequest{Clip: ClipDefinition{Clip: Clip{Name: "broken"}, FPS: 0, Frames: 1}}) {
		t.Fatal("Play(invalid clip) = true, want false")
	}
}

func testClip(name string, fps int, frames int, loop bool) ClipDefinition {
	return ClipDefinition{
		Clip: Clip{
			Name:     name,
			Priority: 1,
		},
		FPS:    fps,
		Frames: frames,
		Loop:   loop,
	}
}
