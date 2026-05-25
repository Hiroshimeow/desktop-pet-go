package main

import (
	"fmt"
	"sort"
)

func printCatalog(m PetManifest) {
	for _, name := range sortedAnimationNames(m.Animations) {
		if a, ok := m.Animations[name]; ok {
			fmt.Printf("pet=%s anim=%s file=%s frames=%d fps=%d locomotion=%v speed=%.0f desc=%s\n", m.ID, name, a.File, frameCountOf(m, a), a.FPS, a.Locomotion, a.SpeedPxS, a.Description)
		}
	}
}

func sortedAnimationNames(anims map[string]AnimationDef) []string {
	names := make([]string, 0, len(anims))
	for name := range anims {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
