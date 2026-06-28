# Clock Drift Slip Controller

![Language](https://img.shields.io/badge/Language-Go-blue.svg)
![License](https://img.shields.io/badge/License-BlueOak_1.0.0-green.svg)
![Status](https://img.shields.io/badge/Status-Production_Ready-brightgreen.svg)

## Release Status
This repository is fully prepared for public release. CI/CD pipelines, exhaustive fuzzing, and extensive benchmarking have been heavily integrated to guarantee production-ready stability.

## Overview
Go library for compensating clock drift between asynchronous audio pipelines via zero-crossing sample insertion/deletion.

Designed strictly for high-performance integrations and infrastructure codebases. No redundant abstractions; focuses entirely on precise data processing.

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
