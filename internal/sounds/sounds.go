// ABOUTME: Sound discovery package for listing available notification sounds.
// ABOUTME: Pure filesystem scanning with no audio dependencies (CGO-free).

package sounds

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// SoundInfo represents a discovered sound file.
type SoundInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Format      string `json:"format"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

// DiscoverOptions controls which sound sources to scan.
type DiscoverOptions struct {
	PluginRoot     string // Root directory of the plugin (for built-in sounds)
	IncludeBuiltIn bool
	IncludeSystem  bool
	MaxSystemDepth int // Max directory depth for Linux system sounds (default 5)
}

// descriptions maps sound names to human-readable descriptions.
var descriptions = map[string]string{
	// Built-in sounds
	"task-complete":   "Triumphant completion chime",
	"review-complete": "Gentle notification tone",
	"question":        "Attention-grabbing sound",
	"plan-ready":      "Professional planning tone",
	"error":           "Error alert sound",
	// macOS system sounds
	"Glass":     "Crisp, clean chime",
	"Hero":      "Triumphant fanfare",
	"Ping":      "Subtle ping sound",
	"Pop":       "Quick pop sound",
	"Purr":      "Gentle purr",
	"Funk":      "Distinctive funk groove",
	"Sosumi":    "Pleasant notification",
	"Basso":     "Deep bass sound",
	"Blow":      "Breeze-like whoosh",
	"Frog":      "Unique ribbit sound",
	"Submarine": "Sonar-like ping",
	"Bottle":    "Cork pop sound",
	"Morse":     "Morse code beeps",
	"Tink":      "Light metallic sound",
}

// Discover scans for available sounds and returns them grouped by source.
// Built-in sounds are listed first, then system sounds.
func Discover(opts DiscoverOptions) []SoundInfo {
	var result []SoundInfo

	if opts.IncludeBuiltIn {
		result = append(result, discoverBuiltIn(opts.PluginRoot)...)
	}

	if opts.IncludeSystem {
		depth := opts.MaxSystemDepth
		if depth <= 0 {
			depth = 5
		}
		result = append(result, discoverSystem(depth)...)
	}

	// Sort within each source group for stable output
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Source != result[j].Source {
			// built-in first, then system
			return result[i].Source == "builtin"
		}
		return result[i].Name < result[j].Name
	})

	return result
}

// FindByName searches for a sound by name with 3-level matching:
// 1. Exact match
// 2. Case-insensitive match
// 3. Prefix match (case-insensitive)
// Built-in sounds are prioritized over system sounds at every level.
func FindByName(name string, available []SoundInfo) (SoundInfo, bool) {
	nameLower := strings.ToLower(name)

	// Level 1: exact match (prefer built-in)
	if s, ok := findPreferBuiltIn(available, func(s SoundInfo) bool {
		return s.Name == name
	}); ok {
		return s, true
	}

	// Level 2: case-insensitive match (prefer built-in)
	if s, ok := findPreferBuiltIn(available, func(s SoundInfo) bool {
		return strings.ToLower(s.Name) == nameLower
	}); ok {
		return s, true
	}

	// Level 3: prefix match (prefer built-in)
	if s, ok := findPreferBuiltIn(available, func(s SoundInfo) bool {
		return strings.HasPrefix(strings.ToLower(s.Name), nameLower)
	}); ok {
		return s, true
	}

	return SoundInfo{}, false
}

// findPreferBuiltIn finds the first match, preferring built-in over system sources.
func findPreferBuiltIn(available []SoundInfo, match func(SoundInfo) bool) (SoundInfo, bool) {
	var firstNonBuiltIn *SoundInfo
	for i, s := range available {
		if match(s) {
			if s.Source == "builtin" {
				return s, true
			}
			if firstNonBuiltIn == nil {
				firstNonBuiltIn = &available[i]
			}
		}
	}
	if firstNonBuiltIn != nil {
		return *firstNonBuiltIn, true
	}
	return SoundInfo{}, false
}

// findSoundsDirectory locates the plugin's sounds/ directory.
func findSoundsDirectory(pluginRoot string) string {
	if pluginRoot != "" {
		dir := filepath.Join(pluginRoot, "sounds")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	// Try relative to this source file (development mode)
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		// internal/sounds/sounds.go -> project root
		projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		dir := filepath.Join(projectRoot, "sounds")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	// Try CLAUDE_PLUGIN_ROOT env
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		dir := filepath.Join(root, "sounds")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	return ""
}

// discoverBuiltIn scans the plugin's sounds/ directory for built-in MP3 files.
func discoverBuiltIn(pluginRoot string) []SoundInfo {
	dir := findSoundsDirectory(pluginRoot)
	if dir == "" {
		return nil
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.mp3"))
	if err != nil {
		return nil
	}

	var result []SoundInfo
	for _, path := range matches {
		name := strings.TrimSuffix(filepath.Base(path), ".mp3")
		result = append(result, SoundInfo{
			Name:        name,
			Path:        path,
			Format:      "mp3",
			Source:      "builtin",
			Description: descriptions[name],
		})
	}

	return result
}

// discoverSystem scans platform-specific system sound directories.
func discoverSystem(maxDepth int) []SoundInfo {
	switch runtime.GOOS {
	case "darwin":
		return discoverMacOSSounds()
	case "linux":
		return discoverLinuxSounds(maxDepth)
	case "windows":
		return discoverWindowsSounds()
	default:
		return nil
	}
}

// discoverMacOSSounds finds AIFF files in /System/Library/Sounds/.
func discoverMacOSSounds() []SoundInfo {
	dir := "/System/Library/Sounds"
	matches, err := filepath.Glob(filepath.Join(dir, "*.aiff"))
	if err != nil {
		return nil
	}

	var result []SoundInfo
	for _, path := range matches {
		name := strings.TrimSuffix(filepath.Base(path), ".aiff")
		result = append(result, SoundInfo{
			Name:        name,
			Path:        path,
			Format:      "aiff",
			Source:      "system",
			Description: descriptions[name],
		})
	}

	return result
}

// discoverLinuxSounds walks /usr/share/sounds/ for OGG and WAV files.
func discoverLinuxSounds(maxDepth int) []SoundInfo {
	baseDir := "/usr/share/sounds"
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil
	}

	var result []SoundInfo
	baseDepth := strings.Count(baseDir, string(os.PathSeparator))

	_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors silently
		}

		// Limit depth
		currentDepth := strings.Count(path, string(os.PathSeparator)) - baseDepth
		if d.IsDir() && currentDepth >= maxDepth {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ogg" && ext != ".wav" {
			return nil
		}

		name := strings.TrimSuffix(filepath.Base(path), ext)
		result = append(result, SoundInfo{
			Name:        name,
			Path:        path,
			Format:      ext[1:], // remove leading dot
			Source:      "system",
			Description: descriptions[name],
		})

		return nil
	})

	return result
}

// discoverWindowsSounds finds WAV files in %SYSTEMROOT%/Media/.
func discoverWindowsSounds() []SoundInfo {
	sysRoot := os.Getenv("SYSTEMROOT")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}

	dir := filepath.Join(sysRoot, "Media")
	matches, err := filepath.Glob(filepath.Join(dir, "*.wav"))
	if err != nil {
		return nil
	}

	var result []SoundInfo
	for _, path := range matches {
		name := strings.TrimSuffix(filepath.Base(path), ".wav")
		result = append(result, SoundInfo{
			Name:        name,
			Path:        path,
			Format:      "wav",
			Source:      "system",
			Description: descriptions[name],
		})
	}

	return result
}
