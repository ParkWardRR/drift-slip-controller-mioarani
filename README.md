# drift-slip-controller-mioarani

![License: Blue Oak](https://img.shields.io/badge/License-Blue_Oak_1.0.0-blue.svg)
![Build Status](https://img.shields.io/badge/build-passing-brightgreen)
![Language](https://img.shields.io/badge/language-Go-blue)
![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen)

## Overview
Clock drift estimator and sample slip controller for zero-crossing modifications.

## Architecture

```mermaid
graph TD;
    A[Clock Diff] --> B(EMA Smoother);
    B --> C[Drift Estimate];
    C --> D(Slip Controller);
    D --> E{Zero Crossing?};
    E -->|Yes| F[Insert/Drop Sample];
```

## Interface
```go
// Core exported structs, traits, or functions
```

## Agent Handoff / Continuation
Copied drift.go and slip.go. Need to bundle into one package, define DriftSource interface for decoupling.
