package protocol

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
)

// ──────────────────────────────────────────────────────────────────────────────
// SlipController — compensates for capture/playout clock drift
// ──────────────────────────────────────────────────────────────────────────────
//
// When capture and playout clocks diverge, audio drifts. The slip controller
// compensates by periodically inserting or dropping single samples at
// inaudible points (zero crossings in stereo PCM).
//
// Modes:
//   slip     — drop/duplicate single samples (simplest, lowest CPU)
//   resample — high-quality sample rate conversion (future)
//   none     — disable correction (for measurement/debugging)

// DriftCorrectionMode selects the drift correction strategy.
type DriftCorrectionMode int

const (
	DriftCorrectionSlip     DriftCorrectionMode = iota // drop/dup single samples
	DriftCorrectionResample                            // SRC (future)
	DriftCorrectionNone                                // disabled
)

func (m DriftCorrectionMode) String() string {
	switch m {
	case DriftCorrectionSlip:
		return "slip"
	case DriftCorrectionResample:
		return "resample"
	case DriftCorrectionNone:
		return "none"
	default:
		return "unknown"
	}
}

// ParseDriftCorrectionMode parses a user-supplied mode string.
func ParseDriftCorrectionMode(s string) (DriftCorrectionMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "slip", "":
		return DriftCorrectionSlip, nil
	case "resample", "src":
		return DriftCorrectionResample, nil
	case "none", "off", "disable":
		return DriftCorrectionNone, nil
	default:
		return DriftCorrectionSlip, fmt.Errorf("unknown drift correction mode %q (use: slip, resample, none)", s)
	}
}

// SlipController manages sample-level drift correction.
type SlipController struct {
	mu   sync.Mutex
	mode DriftCorrectionMode

	// Accumulator: fractional sample debt. When this exceeds ±1.0,
	// we insert or drop a sample.
	accumulator float64

	// Channels (typically 2 for stereo).
	channels int

	// Bytes per sample (2 for S16LE, 3 for S24LE).
	bytesPerSample int

	// Stats
	inserts atomic.Uint64
	drops   atomic.Uint64
}

// NewSlipController creates a controller for the given format.
func NewSlipController(mode DriftCorrectionMode, channels, bytesPerSample int) *SlipController {
	return &SlipController{
		mode:           mode,
		channels:       channels,
		bytesPerSample: bytesPerSample,
	}
}

// ProcessFrame applies drift correction to a buffer of interleaved PCM samples.
// Returns the corrected buffer (may be 1 frame shorter or longer).
//
// driftPPM: current estimated drift from the DriftEstimator.
// frameCount: number of frames in the buffer (samples per channel).
//
// The returned buffer may have frameCount±1 frames. The caller must handle
// the variable output size (ALAC can encode variable frame counts).
func (sc *SlipController) ProcessFrame(buf []byte, frameCount int, driftPPM float64) []byte {
	if sc.mode == DriftCorrectionNone {
		return buf
	}

	if sc.mode == DriftCorrectionResample {
		// TODO: implement high-quality SRC.
		// For now, fall through to slip mode.
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	frameSize := sc.channels * sc.bytesPerSample

	// Accumulate drift. driftPPM samples per 1M samples.
	// Per frame batch: drift * frameCount / 1e6.
	sc.accumulator += driftPPM * float64(frameCount) / 1e6

	// If accumulator hasn't reached ±1 sample, no correction needed.
	if math.Abs(sc.accumulator) < 1.0 {
		return buf
	}

	if sc.accumulator >= 1.0 {
		// Capture clock is fast — we have an extra sample.
		// Drop one frame (skip the last frame in the buffer).
		sc.accumulator -= 1.0
		sc.drops.Add(1)

		if len(buf) >= frameSize {
			// Find a zero-crossing point for minimal audible artifact.
			dropIdx := findZeroCrossing(buf, frameSize, frameCount)
			return dropFrame(buf, frameSize, dropIdx)
		}
		return buf
	}

	// sc.accumulator <= -1.0: capture clock is slow — we're missing a sample.
	// Duplicate one frame (repeat the last frame).
	sc.accumulator += 1.0
	sc.inserts.Add(1)

	if len(buf) >= frameSize {
		// Duplicate the frame nearest a zero crossing.
		dupIdx := findZeroCrossing(buf, frameSize, frameCount)
		return dupFrame(buf, frameSize, dupIdx)
	}
	return buf
}

// Stats returns correction statistics.
func (sc *SlipController) Stats() (inserts, drops uint64) {
	return sc.inserts.Load(), sc.drops.Load()
}

// findZeroCrossing finds the frame index closest to a zero crossing
// in the first channel. This minimizes audible artifacts from sample
// insertion/deletion. Supports both S16LE and S24LE sample formats.
func findZeroCrossing(buf []byte, frameSize, frameCount int) int {
	if frameCount <= 2 {
		return frameCount - 1
	}

	bestIdx := frameCount / 2 // default: middle of buffer
	bestAbs := int32(math.MaxInt32)

	// Determine bytes per sample from frame geometry.
	// frameSize = channels * bytesPerSample; channels is at least 1.
	// For stereo S16LE: frameSize=4, bps=2. For stereo S24LE: frameSize=6, bps=3.
	bps := 2 // default S16LE
	if frameSize >= 6 {
		bps = frameSize / 2 // assumes stereo; works for mono too (frameSize=3 → bps=1, but ≥6 check prevents)
		if bps == 3 {
			bps = 3 // S24LE
		} else {
			bps = 2 // fallback to S16LE
		}
	}

	// Only scan the middle 80% to avoid boundary effects.
	start := frameCount / 10
	end := frameCount - frameCount/10
	if start < 1 {
		start = 1
	}

	for i := start; i < end && (i+1)*frameSize <= len(buf); i++ {
		offset := i * frameSize

		// Read first channel's sample (little-endian, signed).
		var sample int32
		if bps == 3 {
			// S24LE: 3 bytes, sign-extend from 24-bit.
			sample = int32(buf[offset]) | int32(buf[offset+1])<<8 | int32(int8(buf[offset+2]))<<16
		} else {
			// S16LE: 2 bytes.
			sample = int32(int16(uint16(buf[offset]) | uint16(buf[offset+1])<<8))
		}

		absVal := sample
		if absVal < 0 {
			absVal = -absVal
		}
		if absVal < bestAbs {
			bestAbs = absVal
			bestIdx = i
		}
	}

	return bestIdx
}

// dropFrame removes the frame at the given index.
func dropFrame(buf []byte, frameSize, idx int) []byte {
	offset := idx * frameSize
	if offset+frameSize > len(buf) {
		return buf
	}
	result := make([]byte, len(buf)-frameSize)
	copy(result, buf[:offset])
	copy(result[offset:], buf[offset+frameSize:])
	return result
}

// dupFrame duplicates the frame at the given index.
func dupFrame(buf []byte, frameSize, idx int) []byte {
	offset := idx * frameSize
	if offset+frameSize > len(buf) {
		return buf
	}
	result := make([]byte, len(buf)+frameSize)
	copy(result, buf[:offset+frameSize])
	copy(result[offset+frameSize:], buf[offset:])
	return result
}
