package audio

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-audio/audio"
)

// === ListDevices Tests ===

func TestListDevices(t *testing.T) {
	devices, err := ListDevices()
	if err != nil {
		// In CI environments without audio, this may fail
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("ListDevices() error: %v", err)
	}

	// Should return at least one device on systems with audio
	if len(devices) == 0 {
		t.Log("Warning: no audio devices found (may be expected in headless environments)")
	}

	// If devices exist, check structure
	for i, dev := range devices {
		if dev.Name == "" {
			t.Errorf("Device %d has empty name", i)
		}
		t.Logf("Device %d: %s (default: %v)", i, dev.Name, dev.IsDefault)
	}
}

func TestListDevices_HasDefault(t *testing.T) {
	devices, err := ListDevices()
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("ListDevices() error: %v", err)
	}

	if len(devices) == 0 {
		t.Skip("No audio devices available")
	}

	// At least one device should be marked as default
	hasDefault := false
	for _, dev := range devices {
		if dev.IsDefault {
			hasDefault = true
			break
		}
	}

	if !hasDefault {
		t.Log("Warning: no default device found (may vary by platform)")
	}
}

// === NewPlayer Tests ===

func TestNewPlayer_DefaultDevice(t *testing.T) {
	player, err := NewPlayer("", 1.0)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("NewPlayer() error: %v", err)
	}
	defer player.Close()

	if player.ctx == nil {
		t.Error("NewPlayer() ctx is nil")
	}
	if player.volume != 1.0 {
		t.Errorf("NewPlayer() volume = %v, want 1.0", player.volume)
	}
	if player.deviceName != "" {
		t.Errorf("NewPlayer() deviceName = %q, want empty", player.deviceName)
	}
}

func TestNewPlayer_WithVolume(t *testing.T) {
	testCases := []float64{0.0, 0.3, 0.5, 0.7, 1.0}

	for _, vol := range testCases {
		player, err := NewPlayer("", vol)
		if err != nil {
			if os.Getenv("CI") != "" {
				t.Skipf("Skipping in CI: %v", err)
			}
			t.Fatalf("NewPlayer(volume=%v) error: %v", vol, err)
		}

		if player.volume != vol {
			t.Errorf("NewPlayer(volume=%v) got volume = %v", vol, player.volume)
		}
		player.Close()
	}
}

func TestNewPlayer_NonExistentDevice(t *testing.T) {
	player, err := NewPlayer("NonExistentDevice12345XYZ", 1.0)

	if err == nil {
		player.Close()
		t.Error("NewPlayer() expected error for non-existent device, got nil")
	}

	if player != nil {
		t.Error("NewPlayer() should return nil player on error")
	}
}

func TestNewPlayer_SpecificDevice(t *testing.T) {
	// First get available devices
	devices, err := ListDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("No audio devices available")
	}

	// Try to create player with the first device
	deviceName := devices[0].Name
	player, err := NewPlayer(deviceName, 0.5)
	if err != nil {
		t.Fatalf("NewPlayer(%q) error: %v", deviceName, err)
	}
	defer player.Close()

	if player.deviceName != deviceName {
		t.Errorf("NewPlayer() deviceName = %q, want %q", player.deviceName, deviceName)
	}
}

// === Player.Close Tests ===

func TestPlayer_Close(t *testing.T) {
	player, err := NewPlayer("", 1.0)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("NewPlayer() error: %v", err)
	}

	// First close should succeed
	err = player.Close()
	if err != nil {
		t.Errorf("Close() first call error: %v", err)
	}

	// Second close should be safe (idempotent)
	err = player.Close()
	if err != nil {
		t.Errorf("Close() second call error: %v", err)
	}
}

func TestPlayer_CloseNilContext(t *testing.T) {
	// Create a player with nil context to test edge case
	player := &Player{
		ctx:    nil,
		volume: 1.0,
	}

	// Should not panic
	err := player.Close()
	if err != nil {
		t.Errorf("Close() with nil ctx error: %v", err)
	}
}

// === Player.Play Tests ===

func TestPlayer_Play_NonExistentFile(t *testing.T) {
	player, err := NewPlayer("", 1.0)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("NewPlayer() error: %v", err)
	}
	defer player.Close()

	err = player.Play("/nonexistent/path/to/audio.mp3")
	if err == nil {
		t.Error("Play() expected error for non-existent file, got nil")
	}
}

func TestPlayer_Play_UnsupportedFormat(t *testing.T) {
	player, err := NewPlayer("", 1.0)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("NewPlayer() error: %v", err)
	}
	defer player.Close()

	// Create a temp file with unsupported extension
	tmpFile, err := os.CreateTemp("", "test*.xyz")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	err = player.Play(tmpFile.Name())
	if err == nil {
		t.Error("Play() expected error for unsupported format, got nil")
	}
}

func TestPlayer_Play_RealMP3(t *testing.T) {
	// Find sounds directory
	soundsDir := findSoundsDirectory()
	if soundsDir == "" {
		t.Skip("Sounds directory not found")
	}

	mp3Path := filepath.Join(soundsDir, "task-complete.mp3")
	if _, err := os.Stat(mp3Path); os.IsNotExist(err) {
		t.Skip("task-complete.mp3 not found")
	}

	player, err := NewPlayer("", 0.1) // Low volume for tests
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("NewPlayer() error: %v", err)
	}
	defer player.Close()

	// This will actually play the sound (very quietly)
	err = player.Play(mp3Path)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI (no audio device): %v", err)
		}
		t.Errorf("Play(MP3) error: %v", err)
	}
}

func TestPlayer_Play_RealAIFF(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("AIFF test only on macOS")
	}

	aiffPath := "/System/Library/Sounds/Glass.aiff"
	if _, err := os.Stat(aiffPath); os.IsNotExist(err) {
		t.Skip("Glass.aiff not found")
	}

	player, err := NewPlayer("", 0.1)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI: %v", err)
		}
		t.Fatalf("NewPlayer() error: %v", err)
	}
	defer player.Close()

	err = player.Play(aiffPath)
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Skipf("Skipping in CI (no audio device): %v", err)
		}
		t.Errorf("Play(AIFF) error: %v", err)
	}
}

// === intBufferToSamples Tests ===

func TestIntBufferToSamples_8Bit(t *testing.T) {
	buf := &audio.IntBuffer{
		Data: []int{0, 64, 127}, // 8-bit values in safe range
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100,
		},
	}

	samples := intBufferToSamples(buf, 8)

	// 8-bit shifted left by 8 to become 16-bit
	// 0 << 8 = 0, 64 << 8 = 16384, 127 << 8 = 32512
	expected := []int16{0, 16384, 32512}
	for i, s := range samples {
		if s != expected[i] {
			t.Errorf("Sample %d: got %d, want %d", i, s, expected[i])
		}
	}
}

func TestIntBufferToSamples_16Bit(t *testing.T) {
	buf := &audio.IntBuffer{
		Data: []int{0, 16384, 32767, -32768},
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100,
		},
	}

	samples := intBufferToSamples(buf, 16)

	// 16-bit: no conversion
	expected := []int16{0, 16384, 32767, -32768}
	for i, s := range samples {
		if s != expected[i] {
			t.Errorf("Sample %d: got %d, want %d", i, s, expected[i])
		}
	}
}

func TestIntBufferToSamples_24Bit(t *testing.T) {
	buf := &audio.IntBuffer{
		Data: []int{0, 8388607, -8388608}, // 24-bit max/min
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100,
		},
	}

	samples := intBufferToSamples(buf, 24)

	// 24-bit shifted right by 8
	expected := []int16{0, 8388607 >> 8, int16(-8388608 >> 8)}
	for i, s := range samples {
		if s != expected[i] {
			t.Errorf("Sample %d: got %d, want %d", i, s, expected[i])
		}
	}
}

func TestIntBufferToSamples_32Bit(t *testing.T) {
	buf := &audio.IntBuffer{
		Data: []int{0, 2147483647, -2147483648}, // 32-bit max/min
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100,
		},
	}

	samples := intBufferToSamples(buf, 32)

	// 32-bit shifted right by 16
	expected := []int16{0, int16(2147483647 >> 16), int16(-2147483648 >> 16)}
	for i, s := range samples {
		if s != expected[i] {
			t.Errorf("Sample %d: got %d, want %d", i, s, expected[i])
		}
	}
}

func TestIntBufferToSamples_UnknownBitDepth(t *testing.T) {
	buf := &audio.IntBuffer{
		Data: []int{100, 200, 300},
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  44100,
		},
	}

	// Unknown bit depth should fallback to 16-bit behavior
	samples := intBufferToSamples(buf, 12) // Unusual bit depth

	expected := []int16{100, 200, 300}
	for i, s := range samples {
		if s != expected[i] {
			t.Errorf("Sample %d: got %d, want %d", i, s, expected[i])
		}
	}
}

// === samplesToBytes Tests ===

func TestSamplesToBytes(t *testing.T) {
	samples := []int16{0, 256, -1, 32767, -32768}
	bytes := samplesToBytes(samples)

	if len(bytes) != len(samples)*2 {
		t.Errorf("samplesToBytes() length = %d, want %d", len(bytes), len(samples)*2)
	}

	// Verify little-endian encoding
	testCases := []struct {
		idx      int
		sample   int16
		lowByte  byte
		highByte byte
	}{
		{0, 0, 0x00, 0x00},
		{1, 256, 0x00, 0x01},    // 256 = 0x0100
		{2, -1, 0xFF, 0xFF},     // -1 = 0xFFFF
		{3, 32767, 0xFF, 0x7F},  // 32767 = 0x7FFF
		{4, -32768, 0x00, 0x80}, // -32768 = 0x8000
	}

	for _, tc := range testCases {
		lowIdx := tc.idx * 2
		highIdx := tc.idx*2 + 1
		if bytes[lowIdx] != tc.lowByte || bytes[highIdx] != tc.highByte {
			t.Errorf("Sample %d (%d): got [%02X %02X], want [%02X %02X]",
				tc.idx, tc.sample, bytes[lowIdx], bytes[highIdx], tc.lowByte, tc.highByte)
		}
	}
}

func TestSamplesToBytes_Empty(t *testing.T) {
	samples := []int16{}
	bytes := samplesToBytes(samples)

	if len(bytes) != 0 {
		t.Errorf("samplesToBytes(empty) length = %d, want 0", len(bytes))
	}
}

// === Helper Functions ===

func findSoundsDirectory() string {
	// Try different paths
	paths := []string{
		"sounds",
		"../../sounds",
		"../../../sounds",
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			absPath, _ := filepath.Abs(p)
			return absPath
		}
	}

	return ""
}
