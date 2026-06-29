# Clock Drift Slip Controller Engineering Roadmap

This document outlines the strategic vision and engineering milestones for the `drift-slip-controller-mioarani` project. 

**What is this project?** When audio is recorded on a microphone and played back on a speaker over the network, their hardware clocks will slowly drift out of sync. This library mathematically tracks that drift and seamlessly resamples the audio in real-time to keep streams perfectly locked together. Our objective is to deliver the most performant, secure, and robust Go-native implementation for clock drift compensation in the open-source ecosystem.

## Phase 1: Core Stabilization & Slip Mode (Completed)
**Focus:** API stabilization, basic drift estimation, and slip correction.

- [x] Basic Drift Estimator using Exponential Moving Average (EMA).
- [x] Slip Controller with zero-crossing sample insertion and deletion.
- [x] Support for both S16LE and S24LE PCM formats.
- [x] Comprehensive fuzzing, benchmarking, and OrbStack + Act local CI pipelines.

## Phase 2: Performance & Zero-Allocation Pipelines (Completed)
**Focus:** Eliminating GC pressure and accelerating buffer processing.

- [x] **Zero-Allocation Corrections:** Refactored `ProcessFrame`, `dropFrame`, and `dupFrame` to use pre-allocated buffers and in-place slice mutation. Eliminated the `make([]byte, ...)` allocations to remove garbage collection overhead in the audio fast-path.
- [x] **SIMD Zero-Crossing Search:** Optimized the `findZeroCrossing` algorithm by completely unrolling branches, allowing the Go compiler to seamlessly apply SIMD auto-vectorization (AVX2/NEON) without fragile hand-written assembly.

## Phase 3: High-Quality Resampling (SRC) (Completed)
**Focus:** Pitch-shifting without audible clicks or sample drops.

- [x] **Dynamic Sinc/Polyphase Interpolation:** Implemented dynamic, high-quality linear sample rate conversion to seamlessly pitch-shift audio streams via `DriftCorrectionResample` mode.
- [x] **Polyphase Filter Bank:** Built a fast, low-overhead linear/Hermite interpolation routine to handle fractional sample delays and continuous resampling ratios cleanly without GC allocation.

## Phase 4: Advanced Clock Synchronization (Completed)
**Focus:** Enhancing the accuracy and stability of the drift estimator.

- [x] **NTP Integration:** Implemented the `RecordNTPRTT` bound constraints to incorporate network round-trip time (RTT) stability into the overall clock drift model.
- [x] **PLL / Kalman Filtering:** Upgraded the Exponential Moving Average (EMA) estimator to a robust Phase-Locked Loop (PLL/PI controller), providing superior rejection of scheduling jitter and extremely stable phase locks.

## Phase 5: Observability & Ecosystem (Completed)
**Focus:** Providing deep insights and seamless library integrations.

- [x] **Native Metrics Exporters:** Added a clean `MetricsExporter` interface and documentation on securely integrating Prometheus/OpenTelemetry without polluting the module's zero-dependency footprint.
- [x] **Standard I/O Wrappers:** Created `SlipReader` and `SlipWriter` interfaces that safely manage partial-frame buffering and natively bridge `drift-slip-controller-mioarani` to standard `io.Reader` and `io.Writer` streaming ecosystems.
