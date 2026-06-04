package animation

import (
	"fmt"
	"time"
)

type RNG interface {
	Intn(n int) int
}

type Resolver struct {
	intents map[string]CompiledIntent
	now     func() time.Time
	state   resolverState
}

type resolverState struct {
	cooldowns map[string]time.Time
	recent    []string
}

type ResolveContext struct {
	Now                  time.Time
	Cooldown             time.Duration
	RecentLimit          int
	RecentPenalty        int
	CooldownPenalty      int
	AllowCooldownOnly    bool
	RecordSelection      bool
	DisableRecentPenalty bool
}

func NewResolver(intents ...CompiledIntent) (*Resolver, error) {
	byName := make(map[string]CompiledIntent, len(intents))
	for _, intent := range intents {
		if intent.Name == "" {
			return nil, fmt.Errorf("compiled intent name must not be empty")
		}
		if _, exists := byName[intent.Name]; exists {
			return nil, fmt.Errorf("duplicate compiled intent %q", intent.Name)
		}
		byName[intent.Name] = intent
	}
	return &Resolver{
		intents: byName,
		now:     time.Now,
		state: resolverState{
			cooldowns: map[string]time.Time{},
		},
	}, nil
}

func (r *Resolver) Resolve(intentName string, rng RNG) (Clip, bool) {
	return r.ResolveWithContext(intentName, ResolveContext{}, rng)
}

func (r *Resolver) ResolveWithContext(intentName string, ctx ResolveContext, rng RNG) (Clip, bool) {
	intent, ok := r.intents[intentName]
	if !ok {
		return Clip{}, false
	}
	now := ctx.Now
	if now.IsZero() {
		if r.now != nil {
			now = r.now()
		} else {
			now = time.Now()
		}
	}
	for _, group := range intent.Groups {
		usable := r.usableCandidates(group, now)
		if len(usable) == 0 && ctx.AllowCooldownOnly {
			usable = group
		}
		if len(usable) == 0 {
			continue
		}
		scored := r.scoreCandidates(usable, now, ctx)
		clip := chooseWeighted(scored, rng)
		if ctx.RecordSelection {
			r.RecordSelection(clip.Name, now, ctx.Cooldown, ctx.RecentLimit)
		}
		return clip, true
	}
	return Clip{}, false
}

func (r *Resolver) RecordSelection(clipName string, now time.Time, cooldown time.Duration, recentLimit int) {
	if clipName == "" {
		return
	}
	if now.IsZero() {
		if r.now != nil {
			now = r.now()
		} else {
			now = time.Now()
		}
	}
	if cooldown > 0 {
		if r.state.cooldowns == nil {
			r.state.cooldowns = map[string]time.Time{}
		}
		r.state.cooldowns[clipName] = now.Add(cooldown)
	}
	if recentLimit > 0 {
		r.state.recent = append([]string{clipName}, r.state.recent...)
		if len(r.state.recent) > recentLimit {
			r.state.recent = r.state.recent[:recentLimit]
		}
	}
}

func (r *Resolver) usableCandidates(group []Candidate, now time.Time) []Candidate {
	if len(group) == 0 {
		return nil
	}
	if len(r.state.cooldowns) == 0 {
		return group
	}
	usable := make([]Candidate, 0, len(group))
	for _, candidate := range group {
		until, cooling := r.state.cooldowns[candidate.Clip.Name]
		if !cooling || !now.Before(until) {
			usable = append(usable, candidate)
		}
	}
	return usable
}

func (r *Resolver) scoreCandidates(group []Candidate, now time.Time, ctx ResolveContext) []Candidate {
	if len(group) == 0 {
		return nil
	}
	out := make([]Candidate, 0, len(group))
	for _, candidate := range group {
		score := candidate.Score
		if ctx.CooldownPenalty > 0 {
			if until, cooling := r.state.cooldowns[candidate.Clip.Name]; cooling && now.Before(until) {
				score -= ctx.CooldownPenalty
			}
		}
		if !ctx.DisableRecentPenalty && ctx.RecentPenalty > 0 {
			score -= r.recentPenalty(candidate.Clip.Name, ctx.RecentPenalty)
		}
		if score < 1 {
			score = 1
		}
		out = append(out, Candidate{Clip: candidate.Clip, Score: score})
	}
	return out
}

func (r *Resolver) recentPenalty(clipName string, penalty int) int {
	for index, recentName := range r.state.recent {
		if recentName == clipName {
			return penalty * (len(r.state.recent) - index)
		}
	}
	return 0
}

func chooseWeighted(group []Candidate, rng RNG) Clip {
	if len(group) == 1 || rng == nil {
		return group[0].Clip
	}
	total := 0
	for _, candidate := range group {
		if candidate.Score > 0 {
			total += candidate.Score
		}
	}
	if total <= 0 {
		return group[rng.Intn(len(group))].Clip
	}
	pick := rng.Intn(total)
	for _, candidate := range group {
		if candidate.Score <= 0 {
			continue
		}
		if pick < candidate.Score {
			return candidate.Clip
		}
		pick -= candidate.Score
	}
	return group[len(group)-1].Clip
}
