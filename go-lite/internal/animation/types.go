package animation

import (
	"fmt"
	"math/bits"
	"sort"
)

// TagMask is the runtime representation of semantic animation tags.
// It is intentionally numeric so intent matching stays off the string hot path.
type TagMask uint64

const maxTags = 64

func (m TagMask) HasAll(required TagMask) bool { return m&required == required }

func (m TagMask) HasAny(other TagMask) bool { return m&other != 0 }

func (m TagMask) Count() int { return bits.OnesCount64(uint64(m)) }

// TagRegistry assigns stable bit positions to semantic tags for one compiled pet definition.
type TagRegistry struct {
	byName map[string]TagMask
	names  []string
}

func NewTagRegistry(tags ...string) (*TagRegistry, error) {
	r := &TagRegistry{byName: make(map[string]TagMask, len(tags))}
	for _, tag := range tags {
		if err := r.Register(tag); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *TagRegistry) Register(tag string) error {
	if tag == "" {
		return fmt.Errorf("animation tag must not be empty")
	}
	if _, exists := r.byName[tag]; exists {
		return nil
	}
	if len(r.names) >= maxTags {
		return fmt.Errorf("too many animation tags: max %d", maxTags)
	}
	mask := TagMask(1) << len(r.names)
	r.byName[tag] = mask
	r.names = append(r.names, tag)
	return nil
}

func (r *TagRegistry) Mask(tags ...string) (TagMask, error) {
	var mask TagMask
	for _, tag := range tags {
		bit, ok := r.byName[tag]
		if !ok {
			return 0, fmt.Errorf("unknown animation tag %q", tag)
		}
		mask |= bit
	}
	return mask, nil
}

func (r *TagRegistry) MustMask(tags ...string) TagMask {
	mask, err := r.Mask(tags...)
	if err != nil {
		panic(err)
	}
	return mask
}

func (r *TagRegistry) Tags(mask TagMask) []string {
	out := make([]string, 0, mask.Count())
	for i, name := range r.names {
		if mask&(TagMask(1)<<i) != 0 {
			out = append(out, name)
		}
	}
	return out
}

// Clip is the resolver-facing immutable animation metadata.
type Clip struct {
	Name     string
	Tags     TagMask
	Priority int
}

// IntentQuery describes one fallback query for an intent.
type IntentQuery struct {
	Required       TagMask
	Preferred      TagMask
	Excluded       TagMask
	PreferredBonus int
	BaseScore      int
}

func (q IntentQuery) Matches(clip Clip) bool {
	return clip.Tags.HasAll(q.Required) && !clip.Tags.HasAny(q.Excluded)
}

func (q IntentQuery) Score(clip Clip) int {
	bonus := q.PreferredBonus
	if bonus == 0 {
		bonus = 10
	}
	return q.BaseScore + clip.Priority + (clip.Tags&q.Preferred).Count()*bonus
}

type IntentDefinition struct {
	Name      string
	Fallbacks []IntentQuery
}

type Candidate struct {
	Clip  Clip
	Score int
}

type CompiledIntent struct {
	Name   string
	Groups [][]Candidate
}

func CompileIntent(def IntentDefinition, clips []Clip) (CompiledIntent, error) {
	if def.Name == "" {
		return CompiledIntent{}, fmt.Errorf("intent name must not be empty")
	}
	if len(def.Fallbacks) == 0 {
		return CompiledIntent{}, fmt.Errorf("intent %q must have at least one fallback query", def.Name)
	}
	compiled := CompiledIntent{Name: def.Name}
	for _, query := range def.Fallbacks {
		group := make([]Candidate, 0)
		for _, clip := range clips {
			if clip.Name == "" {
				return CompiledIntent{}, fmt.Errorf("clip name must not be empty")
			}
			if query.Matches(clip) {
				group = append(group, Candidate{Clip: clip, Score: query.Score(clip)})
			}
		}
		if len(group) > 0 {
			sort.SliceStable(group, func(i, j int) bool {
				if group[i].Score == group[j].Score {
					return group[i].Clip.Name < group[j].Clip.Name
				}
				return group[i].Score > group[j].Score
			})
			compiled.Groups = append(compiled.Groups, group)
		}
	}
	return compiled, nil
}
