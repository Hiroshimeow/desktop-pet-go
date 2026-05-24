package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PetCache struct {
	Schema       int               `json:"schema"`
	Fingerprint  string            `json:"fingerprint"`
	DefaultHash  string            `json:"default_hash"`
	PetJSONHash  string            `json:"pet_json_hash"`
	AnimationSig map[string]string `json:"animation_sig"`
}

func DiscoverPetDirs(petsRoot string) ([]string, error) {
	entries, err := os.ReadDir(petsRoot)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		petDir := filepath.Join(petsRoot, e.Name())
		if _, err := os.Stat(filepath.Join(petDir, "animations")); err == nil {
			out = append(out, petDir)
		}
	}
	sort.Strings(out)
	return out, nil
}

func LoadPetManifestSynced(defaultPath, petDir string) (PetManifest, error) {
	base, err := readManifestRaw(defaultPath)
	if err != nil {
		return PetManifest{}, err
	}
	base.BaseDir = filepath.Dir(defaultPath)
	petPath := filepath.Join(petDir, "pet.json")
	if _, err := os.Stat(petPath); os.IsNotExist(err) {
		m := mergeForPet(base, PetManifest{}, petDir)
		if err := scanAndSyncAnimations(&m, base); err != nil {
			return PetManifest{}, err
		}
		if err := writeManifest(petPath, m); err != nil {
			return PetManifest{}, err
		}
		_ = writeCache(defaultPath, petPath, m)
		return finalizedManifest(m, petDir)
	}
	pet, err := readManifestRaw(petPath)
	if err != nil {
		return PetManifest{}, err
	}
	needsSync, _ := shouldSync(defaultPath, petPath, petDir)
	m := mergeForPet(base, pet, petDir)
	if err := scanAndSyncAnimations(&m, base); err != nil {
		return PetManifest{}, err
	}
	if needsSync {
		if err := writeManifest(petPath, m); err != nil {
			return PetManifest{}, err
		}
		_ = writeCache(defaultPath, petPath, m)
	}
	return finalizedManifest(m, petDir)
}

func readManifestRaw(path string) (PetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PetManifest{}, err
	}
	var m PetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return PetManifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	return m, nil
}

func mergeForPet(base, pet PetManifest, petDir string) PetManifest {
	m := base
	if pet.Schema != 0 {
		m.Schema = pet.Schema
	}
	m.ID = filepath.Base(petDir)
	m.Name = firstNonEmpty(pet.Name, m.ID)
	if pet.Scale > 0 {
		m.Scale = pet.Scale
	}
	if m.Scale <= 0 {
		m.Scale = 0.5
	}
	if pet.FrameWidth > 0 {
		m.FrameWidth = pet.FrameWidth
	}
	if pet.FrameHeight > 0 {
		m.FrameHeight = pet.FrameHeight
	}
	if pet.Columns > 0 {
		m.Columns = pet.Columns
	}
	if pet.DefaultAnimation != "" {
		m.DefaultAnimation = pet.DefaultAnimation
	}
	if pet.AnimationDir != "" {
		m.AnimationDir = pet.AnimationDir
	}
	if pet.BaselineY > 0 {
		m.BaselineY = pet.BaselineY
	}
	if pet.Hitbox.W > 0 && pet.Hitbox.H > 0 {
		m.Hitbox = pet.Hitbox
	}
	m.Motion = mergeMotion(base.Motion, pet.Motion)
	m.Interactions = mergeInteractions(base.Interactions, pet.Interactions)
	m.ActBlacklist = append([]string{}, base.ActBlacklist...)
	m.ActBlacklist = append(m.ActBlacklist, pet.ActBlacklist...)
	m.UnknownAnimationDefault = mergeAnim(base.UnknownAnimationDefault, pet.UnknownAnimationDefault)
	m.Animations = map[string]AnimationDef{}
	for k, v := range base.Animations {
		m.Animations[k] = v
	}
	for k, v := range pet.Animations {
		m.Animations[k] = mergeAnim(m.Animations[k], v)
	}
	m.BaseDir = petDir
	return m
}

func finalizedManifest(m PetManifest, petDir string) (PetManifest, error) {
	m.BaseDir = petDir
	if m.ID == "" {
		m.ID = filepath.Base(petDir)
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if err := m.Validate(); err != nil {
		return PetManifest{}, err
	}
	return m, nil
}

func scanAndSyncAnimations(m *PetManifest, base PetManifest) error {
	animDir := filepath.Join(m.BaseDir, m.AnimationDir)
	entries, err := os.ReadDir(animDir)
	if err != nil {
		return err
	}
	if m.Animations == nil {
		m.Animations = map[string]AnimationDef{}
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".png" {
			continue
		}
		act := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if isActBlacklisted(act, m.ActBlacklist) {
			continue
		}
		seen[act] = true
		def, ok := m.Animations[act]
		if !ok {
			if baseDef, exists := base.Animations[act]; exists {
				def = baseDef
			} else {
				def = inferAnimationDefault(act, m.UnknownAnimationDefault)
			}
		}
		def.File = e.Name()
		m.Animations[act] = def
	}
	for act, def := range m.Animations {
		if def.File == "" || !seen[act] {
			delete(m.Animations, act)
		}
	}
	if _, ok := m.Animations[m.DefaultAnimation]; !ok {
		if _, ok := m.Animations["idle"]; ok {
			m.DefaultAnimation = "idle"
		}
	}
	sanitizeInteractions(m)
	return nil
}

func sanitizeInteractions(m *PetManifest) {
	for name, action := range m.Interactions {
		if action.Animation != "" {
			if _, ok := m.Animations[action.Animation]; !ok {
				action.Animation = ""
			}
		}
		if len(action.Random) > 0 {
			filtered := action.Random[:0]
			for _, candidate := range action.Random {
				if _, ok := m.Animations[candidate]; ok {
					filtered = append(filtered, candidate)
				}
			}
			action.Random = filtered
		}
		if action.Animation == "" && len(action.Random) == 0 {
			delete(m.Interactions, name)
			continue
		}
		m.Interactions[name] = action
	}
}

func isActBlacklisted(act string, blacklist []string) bool {
	for _, item := range blacklist {
		item = strings.TrimSpace(strings.ToLower(item))
		if item == "" {
			continue
		}
		name := strings.ToLower(act)
		if item == name {
			return true
		}
		if strings.HasSuffix(item, "*") && strings.HasPrefix(name, strings.TrimSuffix(item, "*")) {
			return true
		}
		if strings.HasPrefix(item, "*") && strings.HasSuffix(name, strings.TrimPrefix(item, "*")) {
			return true
		}
	}
	return false
}

func inferAnimationDefault(act string, base AnimationDef) AnimationDef {
	def := base
	if def.FPS <= 0 {
		def.FPS = 5
	}
	if def.Kind == "" {
		def.Kind = "action"
	}
	if def.DurationMS <= 0 {
		def.DurationMS = 1500
	}
	name := strings.ToLower(act)
	switch name {
	case "walk":
		def.Kind = "move"
		def.Locomotion = true
		def.SpeedPxS = 42
		def.NativeFacing = "right"
	case "run":
		def.Kind = "move"
		def.Locomotion = true
		def.SpeedPxS = 82
		def.NativeFacing = "right"
	case "fly":
		def.Kind = "move"
		def.Locomotion = true
		def.SpeedPxS = 90
	case "idle", "sleepy", "sit_idle", "thinking":
		def.Kind = "state"
		def.Locomotion = false
	case "happy", "cry", "angry", "shy", "surprised", "scared", "dizzy":
		def.Kind = "emotion"
		def.Locomotion = false
	case "wave", "dance", "cheer", "fight":
		def.Kind = "reaction"
		def.Locomotion = false
	}
	def.Description = firstNonEmpty(def.Description, "Auto-added animation from animations/"+act+".png")
	return def
}

func shouldSync(defaultPath, petPath, petDir string) (bool, error) {
	cachePath := filepath.Join(petDir, ".petcache.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return true, nil
	}
	var cache PetCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return true, nil
	}
	defaultHash, _ := fileHash(defaultPath)
	petHash, _ := fileHash(petPath)
	animSig, _ := animationSignature(filepath.Join(petDir, "animations"))
	fp := makeFingerprint(defaultHash, petHash, animSig)
	return fp != cache.Fingerprint, nil
}

func writeCache(defaultPath, petPath string, m PetManifest) error {
	defaultHash, _ := fileHash(defaultPath)
	petHash, _ := fileHash(petPath)
	animSig, _ := animationSignature(filepath.Join(m.BaseDir, m.AnimationDir))
	cache := PetCache{Schema: 1, Fingerprint: makeFingerprint(defaultHash, petHash, animSig), DefaultHash: defaultHash, PetJSONHash: petHash, AnimationSig: animSig}
	data, _ := json.MarshalIndent(cache, "", "  ")
	return os.WriteFile(filepath.Join(m.BaseDir, ".petcache.json"), data, 0644)
}

func animationSignature(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return map[string]string{}, err
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".png" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out[e.Name()] = fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	}
	return out, nil
}

func makeFingerprint(defaultHash, petHash string, animSig map[string]string) string {
	h := sha1.New()
	io.WriteString(h, defaultHash)
	io.WriteString(h, petHash)
	keys := make([]string, 0, len(animSig))
	for k := range animSig {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		io.WriteString(h, k+"="+animSig[k]+";")
	}
	return hex.EncodeToString(h.Sum(nil))
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha1.Sum(data)
	return hex.EncodeToString(h[:]), nil
}

func writeManifest(path string, m PetManifest) error {
	copy := m
	copy.BaseDir = ""
	data, err := json.MarshalIndent(copy, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func mergeMotion(a, b MotionConfig) MotionConfig {
	if b.AutoRoamChance != 0 {
		a.AutoRoamChance = b.AutoRoamChance
	}
	if b.MinIdleMS != 0 {
		a.MinIdleMS = b.MinIdleMS
	}
	if b.MaxIdleMS != 0 {
		a.MaxIdleMS = b.MaxIdleMS
	}
	if b.WalkDistanceThreshold != 0 {
		a.WalkDistanceThreshold = b.WalkDistanceThreshold
	}
	if b.ScreenMargin != 0 {
		a.ScreenMargin = b.ScreenMargin
	}
	return a
}
func mergeInteractions(a, b map[string]InteractionAction) map[string]InteractionAction {
	out := map[string]InteractionAction{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
func mergeAnim(a, b AnimationDef) AnimationDef {
	if b.File != "" {
		a.File = b.File
	}
	if b.FPS != 0 {
		a.FPS = b.FPS
	}
	if b.Loop {
		a.Loop = b.Loop
	}
	if b.Kind != "" {
		a.Kind = b.Kind
	}
	if b.Locomotion {
		a.Locomotion = b.Locomotion
	}
	if b.SpeedPxS != 0 {
		a.SpeedPxS = b.SpeedPxS
	}
	if b.DurationMS != 0 {
		a.DurationMS = b.DurationMS
	}
	if b.DurationS != 0 {
		a.DurationS = b.DurationS
	}
	if b.NativeFacing != "" {
		a.NativeFacing = b.NativeFacing
	}
	if b.Frames != 0 {
		a.Frames = b.Frames
	}
	if b.Description != "" {
		a.Description = b.Description
	}
	return a
}
func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
