# nomagique

**No. magic. numbers.**

Composable numeric dynamics behind `core.Number[T]`. Every stage implements `Observe(...core.Number[T]) core.Scalar[T]` and `Reset() error`. Compose through `nomagique.Number[float64](...)`.

## Composability contract

| Layer | Role | Example |
|-------|------|---------|
| Boundary | `core.Scalar[T]` — raw sample in, derived sample out | `core.Scalar[float64](sample).Observe(stages...)` |
| Pipeline stage | `core.Number[T]` — `Observe(...Number[T]) core.Scalar[T]` + `Reset()` | `adaptive.NewEMA[float64]()` |
| Composed number | `nomagique.Number[T](stages...)` — bootstrap through a stage chain | `nomagique.Number[float64](ema, delta)` |
| Multi-input stages | Stages that need paired scalars | `panel.Observe(key, value)`, `NewTimeElastic` `(level, epochNanos)` |

Everything composes through the pipeline. Do not call domain math imperatively when a stage exists.

## Packages

| Package       | Responsibility |
|---------------|----------------|
| `core`        | `Number[T]`, `Scalar[T]`, `Scalars[T]` |
| `adaptive`    | `EMA`, `Delta`, `Accumulator`, `Compression`, `FracDiff`, `Variance`, `ZScore`, `Momentum`, `Range`, `TimeElastic` |
| `learning`    | `Weight`, `SampleRatio`, `Forecast`, `RLS`, `NewClassifierWeights` |
| `probability` | `Bernoulli`, `CUSUM`, `Rank`, `TransitionSurprise` |
| `statistic`   | `Mean`, `Median`, `Panel`, `LeaveOneOutMedian`, `Quantile`, `StdDev`, `Min`, `Max`, `Entropy`, `FastSlow`, `KLDivergence`, `BivariateMoment`, `OLS`, `RidgeSolver` |
| `vector`      | `FeatureExtractor`, `InputSlot`, `FeatureNode` |
| `correlation` | `Pearson`, `HayashiYoshida`, `Covariance`, `Contagion`, `Multiverse`, `IntervalCoupling`, `IntervalSeries`, `WindowSet` |
| `causal`      | Tabular SCM: `NodeTable`, backdoor, `Graph`, abduction, `DoExpectation`, Pearl ladder, regime hysteresis |
| `hawkes`      | Count-stream MoM plus timestamp MLE |
| `decay`       | Exponential kernel and intensity support |
| `timeline`    | Sorted event timestamps, gaps, and span utilities |
| `algorithm`   | `Pearl`, `Hawkes`, `HawkesFit`, `Shift`, `Correlate`, `Backdoor`, `Calibrate`, `Trust` |
| `geometry`    | Phase dials, eigenmodes, PGA, Procrustes; `Velocity`, `Coupling`, `ModeDetector` |
| `logic`       | `And`, `Or`, `Not`, `Xor`, `Compare`, `Select`, `Gate`, `Mux`, `FirstMatch`, `Latch` |
| `nomagique`   | `Number[T]`, `Numbers[T]` entry points |

## Usage

```go
ema := adaptive.NewEMA[float64]()
delta := adaptive.NewDelta[float64]()
derived := nomagique.Number[float64](ema, delta)
_ = derived

sample := float64(core.Scalar[float64](1).Observe(ema, delta))
```

Cross-section leave-one-out median:

```go
panel := statistic.Panel[float64]{}
leaveOneOut := statistic.NewLeaveOneOutMedian(&panel)
_ = panel.Observe(core.Scalar[float64](memberKey), core.Scalar[float64](reading))
peerMedian := float64(core.Scalar[float64](memberKey).Observe(leaveOneOut))
```

Feature extraction:

```go
extractor, err := vector.NewFeatureExtractor(2,
    func(inputs []float64) float64 { return inputs[0] + inputs[1] },
)
leftSlot, _ := vector.NewInputSlot[float64](extractor, 0)
sumNode, _ := vector.NewFeatureNode[float64](extractor, 0)
_ = core.Scalar[float64](10).Observe(leftSlot)
sum := nomagique.Number[float64](sumNode)
```

Batch (same exact math, higher throughput on amd64):

```go
samples := []float64{1, 2, 3, 4}
out := make([]float64, len(samples))
ema.ObserveSamples(samples, out)
```

## Profiling

```bash
make profile
make profile BENCH=BenchmarkNumber_retainedObserve/ema_delta BENCH_TIME=10s
make profile KIND=mem
make profile-open FILE=.profiles/cpu.prof
```
