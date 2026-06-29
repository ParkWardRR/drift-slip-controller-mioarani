# Clock Drift Slip Controller

![Language](https://img.shields.io/badge/Language-Go-blue.svg)
![License](https://img.shields.io/badge/License-BlueOak_1.0.0-green.svg)
 
## Overview
Go library for compensating clock drift between asynchronous audio pipelines. When capturing audio from hardware and sending it over a network, the two clocks will never tick at the exact same speed. One might process 44,100 samples per second, while the other runs at 44,100.02. Over a long session (like a livestream), this tiny difference builds up into massive audio desync or a buffer crash.

### How It Works (Under the Hood)
This library solves the desync problem dynamically:
1. **The Brain (Phase-Locked Loop):** It constantly monitors the stream, tracking the absolute "phase error" (how many samples we are drifting). Instead of reacting to random thread jitter, it uses a PI controller to lock onto the exact hardware clock drift in parts-per-million.
2. **The Fix (Dynamic Resampling):** Once it knows the drift, it stretches or shrinks the audio *just enough* to compensate. It uses dynamic linear interpolation to resample the audio on the fly. Alternatively, it can just drop or duplicate a single audio frame right at a "zero-crossing" (when the sound wave hits absolute silence) so human ears can't hear a pop.
3. **The Shell (Standard I/O):** You wrap your network socket or file stream with our `SlipReader` or `SlipWriter`. All of this complex math happens completely invisibly as you read and write standard Go bytes.

## Key Features
- **Zero-Allocation Audio Path:** Mutates slices fully in-place when adequate capacity is provided, eliminating garbage collection pauses during drift correction.
- **SIMD Auto-Vectorization:** Highly unrolled inner loops enable the Go compiler to seamlessly emit AVX2/NEON instructions, drastically increasing zero-crossing search speed on large buffers.
- **Cross-Platform Purity:** 100% pure Go. No fragile `cgo` bindings or custom assembly required.

## Architecture

```mermaid
graph TD;
    A[Drift Estimator (PPM)] --> B[Accumulator];
    B -->|±1 frame| C[Find Zero Crossing];
    C --> D[Insert / Drop Frame];

```

## Requirements
- **Go**: Latest stable toolchain.
- **OS Support**: Cross-platform (macOS/Linux prioritized).
- **Dependencies**: Minimal to none (strictly constrained to standard library where mathematically possible).

## Quick Tutorial

Integration is straightforward. Consult the module source for exact API signatures.

```go
// 1. Initialize the primary component
// 2. Supply the required I/O interfaces or buffers
// 3. Execute the processing loop or listener
```
## Testing, Fuzzing, and Benchmarking

To run the test suite and benchmarks:
```bash
go test -v ./...
go test -bench .
```

To run the fuzzer:
```bash
go test -fuzz=Fuzz -fuzztime=10s
```
*(Refer to the in-code documentation and `*_test.go` files for exhaustive initialization examples and constraints).*


## Local CI Testing (OrbStack + Act)
This repository is configured for local, completely free CI testing powered by [OrbStack](https://orbstack.dev/) and [act](https://github.com/nektos/act). We deliberately keep the CI workflow definitions out of `.github/` to prevent remote execution and quota consumption.

To run the full test suite locally:
1. Ensure OrbStack is running.
2. Install `act` (e.g., `brew install act`).
3. Run the following command from the root of the repository:
   ```bash
   act -W .local-ci/workflows
   ```
