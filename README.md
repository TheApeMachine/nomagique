# nomagique

**No. magic. numbers.**

Composable compute primitives behind `nomagique.Number(...)`. Each primitive is **one file, one type** implementing `io.ReadWriteCloser`:

| Method | Role |
|--------|------|
| `New*(artifact *datura.Artifact)` | Config lives on the constructor artifact |
| `Write(p []byte)` | Buffer inbound wire artifact on the stage payload |
| `Read(p []byte)` | Evaluate lazily; stamp `output` on the wire |
| `Close() error` | Dispose |

Wire navigation: read `root`, loop `inputs`, match config `input`. No fallbacks, no silent zeros, no warmup/bootstrap, no static windows — parameters derive from the stream (timestamps, spread, history).

Compose with `datura/transport`:

```go
emaConfig := datura.Acquire("ema-config", datura.APPJSON).
    Poke("sample", "input").
    Poke(2, "period")
deltaConfig := datura.Acquire("delta-config", datura.APPJSON).Poke("value", "input")

wire := datura.Acquire("wire", datura.APPJSON)
wire.Poke("features", "root")
wire.Poke([]string{"sample"}, "inputs")
wire.Merge("features", []float64{10})

pipeline := nomagique.Number(adaptive.NewEMA(emaConfig), adaptive.NewDelta(deltaConfig))
err := transport.NewFlipFlop(wire, pipeline)
```

**Migration:** `learning/`, `probability/`, and parts of `geometry/` still expose legacy `Observe`/`Reset` APIs. Target primitives live in `adaptive/`, `statistic/`, `vector/`, and `correlation/Pearson`. See `core/dynamic.go`.

## Composability contract

| Layer | Role | Example |
|-------|------|---------|
| Boundary | `transport.NewFlipFlop` — artifact in, artifact out | `transport.NewFlipFlop(wire, pipeline)` |
| Pipeline | `nomagique.Number(stages...)` | `nomagique.Number(ema, delta)` |
| Stage | Four-method primitive | `adaptive.NewEMA(configArtifact)` |

## Packages

| Package       | Responsibility                                                                                                                                                     |
|---------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `core`        | `Number[T]`, `Scalar[T]`, `Scalars[T]`                                                                                                                             |
| `adaptive`    | `EMA`, `Delta`, `Accumulator`, `Compression`, `FracDiff`, `Variance`, `ZScore`, `Momentum`, `Range`, `TimeElastic`                                                 |
| `learning`    | `Weight`, `SampleRatio`, `Forecast`, `RLS`, `NewClassifierWeights`                                                                                                 |
| `probability` | `Bernoulli`, `CUSUM`, `Rank`, `TransitionSurprise`, `Classifier`, `SoftmaxScores`                                                                                  |
| `statistic`   | `Mean`, `Median`, `Panel`, `LeaveOneOutMedian`, `Quantile`, `StdDev`, `Min`, `Max`, `Entropy`, `FastSlow`, `KLDivergence`, `BivariateMoment`, `OLS`, `RidgeSolver` |
| `vector`      | `FeatureExtractor`, `InputSlot`, `FeatureNode`                                                                                                                     |
| `correlation` | `Pearson`, `HayashiYoshida`, `Covariance`, `Contagion`, `IntervalCoupling`, `IntervalSeries`, `WindowSet`                                            |
| `causal`      | Tabular SCM stages: `NodeRing`, `Zip`, `Backdoor`, `Graph`, `Abduction`, `Do`, `Ladder`, `Regime`, `Contagion` |
| `hawkes`      | Count-stream MoM plus timestamp MLE                                                                                                                                |
| `decay`       | Exponential kernel and intensity support                                                                                                                           |
| `timeline`    | Sorted event timestamps, gaps, and span utilities                                                                                                                  |
| `algorithm`   | `Pearl`, `Hawkes`, `HawkesFit`, `Shift`, `Correlate`, `Backdoor`, `Calibrate`, `Trust`                                                                             |
| `geometry`    | Phase dials, eigenmodes, PGA, Procrustes; pipeline stages `Velocity`, `Coupling`, `ModePartition`, `Rotor`, `Translator`, `Sandwich`                               |
| `logic`       | `Circuit`, `Rules`, `Condition`, `True`, `And`, `Or`, `Not`, `Xor`, `GreaterThan`, `LessThan`, `Equal`                                                             |
| `transport`   | `FlipFlop`, `Pipeline`, `Graph`, `Feedback` (in `datura/transport`)                                                                                                |
| `nomagique`   | `Number` entry point                                                                                                                                               |

## Legacy examples (Observe API — pending migration)

The examples below use the pre-migration `Observe`/`Reset` API. Prefer the FlipFlop pattern above.

## Usage

```go
derived := nomagique.Number[float64](
	transport.Through(0.3),
	adaptive.NewEMA[float64](),
	adaptive.NewDelta[float64](),
)
```

Cross-section leave-one-out median:

```go
panel := statistic.NewPanel[float64]()
leaveOneOut := statistic.NewLeaveOneOutMedian(panel)

source := [1.0, 0.02, 2.0, 0.04, 3.0, 0.06, 1.0]

_ = nomagique.Number[float64](panel).Observe(source)

peerMedian := nomagique.Number[float64](
    transport.Through(1.0),
    leaveOneOut,
    transport.From(leaveOneOut),
)
```

Feature extraction:

```go
extractor := vector.NewFeatureExtractor(2,
    func(inputs []float64) float64 { return inputs[0] + inputs[1] },
)

source := [10.0, 3.0]

derived := nomagique.Number[float64](
    vector.NewInputSlot[float64](extractor, 0),
    vector.NewInputSlot[float64](extractor, 1),
    vector.NewFeatureNode[float64](extractor, 0),
).Observe(source)
```

Composition:

```go
panel := statistic.NewPanel[float64]()
extractor := vector.NewFeatureExtractor(2,
    func(inputs []float64) float64 { return inputs[0] + inputs[1] },
)

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](nomagique.Number[float64](
        vector.NewInputSlot[float64](extractor, 0),
        vector.NewInputSlot[float64](extractor, 1),
        vector.NewFeatureNode[float64](extractor, 0),
    )), 
    adaptive.NewDelta[float64](),
).Observe(
    nomagique.Number[float64](
        panel,
    ).Observe(source),
)
```

Logic circuits and branching. The signal arrives from upstream stages fed by
`source`; the circuit constructor wires the reference and the branches:

```go
source := [0.1, 0.2, 0.3, 0.4, 0.5]

threshold := adaptive.NewEMA[float64]()
consequence := adaptive.NewEMA[float64]()
alternative := adaptive.NewEMA[float64]()

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](),
    logic.NewCircuit(logic.Rules[float64]{
        {
            Condition: logic.GreaterThan[float64]{
                Right: threshold,
            },
            Then: consequence,
        },
        {
            Condition: logic.True[float64]{Operand: true},
            Then:      alternative,
        },
    }),
).Observe(source)
```

Signal above a peer-median reference:

```go
panel := statistic.NewPanel[float64]()
leaveOneOut := statistic.NewLeaveOneOutMedian(panel)

panelSource := [1.0, 0.02, 2.0, 0.04, 3.0, 0.06, 1.0]
source := [0.1, 0.2, 0.3, 0.4, 0.5]

abovePeers := adaptive.NewEMA[float64]()
belowPeers := adaptive.NewEMA[float64]()

nomagique.Number[float64](panel).Observe(panelSource)

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](),
    logic.NewCircuit(logic.Rules[float64]{
        {
            Condition: logic.GreaterThan[float64]{
                Right: leaveOneOut,
            },
            Then: abovePeers,
        },
        {
            Condition: logic.True[float64]{Operand: true},
            Then:      belowPeers,
        },
    }),
).Observe(source)
```

Signal above threshold and enable gate armed:

```go
source := [0.1, 0.2, 0.3, 0.4, 0.5]

threshold := adaptive.NewEMA[float64]()
armed := adaptive.NewEMA[float64]()
blocked := core.Scalar[float64](0)

derived := nomagique.Number[float64](
    adaptive.NewZScore[float64](),
    logic.NewCircuit(logic.Rules[float64]{
        {
            Condition: logic.And[float64]{
                logic.GreaterThan[float64]{
                    Right: threshold,
                },
                logic.True[float64]{
                    Stage: core.Scalar[float64](1),
                },
            },
            Then: armed,
        },
        {
            Condition: logic.True[float64]{Operand: true},
            Then:      blocked,
        },
    }),
).Observe(source)
```

Signal outside a fast/slow band:

```go
source := [0.1, 0.2, 0.3, 0.4, 0.5]

fast := adaptive.NewEMA[float64]()
slow := adaptive.NewEMA[float64]()
expansion := adaptive.NewDelta[float64]()
compression := adaptive.NewCompression[float64]()

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](),
    logic.NewCircuit(logic.Rules[float64]{
        {
            Condition: logic.Or[float64]{
                logic.GreaterThan[float64]{
                    Right: fast,
                },
                logic.LessThan[float64]{
                    Right: slow,
                },
            },
            Then: expansion,
        },
        {
            Condition: logic.True[float64]{Operand: true},
            Then:      compression,
        },
    }),
).Observe(source)
```

Signal not above threshold:

```go
source := [0.1, 0.2, 0.3, 0.4, 0.5]

threshold := adaptive.NewEMA[float64]()
rejected := adaptive.NewEMA[float64]()
accepted := adaptive.NewEMA[float64]()

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](),
    logic.NewCircuit(logic.Rules[float64]{
        {
            Condition: logic.Not[float64]{
                Operand: logic.GreaterThan[float64]{
                    Right: threshold,
                },
            },
            Then: rejected,
        },
        {
            Condition: logic.True[float64]{Operand: true},
            Then:      accepted,
        },
    }),
).Observe(source)
```

Exactly one of two references matched:

```go
source := [0.1, 0.2, 0.3, 0.4, 0.5]

upper := adaptive.NewEMA[float64]()
lower := adaptive.NewEMA[float64]()
either := adaptive.NewEMA[float64]()
neither := core.Scalar[float64](0)

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](),
    logic.NewCircuit(logic.Rules[float64]{
        {
            Condition: logic.Xor[float64]{
                logic.GreaterThan[float64]{
                    Right: upper,
                },
                logic.LessThan[float64]{
                    Right: lower,
                },
            },
            Then: either,
        },
        {
            Condition: logic.True[float64]{Operand: true},
            Then:      neither,
        },
    }),
).Observe(source)
```

Signal pinned to reference:

```go
source := [0.1, 0.2, 0.3, 0.4, 0.5]

target := adaptive.NewEMA[float64]()
pinned := core.Scalar[float64](1)
drifting := adaptive.NewDelta[float64]()

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](),
    logic.NewCircuit(logic.Rules[float64]{
        {
            Condition: logic.Equal[float64]{
                Right: target,
            },
            Then: pinned,
        },
        {
            Condition: logic.True[float64]{Operand: true},
            Then:      drifting,
        },
    }),
).Observe(source)
```

Batch (same exact math, higher throughput on amd64):

```go
source := [1.0, 2.0, 3.0, 4.0]

derived := nomagique.Number[float64](
    adaptive.NewEMA[float64](),
).Observe(source)
```

## Profiling

```bash
make profile
make profile BENCH=BenchmarkNumber_retainedObserve/ema_delta BENCH_TIME=10s
make profile KIND=mem
make profile-open FILE=.profiles/cpu.prof
```
