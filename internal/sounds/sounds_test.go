package sounds

import (
	"runtime"
	"testing"
)

func TestListBuiltIn(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  false,
	})

	if len(sounds) != 5 {
		t.Fatalf("expected 5 built-in sounds, got %d", len(sounds))
	}

	expected := map[string]bool{
		"task-complete":   false,
		"review-complete": false,
		"question":        false,
		"plan-ready":      false,
		"error":           false,
	}

	for _, s := range sounds {
		if _, ok := expected[s.Name]; !ok {
			t.Errorf("unexpected built-in sound: %s", s.Name)
			continue
		}
		expected[s.Name] = true

		if s.Source != "builtin" {
			t.Errorf("sound %s: expected source=builtin, got %s", s.Name, s.Source)
		}
		if s.Format != "mp3" {
			t.Errorf("sound %s: expected format=mp3, got %s", s.Name, s.Format)
		}
		if s.Path == "" {
			t.Errorf("sound %s: path is empty", s.Name)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("expected built-in sound not found: %s", name)
		}
	}
}

func TestListBuiltIn_MissingDir(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		PluginRoot:     "/nonexistent/path/that/does/not/exist",
		IncludeBuiltIn: true,
		IncludeSystem:  false,
	})

	// Should find sounds via runtime.Caller fallback (development mode)
	// If running from a non-standard location, may be empty — that's OK
	if len(sounds) > 0 {
		// Verify they're still valid
		for _, s := range sounds {
			if s.Source != "builtin" {
				t.Errorf("unexpected source: %s", s.Source)
			}
		}
	}
}

func TestListSystem(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: false,
		IncludeSystem:  true,
	})

	switch runtime.GOOS {
	case "darwin":
		if len(sounds) == 0 {
			t.Error("expected system sounds on macOS, got none")
		}
		for _, s := range sounds {
			if s.Source != "system" {
				t.Errorf("expected source=system, got %s", s.Source)
			}
			if s.Format != "aiff" {
				t.Errorf("expected format=aiff on macOS, got %s", s.Format)
			}
		}
	case "linux":
		// Linux may or may not have system sounds
		t.Logf("found %d system sounds on Linux", len(sounds))
	case "windows":
		t.Logf("found %d system sounds on Windows", len(sounds))
	}
}

func TestFindByName(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  true,
	})

	// Test exact match for all 4 built-in sounds
	builtins := []string{"task-complete", "review-complete", "question", "plan-ready"}
	for _, name := range builtins {
		s, found := FindByName(name, sounds)
		if !found {
			t.Errorf("FindByName(%q): not found", name)
			continue
		}
		if s.Name != name {
			t.Errorf("FindByName(%q): got name=%q", name, s.Name)
		}
		if s.Source != "builtin" {
			t.Errorf("FindByName(%q): expected source=builtin, got %s", name, s.Source)
		}
	}
}

func TestFindByName_CaseInsensitive(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  false,
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"Task-Complete", "task-complete"},
		{"QUESTION", "question"},
		{"Plan-Ready", "plan-ready"},
		{"REVIEW-COMPLETE", "review-complete"},
	}

	for _, tc := range tests {
		s, found := FindByName(tc.input, sounds)
		if !found {
			t.Errorf("FindByName(%q): not found", tc.input)
			continue
		}
		if s.Name != tc.expected {
			t.Errorf("FindByName(%q): expected %q, got %q", tc.input, tc.expected, s.Name)
		}
	}
}

func TestFindByName_PrefixMatch(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  false,
	})

	s, found := FindByName("task", sounds)
	if !found {
		t.Fatal("prefix match should find 'task-complete'")
	}
	if s.Name != "task-complete" {
		t.Errorf("expected 'task-complete', got %q", s.Name)
	}
}

func TestFindByName_PrioritizeBuiltIn(t *testing.T) {
	// System first in slice, built-in second — built-in should still win at every level
	list := []SoundInfo{
		{Name: "test-sound", Source: "system", Path: "/sys/test.aiff"},
		{Name: "test-sound", Source: "builtin", Path: "/builtin/test.mp3"},
	}

	// Exact match: built-in preferred even though system is first in slice
	s, found := FindByName("test-sound", list)
	if !found {
		t.Fatal("should find sound")
	}
	if s.Source != "builtin" {
		t.Errorf("exact match should prefer built-in, got source=%s", s.Source)
	}

	// Case-insensitive match: built-in preferred
	s, found = FindByName("Test-Sound", list)
	if !found {
		t.Fatal("case-insensitive match should find sound")
	}
	if s.Source != "builtin" {
		t.Errorf("case-insensitive match should prefer built-in, got source=%s", s.Source)
	}

	// Prefix match: built-in preferred
	s, found = FindByName("test-so", list)
	if !found {
		t.Fatal("prefix match should find sound")
	}
	if s.Source != "builtin" {
		t.Errorf("prefix match should prefer built-in, got source=%s", s.Source)
	}
}

func TestFindByName_NotFound(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  false,
	})

	_, found := FindByName("nonexistent-sound-xyz", sounds)
	if found {
		t.Error("FindByName should return false for nonexistent sound")
	}
}

func TestFindByName_EmptyList(t *testing.T) {
	_, found := FindByName("any", []SoundInfo{})
	if found {
		t.Error("should not find in empty list")
	}
}

func TestListAll(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  true,
	})

	if len(sounds) < 4 {
		t.Errorf("expected at least 4 sounds (built-in), got %d", len(sounds))
	}

	if runtime.GOOS == "darwin" {
		hasSystem := false
		for _, s := range sounds {
			if s.Source == "system" {
				hasSystem = true
				break
			}
		}
		if !hasSystem {
			t.Error("expected system sounds on macOS")
		}
	}
}

func TestDiscoverSorted(t *testing.T) {
	result := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  true,
	})

	if len(result) < 2 {
		t.Skip("not enough sounds to test sorting")
	}

	// Built-in should come before system
	seenSystem := false
	for _, s := range result {
		if s.Source == "system" {
			seenSystem = true
		}
		if s.Source == "builtin" && seenSystem {
			t.Error("built-in sounds should come before system sounds")
			break
		}
	}

	// Within each group, names should be sorted
	var lastBuiltIn, lastSystem string
	for _, s := range result {
		switch s.Source {
		case "builtin":
			if s.Name < lastBuiltIn {
				t.Errorf("built-in sounds not sorted: %q after %q", s.Name, lastBuiltIn)
			}
			lastBuiltIn = s.Name
		case "system":
			if s.Name < lastSystem {
				t.Errorf("system sounds not sorted: %q after %q", s.Name, lastSystem)
			}
			lastSystem = s.Name
		}
	}
}

func TestDescriptions(t *testing.T) {
	sounds := Discover(DiscoverOptions{
		IncludeBuiltIn: true,
		IncludeSystem:  false,
	})

	for _, s := range sounds {
		if s.Description == "" {
			t.Errorf("built-in sound %q has no description", s.Name)
		}
	}
}
