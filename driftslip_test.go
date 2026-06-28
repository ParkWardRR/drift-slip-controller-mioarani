package driftslip

import (
	"bytes"
	"math"
	"testing"
)

// mockDrift implements DriftSource
type mockDrift struct {
	ppm float64
}

func (m mockDrift) DriftPPM() float64 {
	return m.ppm
}

func TestSlipController_DropSample(t *testing.T) {
	sc := NewSlipController(DriftCorrectionSlip, 2, 2)
	
	// Create a buffer of 10 stereo frames (4 bytes per frame)
	frameCount := 10
	frameSize := 4
	buf := make([]byte, frameCount*frameSize)
	
	// Fast capture clock (+100,000 ppm drift = +1 sample per 10 frames)
	// Accumulator should reach 1.0 after exactly 1 call with 10 frames
	drift := mockDrift{ppm: 100000.0} 
	
	// The slip controller should drop 1 frame (4 bytes)
	out := sc.ProcessFrame(buf, frameCount, drift)
	
	expectedSize := (frameCount - 1) * frameSize
	if len(out) != expectedSize {
		t.Errorf("expected %d bytes (dropped frame), got %d", expectedSize, len(out))
	}
	
	inserts, drops := sc.Stats()
	if inserts != 0 || drops != 1 {
		t.Errorf("expected 0 inserts and 1 drop, got %d inserts, %d drops", inserts, drops)
	}
}

func TestSlipController_DuplicateSample(t *testing.T) {
	sc := NewSlipController(DriftCorrectionSlip, 2, 2)
	
	frameCount := 10
	frameSize := 4
	buf := make([]byte, frameCount*frameSize)
	
	// Slow capture clock (-100,000 ppm drift = -1 sample per 10 frames)
	drift := mockDrift{ppm: -100000.0} 
	
	// The slip controller should duplicate 1 frame (4 bytes)
	out := sc.ProcessFrame(buf, frameCount, drift)
	
	expectedSize := (frameCount + 1) * frameSize
	if len(out) != expectedSize {
		t.Errorf("expected %d bytes (duplicated frame), got %d", expectedSize, len(out))
	}
	
	inserts, drops := sc.Stats()
	if inserts != 1 || drops != 0 {
		t.Errorf("expected 1 insert and 0 drops, got %d inserts, %d drops", inserts, drops)
	}
}

func TestFindZeroCrossing(t *testing.T) {
	// Create a synthetic sine wave at 44100Hz
	frameCount := 100
	frameSize := 4
	buf := make([]byte, frameCount*frameSize)
	
	for i := 0; i < frameCount; i++ {
		// A full cycle every 10 frames (freq = 4410Hz)
		val := math.Sin(2.0 * math.Pi * float64(i) / 10.0)
		sample := int16(val * 10000)
		
		// S16LE, 2 channels
		buf[i*4+0] = byte(sample)
		buf[i*4+1] = byte(sample >> 8)
		buf[i*4+2] = byte(sample)
		buf[i*4+3] = byte(sample >> 8)
	}
	
	idx := findZeroCrossing(buf, frameSize, frameCount)
	// Zero crossings should happen at i=0, 5, 10, 15, ...
	// Since we search middle 80% (10..90), it should find a multiple of 5 in the middle.
	
	// Validate the sample at idx is close to 0
	offset := idx * 4
	sample := int16(uint16(buf[offset]) | uint16(buf[offset+1])<<8)
	if math.Abs(float64(sample)) > 1000 {
		t.Errorf("findZeroCrossing returned index %d with amplitude %d, not a zero crossing", idx, sample)
	}
}

func TestDriftEstimator(t *testing.T) {
	de := NewDriftEstimator(44100)
	
	// Simulate perfectly matched clocks
	for i := uint64(0); i < 10; i++ {
		de.RecordSamples(i * 4410) // 100ms per iteration
		// In a real test, time.Now() moves. We can't easily fake time.Now()
		// without modifying DriftEstimator. This test just ensures it doesn't crash.
	}
	
	// In the absence of real time mocking, it won't trigger elapsed >= 100ms
	// but it ensures the code is safe and covers basic init.
	_ = de.DriftPPM()
	_ = de.DriftSamplesPerSecond()
	
	stats := de.Stats()
	if stats.MeasurementCount > 10 {
		t.Errorf("Unexpected measurement count %d", stats.MeasurementCount)
	}
}
