package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func loadRuntimeProfile(profilePath, assetsPath string, petSelect string) (Profile, string, error) {
	if profilePath != "" {
		p, err := LoadProfile(profilePath)
		return p, filepath.Dir(profilePath), err
	}

	petsRoot := filepath.Join(assetsPath, "pets")
	petDirs, err := DiscoverPetDirs(petsRoot)
	if err != nil {
		return Profile{}, "", err
	}
	if len(petDirs) == 0 {
		return Profile{}, "", fmt.Errorf("no pets discovered in %s", petsRoot)
	}

	available := make([]string, 0, len(petDirs))
	availableSet := make(map[string]bool, len(petDirs))
	for _, petDir := range petDirs {
		id := filepath.Base(petDir)
		available = append(available, id)
		availableSet[id] = true
	}

	selected, err := parsePetSelection(petSelect, available)
	if err != nil {
		return Profile{}, "", fmt.Errorf("%w; petsRoot=%s available=%s", err, petsRoot, strings.Join(available, ","))
	}

	profile := Profile{Schema: 2, BaseDir: "."}
	for _, id := range selected {
		if !availableSet[id] {
			return Profile{}, "", fmt.Errorf("selected pet %q not found; petsRoot=%s available=%s", id, petsRoot, strings.Join(available, ","))
		}
		profile.ActivePets = append(profile.ActivePets, ActivePetConfig{
			ID:        id,
			Name:      id,
			PetID:     id,
			Enabled:   true,
			Count:     1,
			Scale:     0,
			Home:      "bottom",
			AutoRoam:  true,
			AllowDrag: true,
		})
	}
	return profile, ".", nil
}

func parsePetSelection(value string, available []string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		if len(available) == 0 {
			return nil, fmt.Errorf("no pets are available")
		}
		return []string{available[0]}, nil
	}

	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "all" {
			return append([]string(nil), available...), nil
		}
		if !seen[part] {
			seen[part] = true
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty pet selection %q", value)
	}
	return out, nil
}
