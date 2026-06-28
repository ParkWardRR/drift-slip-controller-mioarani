# Drift-Slip Controller Roadmap

## Completed (Prior History)
- [x] Implemented clock synchronization and sample dropping/inserting (slip) algorithms.
- [x] Built Fuzzers and `BenchmarkDriftSlip` for validating sync edge cases.
- [x] Ensured strict code formatting, linting, and CI testing.
- [x] Overhauled documentation to reflect production readiness.

## Short-term Goals
- [ ] Add sophisticated Zero-Crossing detection for inaudible sample slipping.
- [ ] Implement PID control loops for smoother jitter mitigation.
- [ ] Expose extensive metrics (drift metrics, slip counts) for Prometheus integration.

## Mid-term Goals
- [ ] Provide customizable cross-fade algorithms when interpolating missing samples.
- [ ] Support dynamic feedback loops synchronized via PTP (Precision Time Protocol).
- [ ] Refactor for strict bounded-latency allocations on the audio thread.

## Long-term Vision
- [ ] Become the definitive Go module for real-time RTP/AirPlay clock synchronization.
- [ ] Hardware offloading for interpolation stages.
