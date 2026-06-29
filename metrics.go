package driftslip

// MetricsExporter defines an interface for exporting internal statistics
// to external monitoring systems like Prometheus or OpenTelemetry.
// 
// By providing this interface rather than hardcoding external dependencies,
// we ensure this module remains lightweight and dependency-free.
type MetricsExporter interface {
	// RecordDriftPPM is called when the drift estimator computes a new PPM value.
	RecordDriftPPM(ppm float64)

	// RecordSlipStats is called periodically to report cumulative inserts/drops.
	RecordSlipStats(inserts, drops uint64)
}

// Example integration for Prometheus (do not uncomment, copy to your app):
/*
import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PromExporter struct {
	driftGauge prometheus.Gauge
	inserts    prometheus.Counter
	drops      prometheus.Counter
	
	lastInserts uint64
	lastDrops   uint64
}

func NewPromExporter() *PromExporter {
	return &PromExporter{
		driftGauge: promauto.NewGauge(prometheus.GaugeOpts{Name: "audio_drift_ppm"}),
		inserts:    promauto.NewCounter(prometheus.CounterOpts{Name: "audio_slip_inserts_total"}),
		drops:      promauto.NewCounter(prometheus.CounterOpts{Name: "audio_slip_drops_total"}),
	}
}

func (p *PromExporter) RecordDriftPPM(ppm float64) {
	p.driftGauge.Set(ppm)
}

func (p *PromExporter) RecordSlipStats(inserts, drops uint64) {
	if inserts > p.lastInserts {
		p.inserts.Add(float64(inserts - p.lastInserts))
		p.lastInserts = inserts
	}
	if drops > p.lastDrops {
		p.drops.Add(float64(drops - p.lastDrops))
		p.lastDrops = drops
	}
}
*/
