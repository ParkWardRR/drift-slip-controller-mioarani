package driftslip

import (
	"io"
)

// SlipReader wraps an io.Reader. It reads uncorrected audio bytes, buffers them
// into complete frames, applies drift correction, and yields the corrected audio.
type SlipReader struct {
	r         io.Reader
	sc        *SlipController
	ds        DriftSource
	frameSize int
	remBuf    []byte
	outBuf    []byte
	scratch   []byte
}

// NewSlipReader creates a new SlipReader.
func NewSlipReader(r io.Reader, sc *SlipController, ds DriftSource) *SlipReader {
	return &SlipReader{
		r:         r,
		sc:        sc,
		ds:        ds,
		frameSize: sc.channels * sc.bytesPerSample,
		scratch:   make([]byte, 4096),
	}
}

// Read reads corrected audio bytes into p.
func (sr *SlipReader) Read(p []byte) (n int, err error) {
	// If we have ready output, yield it first.
	if len(sr.outBuf) > 0 {
		n = copy(p, sr.outBuf)
		sr.outBuf = sr.outBuf[n:]
		return n, nil
	}

	// Read new data
	readN, readErr := sr.r.Read(sr.scratch)
	if readN > 0 {
		sr.remBuf = append(sr.remBuf, sr.scratch[:readN]...)

		// Process all complete frames
		frames := len(sr.remBuf) / sr.frameSize
		if frames > 0 {
			processBytes := frames * sr.frameSize
			// ProcessFrame modifies or returns a new slice, so we pass a slice of remBuf
			// For zero-allocation, ProcessFrame requires capacity.
			// We handle this by copying to a sufficiently large buffer if necessary, or just relying on Go's GC if the user doesn't pre-allocate properly.
			// To be safe, we just pass the sub-slice.
			corrected := sr.sc.ProcessFrame(sr.remBuf[:processBytes], frames, sr.ds)
			sr.outBuf = append(sr.outBuf, corrected...)

			// Shift remainder
			sr.remBuf = sr.remBuf[processBytes:]
		}
	}

	if len(sr.outBuf) > 0 {
		n = copy(p, sr.outBuf)
		sr.outBuf = sr.outBuf[n:]
		return n, readErr
	}

	return 0, readErr
}

// SlipWriter wraps an io.Writer. It buffers written audio bytes into complete frames,
// applies drift correction, and writes the corrected audio to the underlying writer.
type SlipWriter struct {
	w         io.Writer
	sc        *SlipController
	ds        DriftSource
	frameSize int
	remBuf    []byte
}

// NewSlipWriter creates a new SlipWriter.
func NewSlipWriter(w io.Writer, sc *SlipController, ds DriftSource) *SlipWriter {
	return &SlipWriter{
		w:         w,
		sc:        sc,
		ds:        ds,
		frameSize: sc.channels * sc.bytesPerSample,
	}
}

// Write writes audio bytes. Drift correction is applied before sending downstream.
func (sw *SlipWriter) Write(p []byte) (n int, err error) {
	sw.remBuf = append(sw.remBuf, p...)

	frames := len(sw.remBuf) / sw.frameSize
	if frames > 0 {
		processBytes := frames * sw.frameSize
		corrected := sw.sc.ProcessFrame(sw.remBuf[:processBytes], frames, sw.ds)
		
		_, writeErr := sw.w.Write(corrected)
		if writeErr != nil {
			return 0, writeErr // we only return error; bytes are consumed from caller's perspective but failed downstream
		}

		// Shift remainder
		// We re-slice to reuse the backing array safely if possible, but append above might reallocate.
		// A high-performance app would manage buffers via sync.Pool.
		sw.remBuf = sw.remBuf[processBytes:]
	}

	return len(p), nil
}
