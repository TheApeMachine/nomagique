# nomagique

**No. magic. numbers.**

Composable numeric dynamics behind `core.Number`. Boundary scalars use `nomagique.Scalar` with `+=` for raw samples and `Observe` to apply stages.

## Packages

| Package | Responsibility |
|---------|----------------|
| `core` | `Number`, `Pipeline`, `BoundaryRegistry`, `StageParser`, `Stage.Apply` fast path |
| `kernel` | Exact `float64` EMA, Delta, Accumulator, Compression, FracDiff, Variance, ZScore, Momentum, Range; arm64 EMA batch in `batch_hot_ema_arm64.s` (bit-identical to Go); delta batch uses 8× unrolled Go on all arches |
| `adaptive` | Constructors for signal dynamics, wired into `core.Number` |
| `learning` | `Weight`, `SampleRatio`, `Forecast` for predicted-vs-actual calibration |
| `probability` | `Bernoulli`, `CUSUM`, `Rank` for streaming probability signals |
| `geometric` | `Velocity`, `Coupling` for phase-space geometry on scalar pipelines |
| `geometry` | Phase dials, eigenmodes, PGA, scans (batch / token-oriented math) |
| `nomagique` | `Scalar` boundary API and nested composition via `resolveStages` |

Algorithms live in `kernel` (including `kernel/learn`, `kernel/prob`, and `kernel/geom`). Orchestration lives in `core`. `adaptive`, `learning`, `probability`, and `geometric` bind types into pipelines.

## Profiling

```bash
make profile                              # CPU profile for stress-series benchmark
make profile BENCH=BenchmarkNumber_retainedObserve/ema_delta BENCH_TIME=10s
make profile KIND=mem                     # heap profile
make profile-open FILE=.profiles/cpu.prof # reopen last CPU profile
```

## Usage

```go
exponential := adaptive.EMA()
number, err := nomagique.Number(exponential)
if err != nil {
    return err
}

number += 1.0
number, err = number.Observe(exponential)
if err != nil {
    return err
}
```

Batch (same exact math, higher throughput on amd64):

```go
samples := []float64{1, 2, 3, 4}
out := make([]float64, len(samples))
exponential.ObserveSamples(samples, out)
```

Nest pipelines:

```go
chain, err := nomagique.Number(adaptive.EMA(), adaptive.Delta())
number, err := nomagique.Number(chain)
number += 10.0
derived, err := number.Observe(chain)
number = nomagique.Scalar(derived)
```

Other dynamics:

```go
integrator := adaptive.Accumulator()
compressor := adaptive.Compression()
fractional := adaptive.FracDiff()
surprise := adaptive.ZScore()

number, err := nomagique.Number(adaptive.EMA(), surprise)

// Nested turbulence-style chain
chain, err := nomagique.Number(adaptive.FracDiff(), adaptive.Momentum(), adaptive.Compression())
number, err = nomagique.Number(chain)
```

Learning (predicted vs actual pairs):

```go
forecaster := learning.Forecast()
_, err = forecaster.Observe(core.Float64(10), core.Float64(15))
scale := forecaster.Scale() // feedback into signal internals
```

Probability:

```go
hitRate := probability.Bernoulli()
evidence := probability.CUSUM()
rank := probability.Rank()

_, _ = hitRate.Observe(core.Float64(10), core.Float64(15))
_, _ = evidence.Observe(core.Float64(residual))
_, _ = rank.Observe(core.Float64(sample))
```

Geometry (phase velocity and coupling on means or growth pairs):

```go
phaseVelocity := geometric.Velocity()
phaseCoupling := geometric.Coupling()

number, err := nomagique.Number(adaptive.EMA(), phaseVelocity)
_, _ = phaseCoupling.Observe(leftGrowth, rightGrowth)
```
