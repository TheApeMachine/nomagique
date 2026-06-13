# nomagique

**No. magic. numbers.**

Composable numeric dynamics behind `core.Number`. Boundary scalars use `nomagique.Scalar` with `+=` for raw samples and `Observe` to apply stages.

## Packages

| Package | Responsibility |
|---------|----------------|
| `core` | `Number`, `Pipeline`, `BoundaryRegistry`, `StageParser`, `Stage.Apply` fast path |
| `adaptive` | Exact `float64` EMA, Delta, Accumulator, Compression, FracDiff, Variance, ZScore, Momentum, Range; arm64 EMA batch in `batch_hot_ema_arm64.s` (bit-identical to Go); pipeline bindings |
| `learning` | `Weight`, `SampleRatio`, `Forecast` for predicted-vs-actual calibration |
| `probability` | `Bernoulli`, `CUSUM`, `Rank` for streaming probability signals |
| `statistic` | `Mean`, `Median`, `Quantile`, `StdDev`, `Min`, `Entropy`, `FastSlow`, `KLDivergence`, `BivariateMoment`, `OLS`, `RidgeSolver`, `LinSpace`/`LogSpace` |
| `correlation` | `Pearson`, `HayashiYoshida`, `Covariance`, `Multiverse`, `IntervalCoupling` |
| `causal` | Tabular SCM: `NodeTable`, backdoor, stump ensemble, regime hysteresis |
| `hawkes` | Count-stream MoM plus timestamp MLE: `ArrivalStream`, `BivariateFit`, `BivariateEstimator` |
| `algorithm` | `Pearl`, `Hawkes`, `HawkesFit`, `Shift`, `Correlate`, `Backdoor`, `Calibrate`, `Trust` |
| `geometry` | Phase dials, eigenmodes, PGA, Procrustes, scans; `Velocity`, `Coupling`, `ModeDetector` pipeline bindings |
| `nomagique` | `Scalar` boundary API and nested composition via `resolveStages` |

Domain math lives in `causal`, `hawkes`, `learning`, `probability`, `geometry`, `statistic`, and `correlation`. Hot-path signal kernels and SIMD batch code live in `adaptive`. Orchestration lives in `core`. `adaptive`, `learning`, `probability`, and `geometry` bind types into pipelines.

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

Pearl ladder (tabular SCM over per-node streams):

```go
config := causal.LadderConfig{
    TreatmentNormal: 2, ControlsNormal: []int{0, 1},
    TreatmentInverted: 1, ControlsInverted: []int{0},
    ConditionLeft: 1, ConditionRight: 2, MinHistory: 12,
}
streams := []core.Numbers{node0, node1, node2, node3}
ladder := algorithm.NewPearl(3, config, streams, contagion, nil)
intervention := ladder.Observe()
```

Hawkes timestamp MLE:

```go
fitProcess := algorithm.NewHawkesFit(xTimes, yTimes, horizonNano, hawkes.BivariateFit{})
excitation := fitProcess.Observe()
```

Distribution shift (KL drift between reference and live streams):

```go
shift := algorithm.NewShift(reference, live, nil, 0, 0)
drift := shift.Observe()
```

Dual correlation (async minus sync coupling gap):

```go
correlate := algorithm.NewCorrelate(syncLeft, syncRight, asyncLeft, asyncRight, nil, maxInterval)
gap := correlate.Observe()
```

Linear backdoor adjustment:

```go
backdoor := algorithm.NewBackdoor(targetNode, treatmentNode, []int{0, 1}, streams, 12)
effect := backdoor.Observe()
```

RLS calibration over feature streams:

```go
calibrate, err := algorithm.NewCalibrate([]core.Numbers{feature}, target, 1000, 1)
residual := calibrate.Observe()
```

Calibration trust (forecast scale × adaptive weight):

```go
trust := algorithm.NewTrust()
score := trust.Observe(core.Float64(predicted), core.Float64(actual))
```

Multiverse coupling (feed WindowSets, then observe):

```go
multiverse := correlation.NewMultiverse([]*correlation.WindowSet{left, right}, tiers, config)
coupling := multiverse.Observe()
```

Eigenmode detection:

```go
detector := geometry.NewModeDetector(threshold, origins, energies, couplingMatrix)
dominantEnergy := detector.Observe()
```

Geometry (phase velocity and coupling on means or growth pairs):

```go
phaseVelocity := geometry.Velocity()
phaseCoupling := geometry.Coupling()

number, err := nomagique.Number(adaptive.EMA(), phaseVelocity)
_, _ = phaseCoupling.Observe(leftGrowth, rightGrowth)
```
