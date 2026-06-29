package driftslip

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// DriftEstimator — measures capture clock vs playout clock divergence
// ──────────────────────────────────────────────────────────────────────────────
//
// Audio capture clocks (PipeWire, CoreAudio) and protocol receiver playout
// clocks inevitably diverge. Without correction, audio drifts audibly
// after minutes (~1 ppm drift = ~1 sample/22 seconds at 44100 Hz).
//
// The estimator compares captured sample counts against the monotonic
// session clock to compute parts-per-million drift. This feeds into the
// AdaptiveSlipController which compensates.

// DriftEstimator tracks the rate difference between capture and playout clocks.
type DriftEstimator struct {
	mu sync.RWMutex

	// Reference points
	firstCapture time.Time // monotonic time of first sample
	lastCapture  time.Time // monotonic time of latest measurement
	firstSample  uint64    // cumulative sample count at first measurement
	lastSample   uint64    // cumulative sample count at latest measurement

	// Estimated drift in parts-per-million.
	driftPPM float64

	// PI Controller state (Phase-Locked Loop)
	integralError float64
	kp, ki        float64

	// Stats
	measurementCount uint64
	peakDriftPPM     float64
	sampleRate       int

	// Alert state
	lastAlertTime time.Time

	// NTP bounds
	latestNTPDriftPPM float64
	ntpMeasurements   uint64
}

// NewDriftEstimator creates a drift estimator for the given sample rate.
func NewDriftEstimator(sampleRate int) *DriftEstimator {
	return &DriftEstimator{
		kp:         0.1,    // Proportional gain
		ki:         0.001,  // Integral gain
		sampleRate: sampleRate,
	}
}

// RecordSamples records a measurement: the capture pipeline has produced
// `totalSamples` cumulative samples at the current monotonic time.
//
// Call this periodically (e.g., every ~1 second) from the capture thread.
func (d *DriftEstimator) RecordSamples(totalSamples uint64) {
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.firstCapture.IsZero() {
		// First measurement — anchor.
		d.firstCapture = now
		d.firstSample = totalSamples
		d.lastCapture = now
		d.lastSample = totalSamples
		d.measurementCount = 1
		return
	}

	// Need at least 100ms between measurements for meaningful drift.
	elapsed := now.Sub(d.lastCapture)
	if elapsed < 100*time.Millisecond {
		return
	}

	d.measurementCount++

	// Compute instantaneous drift since the last measurement.
	// Expected samples = elapsed * sampleRate
	expectedSamples := elapsed.Seconds() * float64(d.sampleRate)
	actualSamples := float64(totalSamples - d.lastSample)

	if expectedSamples < 1.0 {
		d.lastCapture = now
		d.lastSample = totalSamples
		return
	}

	// Drift ratio: actual/expected. 1.0 = perfect. 1.000001 = +1 ppm.
	ratio := actualSamples / expectedSamples
	instantPPM := (ratio - 1.0) * 1e6

	// PI Controller (PLL) update based on instantaneous phase error
	d.integralError += instantPPM * d.ki
	d.driftPPM = (instantPPM * d.kp) + d.integralError

	// Apply NTP bounds if we have NTP sync
	if d.ntpMeasurements > 5 {
		// Constrain drift to be within 50 PPM of the NTP estimated drift
		const maxNTPDeviation = 50.0
		if d.driftPPM > d.latestNTPDriftPPM+maxNTPDeviation {
			d.driftPPM = d.latestNTPDriftPPM + maxNTPDeviation
			d.integralError = d.driftPPM - (instantPPM * d.kp) // anti-windup
		} else if d.driftPPM < d.latestNTPDriftPPM-maxNTPDeviation {
			d.driftPPM = d.latestNTPDriftPPM - maxNTPDeviation
			d.integralError = d.driftPPM - (instantPPM * d.kp) // anti-windup
		}
	}

	// Track peak.
	if math.Abs(d.driftPPM) > d.peakDriftPPM {
		d.peakDriftPPM = math.Abs(d.driftPPM)
	}

	// Alert on significant drift (> 1 ppm) at most every 30 seconds.
	if math.Abs(d.driftPPM) > 1.0 && now.Sub(d.lastAlertTime) > 30*time.Second {
		log.Printf("[DRIFT] WARNING: capture clock drift %.2f ppm (peak: %.2f ppm, measurements: %d)\n",
			d.driftPPM, d.peakDriftPPM, d.measurementCount)
		d.lastAlertTime = now
	}

	// Periodic logging every 100 measurements.
	if d.measurementCount%100 == 0 {
		log.Printf("[DRIFT] drift=%.3f ppm peak=%.3f ppm measurements=%d\n",
			d.driftPPM, d.peakDriftPPM, d.measurementCount)
	}

	d.lastCapture = now
	d.lastSample = totalSamples
}

// DriftPPM returns the current estimated drift in parts-per-million.
func (d *DriftEstimator) DriftPPM() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.driftPPM
}

// DriftSamplesPerSecond returns the drift expressed as samples-per-second
// deviation from nominal. Useful for the slip controller.
//
// Example: 1.0 ppm at 44100 Hz = 0.0441 extra samples/second.
func (d *DriftEstimator) DriftSamplesPerSecond() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.driftPPM * float64(d.sampleRate) / 1e6
}

// Stats returns drift statistics.
func (d *DriftEstimator) Stats() DriftStats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return DriftStats{
		DriftPPM:         d.driftPPM,
		PeakDriftPPM:     d.peakDriftPPM,
		MeasurementCount: d.measurementCount,
		Uptime:           time.Since(d.firstCapture),
	}
}

// DriftStats holds human-readable drift statistics.
type DriftStats struct {
	DriftPPM         float64
	PeakDriftPPM     float64
	MeasurementCount uint64
	Uptime           time.Duration
}

func (s DriftStats) String() string {
	return fmt.Sprintf("drift=%.3f ppm (peak=%.3f) measurements=%d uptime=%s",
		s.DriftPPM, s.PeakDriftPPM, s.MeasurementCount, s.Uptime.Truncate(time.Second))
}

// ──────────────────────────────────────────────────────────────────────────────
// NTP RTT Drift — estimates drift from NTP sync responses
// ──────────────────────────────────────────────────────────────────────────────

// RecordNTPRTT records an NTP round-trip measurement. Called when the receiver
// responds to a timing packet.
//
// localSendTime: when we sent the NTP request (monotonic)
// localRecvTime: when we received the NTP response (monotonic)
// remoteTime:    receiver's NTP timestamp from the response
func (d *DriftEstimator) RecordNTPRTT(localSendTime, localRecvTime time.Time, remoteNTPTimestamp uint64) {
	rtt := localRecvTime.Sub(localSendTime)
	oneWay := rtt / 2

	d.mu.Lock()
	defer d.mu.Unlock()

	d.ntpMeasurements++

	// Log RTT for observability.
	if d.measurementCount%50 == 0 {
		log.Printf("[DRIFT-NTP] rtt=%v oneWay=%v\n", rtt, oneWay)
	}

	// Pseudo-NTP offset calculation:
	// A real implementation maps the remoteNTPTimestamp to the local clock.
	// We simulate extracting a bounded PPM drift metric.
	// For now, we assume the remote clock has 0 drift relative to monotonic,
	// so the target is 0.0 PPM.
	// Downstream callers would inject the true remote clock offset here.
	d.latestNTPDriftPPM = 0.0 
}
