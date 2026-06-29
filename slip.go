package driftslip

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

	// Resampling state
	resampleBuf []byte
	phase       float64
}

// NewSlipController creates a controller for the given format.
func NewSlipController(mode DriftCorrectionMode, channels, bytesPerSample int) *SlipController {
	return &SlipController{
		mode:           mode,
		channels:       channels,
		bytesPerSample: bytesPerSample,
	}
}

// DriftSource provides the current estimated clock drift.
type DriftSource interface {
	DriftPPM() float64
}

// ProcessFrame applies drift correction to a buffer of interleaved PCM samples.
// Returns the corrected buffer (may be 1 frame shorter or longer).
//
// drift: source of current estimated drift.
// frameCount: number of frames in the buffer (samples per channel).
//
// The returned buffer may have frameCount±1 frames. The caller must handle
// the variable output size (ALAC can encode variable frame counts).
// For zero-allocation duplication, ensure cap(buf) >= len(buf) + (channels * bytesPerSample).
func (sc *SlipController) ProcessFrame(buf []byte, frameCount int, drift DriftSource) []byte {
	if sc.mode == DriftCorrectionNone {
		return buf
	}

	driftPPM := 0.0
	if drift != nil {
		driftPPM = drift.DriftPPM()
	}

	if sc.mode == DriftCorrectionResample {
		sc.mu.Lock()
		defer sc.mu.Unlock()
		return sc.resample(buf, frameCount, driftPPM)
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

// resample performs dynamic linear interpolation to pitch-shift the audio slightly.
func (sc *SlipController) resample(buf []byte, frameCount int, driftPPM float64) []byte {
	frameSize := sc.channels * sc.bytesPerSample
	ratio := 1.0 - (driftPPM / 1e6) // Positive drift means capture is fast (too many samples), so shrink output.

	outFrames := int(float64(frameCount)/ratio + sc.phase)
	outSize := outFrames * frameSize

	if cap(sc.resampleBuf) < outSize {
		sc.resampleBuf = make([]byte, outSize*2) // over-allocate to prevent frequent resizing
	}
	sc.resampleBuf = sc.resampleBuf[:outSize]

	bps := sc.bytesPerSample
	if bps != 2 {
		// Fallback for S24LE or unsupported formats: Nearest neighbor
		for i := 0; i < outFrames; i++ {
			inPos := float64(i)*ratio + sc.phase
			inIdx := int(inPos)
			if inIdx >= frameCount {
				inIdx = frameCount - 1
			}
			copy(sc.resampleBuf[i*frameSize:(i+1)*frameSize], buf[inIdx*frameSize:(inIdx+1)*frameSize])
		}
	} else {
		// S16LE Linear Interpolation
		for i := 0; i < outFrames; i++ {
			inPos := float64(i)*ratio + sc.phase
			inIdx := int(inPos)
			frac := inPos - float64(inIdx)

			for c := 0; c < sc.channels; c++ {
				idx0 := inIdx*frameSize + c*2
				idx1 := idx0 + frameSize
				if inIdx >= frameCount-1 {
					idx1 = idx0 // duplicate last sample
				}

				s0 := float64(int16(uint16(buf[idx0]) | uint16(buf[idx0+1])<<8))
				s1 := float64(int16(uint16(buf[idx1]) | uint16(buf[idx1+1])<<8))
				
				interp := s0 + frac*(s1-s0)
				val := int16(interp)

				outIdx := i*frameSize + c*2
				sc.resampleBuf[outIdx] = byte(val)
				sc.resampleBuf[outIdx+1] = byte(val >> 8)
			}
		}
	}

	// Update phase for next block
	sc.phase = float64(outFrames)*ratio + sc.phase - float64(frameCount)
	if sc.phase > 1.0 {
		sc.phase -= 1.0
	} else if sc.phase < -1.0 {
		sc.phase += 1.0
	}

	return sc.resampleBuf
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

	// Split loops to allow compiler auto-vectorization (SIMD)
	if bps == 3 {
		for i := start; i < end && (i+1)*frameSize <= len(buf); i++ {
			offset := i * frameSize
			sample := int32(buf[offset]) | int32(buf[offset+1])<<8 | int32(int8(buf[offset+2]))<<16
			if sample < 0 {
				sample = -sample
			}
			if sample < bestAbs {
				bestAbs = sample
				bestIdx = i
			}
		}
	} else {
		for i := start; i < end && (i+1)*frameSize <= len(buf); i++ {
			offset := i * frameSize
			sample := int32(int16(uint16(buf[offset]) | uint16(buf[offset+1])<<8))
			if sample < 0 {
				sample = -sample
			}
			if sample < bestAbs {
				bestAbs = sample
				bestIdx = i
			}
		}
	}

	return bestIdx
}

// dropFrame removes the frame at the given index in-place.
func dropFrame(buf []byte, frameSize, idx int) []byte {
	offset := idx * frameSize
	if offset+frameSize > len(buf) {
		return buf
	}
	// Shift remaining bytes left
	copy(buf[offset:], buf[offset+frameSize:])
	return buf[:len(buf)-frameSize]
}

// dupFrame duplicates the frame at the given index.
// If cap(buf) >= len(buf) + frameSize, it mutates in-place.
// Otherwise, it allocates a new slice.
func dupFrame(buf []byte, frameSize, idx int) []byte {
	offset := idx * frameSize
	if offset+frameSize > len(buf) {
		return buf
	}
	
	newLen := len(buf) + frameSize
	if cap(buf) >= newLen {
		// We have capacity. Shift right and insert.
		buf = buf[:newLen]
		// Shift elements after the duplicated frame to the right
		copy(buf[offset+2*frameSize:], buf[offset+frameSize:len(buf)-frameSize])
		// Duplicate the frame
		copy(buf[offset+frameSize:offset+2*frameSize], buf[offset:offset+frameSize])
		return buf
	}

	// Fallback: Allocate new buffer
	result := make([]byte, newLen)
	copy(result, buf[:offset+frameSize])
	copy(result[offset+frameSize:], buf[offset:])
	return result
}
