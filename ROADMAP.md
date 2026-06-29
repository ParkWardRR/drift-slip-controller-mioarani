# Clock Drift Slip Controller Engineering Roadmap

This document outlines the strategic vision and engineering milestones for the `drift-slip-controller-mioarani` project. Our objective is to deliver the most performant, secure, and robust Go-native implementation for clock drift compensation in asynchronous audio pipelines.

## Phase 1: Core Stabilization & Slip Mode (Completed)
**Focus:** API stabilization, basic drift estimation, and slip correction.

- [x] Basic Drift Estimator using Exponential Moving Average (EMA).
- [x] Slip Controller with zero-crossing sample insertion and deletion.
- [x] Support for both S16LE and S24LE PCM formats.
- [x] Comprehensive fuzzing, benchmarking, and OrbStack + Act local CI pipelines.

## Phase 2: Performance & Zero-Allocation Pipelines
**Focus:** Eliminating GC pressure and accelerating buffer processing.

- [ ] **Zero-Allocation Corrections:** Refactor `ProcessFrame`, `dropFrame`, and `dupFrame` to use pre-allocated buffers or in-place slice mutation. Eliminate the `make([]byte, ...)` allocations to remove garbage collection overhead in the audio fast-path.
- [ ] **SIMD Zero-Crossing Search:** Optimize the `findZeroCrossing` algorithm using Go Assembly (e.g., via `avo`) to leverage AVX2/NEON instructions, drastically reducing CPU load on large buffer sizes.

## Phase 3: High-Quality Resampling (SRC)
**Focus:** Pitch-shifting without audible clicks or sample drops.

- [ ] **Dynamic Sinc/Polyphase Interpolation:** Implement the currently stubbed `DriftCorrectionResample` mode. Replace crude single-sample dropping/duplication with dynamic, high-quality sample rate conversion to seamlessly pitch-shift audio streams.
- [ ] **Polyphase Filter Bank:** Build a highly optimized polyphase filter to handle fractional sample delays and fractional resampling ratios cleanly.

## Phase 4: Advanced Clock Synchronization
**Focus:** Enhancing the accuracy and stability of the drift estimator.

- [ ] **NTP Integration:** Complete the `RecordNTPRTT` implementation to incorporate network round-trip time (RTT) constraints into the overall clock drift model, bridging local monotonic time with network bounds.
- [ ] **PLL / Kalman Filtering:** Upgrade the crude Exponential Moving Average (EMA) estimator in `DriftEstimator` to a robust Phase-Locked Loop (PLL) or Kalman filter, providing superior rejection of scheduling jitter and more stable drift estimates.

## Phase 5: Observability & Ecosystem
**Focus:** Providing deep insights and seamless library integrations.

- [ ] **Native Metrics Exporters:** Add integrated Prometheus and OpenTelemetry hooks for `DriftStats`, exposing real-time PPM drift, correction rates, and system uptime for distributed tracing.
- [ ] **Standard I/O Wrappers:** Provide thin wrappers that implement `io.Reader` and `io.Writer` on top of the slip controller, allowing drop-in compatibility with standard Go stream processing ecosystems.
