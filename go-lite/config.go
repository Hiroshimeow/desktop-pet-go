package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type AnimationDef struct {
	File         string  `json:"file"`
	FPS          int     `json:"fps"`
	Loop         bool    `json:"loop"`
	Kind         string  `json:"kind"`
	Locomotion   bool    `json:"locomotion"`
	SpeedPxS     float64 `json:"speed_px_s"`
	DurationMS   int     `json:"duration_ms"`
	DurationS    float64 `json:"duration_s"`
	NativeFacing string  `json:"native_facing"`
	Frames       int     `json:"frames"`
	Description  string  `json:"description"`
}

type Hitbox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type MotionConfig struct {
	AutoRoamChance        int `json:"auto_roam_chance"`
	MinIdleMS             int `json:"min_idle_ms"`
	MaxIdleMS             int `json:"max_idle_ms"`
	WalkDistanceThreshold int `json:"walk_distance_threshold"`
	ScreenMargin          int `json:"screen_margin"`
}

type InteractionAction struct {
	Animation  string   `json:"animation"`
	Random     []string `json:"random"`
	DurationMS int      `json:"duration_ms"`
	IntervalMS int      `json:"interval_ms"`
}

type PetManifest struct {
	Schema                  int                          `json:"schema"`
	ID                      string                       `json:"id"`
	Name                    string                       `json:"name"`
	Scale                   float64                      `json:"scale"`
	FrameWidth              int                          `json:"frame_width"`
	FrameHeight             int                          `json:"frame_height"`
	Columns                 int                          `json:"columns"`
	DefaultAnimation        string                       `json:"default_animation"`
	AnimationDir            string                       `json:"animation_dir"`
	BaselineY               int                          `json:"baseline_y"`
	Hitbox                  Hitbox                       `json:"hitbox"`
	Motion                  MotionConfig                 `json:"motion"`
	Interactions            map[string]InteractionAction `json:"interactions"`
	ActBlacklist            []string                     `json:"act_blacklist"`
	UnknownAnimationDefault AnimationDef                 `json:"unknown_animation_default"`
	Animations              map[string]AnimationDef      `json:"animations"`
	BaseDir                 string                       `json:"-"`
}

type ActivePetConfig struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	PetID     string  `json:"pet_id"`
	Manifest  string  `json:"manifest"`
	Enabled   bool    `json:"enabled"`
	Count     int     `json:"count"`
	Scale     float64 `json:"scale"`
	Home      string  `json:"home"`
	AutoRoam  bool    `json:"auto_roam"`
	AllowDrag bool    `json:"allow_drag"`
}

type ControllerConfig struct {
	Enabled     bool   `json:"enabled"`
	Hotkey      string `json:"hotkey"`
	Description string `json:"description"`
}

type Profile struct {
	Schema     int               `json:"schema"`
	ActivePets []ActivePetConfig `json:"active_pets"`
	Controller ControllerConfig  `json:"controller"`
	BaseDir    string            `json:"-"`
}

func LoadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, err
	}
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, err
	}
	profile.BaseDir = filepath.Dir(path)
	if len(profile.ActivePets) == 0 {
		return Profile{}, fmt.Errorf("profile has no active_pets")
	}
	return profile, nil
}

func LoadPetManifest(path string) (PetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PetManifest{}, err
	}
	var m PetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return PetManifest{}, err
	}
	m.BaseDir = filepath.Dir(path)
	if err := m.NormalizeAndValidate(); err != nil {
		return PetManifest{}, err
	}
	return m, nil
}

func (m *PetManifest) NormalizeAndValidate() error {
	if m == nil {
		return fmt.Errorf("pet manifest is nil")
	}
	if m.ID == "" {
		return fmt.Errorf("pet manifest missing id")
	}
	if m.FrameWidth <= 0 || m.FrameHeight <= 0 {
		return fmt.Errorf("frame size must be positive")
	}
	if m.Scale <= 0 {
		m.Scale = 0.5
	}
	if m.Columns <= 0 {
		m.Columns = 5
	}
	if m.AnimationDir == "" {
		return fmt.Errorf("animation_dir is required")
	}
	if _, ok := m.Animations[m.DefaultAnimation]; !ok {
		return fmt.Errorf("default_animation %q not found", m.DefaultAnimation)
	}
	for name, anim := range m.Animations {
		if anim.File == "" {
			return fmt.Errorf("animation %s missing file", name)
		}
		if anim.FPS <= 0 {
			return fmt.Errorf("animation %s must have fps > 0", name)
		}
		if anim.Frames < 0 {
			return fmt.Errorf("animation %s frames must be >= 0", name)
		}
		if anim.Kind == "move" && !anim.Locomotion {
			return fmt.Errorf("move animation %s must set locomotion=true", name)
		}
	}
	if m.Motion.AutoRoamChance <= 0 {
		m.Motion.AutoRoamChance = 35
	}
	return nil
}

func (m PetManifest) Validate() error {
	copy := m
	return (&copy).NormalizeAndValidate()
}

func (m PetManifest) AnimationPath(anim AnimationDef) string {
	p := filepath.FromSlash(filepath.Join(m.AnimationDir, anim.File))
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(m.BaseDir, p)
}

func frameCountOf(m PetManifest, anim AnimationDef) int {
	if anim.Frames > 0 {
		return anim.Frames
	}
	if m.Columns > 0 {
		return m.Columns
	}
	return 5
}

func durationOf(anim AnimationDef) int {
	if anim.DurationMS > 0 {
		return anim.DurationMS
	}
	if anim.DurationS > 0 {
		return int(anim.DurationS * 1000)
	}
	return 1500
}
