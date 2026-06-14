# nomagique

**No. magic. numbers.**

Composable numeric dynamics behind `core.Number`. Boundary scalars use `nomagique.Scalar` with `+=` for raw samples and `Observe` to apply stages.

## Composability contract

| Layer | Role | Example |
|-------|------|---------|
| Boundary | `nomagique.Scalar` — raw sample in, derived sample out | `sample.Observe(stages...)` |
| Pipeline stage | `core.Number` — `Observe(...Number) core.Float64` + `Reset()` | `adaptive.EMA()`, `statistic.NewMedian(nil)` |
| Composed number | `nomagique.Number(stages...)` — registers a reusable pipeline | `macro, _ := nomagique.Number(leaveOneOut)` |
| Backend state | **Not** `core.Number` — shared mutable registry | `vector.FeatureExtractor`, `statistic.Panel` (write side is `Panel.Observe`) |
| Feed adapter | **Not** `core.Number` — imperative market feeds | `correlation.WindowSet.Observe(nanos, price)` |

Compose through `nomagique.Number(...)`. Do not call domain math imperatively when a stage exists.

## Packages

| Package       | Responsibility                                                                                                                                                                         |
|---------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `core`        | `Number`, `Pipeline`, `BoundaryRegistry`, `StageParser`, `Stage.Apply` fast path                                                                                                       |
| `adaptive`    | Exact `float64` EMA, Delta, Accumulator, Compression, FracDiff, Variance, ZScore, Momentum, Range; arm64 EMA batch in `batch_hot_ema_arm64.s` (bit-identical to Go); pipeline bindings |
| `learning`    | `Weight`, `SampleRatio`, `Forecast`, `NewClassifierWeights` for predicted-vs-actual calibration                                                                                        |
| `probability` | `Bernoulli`, `CUSUM`, `Rank` for streaming probability signals                                                                                                                         |
| `statistic`   | `Mean`, `Median`, `Panel`, `LeaveOneOutMedian`, `Quantile`, `StdDev`, `Min`, `Max`, `Entropy`, `FastSlow`, `KLDivergence`, `BivariateMoment`, `OLS`, `RidgeSolver`                      |
| `vector`      | `FeatureExtractor`, `InputSlot`, `FeatureNode`, `NewL1BookExtractor` — L1 book feature pipeline                                                                                        |
| `correlation` | `Pearson`, `HayashiYoshida`, `Covariance`, `Multiverse`, `IntervalCoupling`; `WindowSet` feed adapters                                                                                 |
| `causal`      | Tabular SCM: `NodeTable`, backdoor, `Graph` identification, abduction, `DoExpectation`, Pearl ladder, regime hysteresis                                                                |
| `hawkes`      | Count-stream MoM plus timestamp MLE: `ArrivalStream`, `BivariateFit`, `BivariateEstimator`                                                                                             |
| `decay`       | Exponential kernel and intensity support for Hawkes excitation                                                                                                                         |
| `timeline`    | Sorted event timestamps, gaps, and span utilities                                                                                                                                      |
| `algorithm`   | `Pearl`, `Hawkes`, `HawkesFit`, `Shift`, `Correlate`, `Backdoor`, `Calibrate`, `Trust`                                                                                                 |
| `geometry`    | Phase dials, eigenmodes, PGA, Procrustes, scans; `Velocity`, `Coupling`, `ModeDetector` pipeline bindings                                                                              |
| `physics/manifold` | GPU torus manifold solver (`Config`, `Solver`, Metal)                                                                                                                             |
| `nomagique`   | `Scalar` boundary API and nested composition via `resolveStages`                                                                                                                       |

Domain math lives in `causal`, `hawkes`, `learning`, `probability`, `geometry`, `statistic`, `vector`, and `correlation`. Hot-path signal kernels and SIMD batch code live in `adaptive`. Orchestration lives in `core`.

## Usage

```go
exponential := adaptive.EMA()
number, err := nomagique.Number(exponential)
if err != nil {
    return err
}

number += 1.0
derived := number.Observe(exponential)
number = nomagique.Scalar(derived)
```

Cross-section macro (leave-one-out peer median):

```go
panel := statistic.Panel{}
leaveOneOut := statistic.NewLeaveOneOutMedian(&panel)
macro, err := nomagique.Number(leaveOneOut)

_ = panel.Observe(nomagique.Scalar(symbolKey), nomagique.Scalar(changePct))
peerMedian := nomagique.Scalar(symbolKey).Observe(macro)
```

L1 book features:

```go
extractor, err := vector.NewL1BookExtractor()
spreadNode, err := vector.NewFeatureNode(extractor, vector.L1SpreadBPS)
spread, err := nomagique.Number(spreadNode)

_ = extractor.SetInput(vector.L1BidPrice, bid)
_ = extractor.SetInput(vector.L1AskPrice, ask)
bps := nomagique.Scalar(0).Observe(spread)
```

Nest pipelines:

```go
chain, err := nomagique.Number(adaptive.EMA(), adaptive.Delta())
number, err := nomagique.Number(chain)
number += 10.0
derived := number.Observe(chain)
number = nomagique.Scalar(derived)
```

Batch (same exact math, higher throughput on amd64):

```go
samples := []float64{1, 2, 3, 4}
out := make([]float64, len(samples))
exponential.ObserveSamples(samples, out)
```

## Profiling

```bash
make profile                              # CPU profile for stress-series benchmark
make profile BENCH=BenchmarkNumber_retainedObserve/ema_delta BENCH_TIME=10s
make profile KIND=mem                     # heap profile
make profile-open FILE=.profiles/cpu.prof # reopen last CPU profile
```
