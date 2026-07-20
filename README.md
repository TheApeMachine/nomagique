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

---

## Domain notes: crypto opportunity detection (symm)

These notes record findings from live-market observation (Kraken Pro, Jun 2025). They connect nomagique's contract to what symm signals must actually measure. Update this section when new patterns are confirmed or refuted.

### North star

**Maximize the wallet. Minimize the time to do so.**

No miracles are expected. What is expected is a highly principled system that detects as many opportunity types as the market actually presents — and that every threshold, window, and scale is derived from observed data, never copied from a tutorial repo (e.g. a hardcoded 60-second window).

If the system fails after a best-effort principled design, that is acceptable. What is not acceptable is failing because we cut corners on semantics or data.

### Why "no magic numbers" is not pedantry

Crypto microstructure varies by asset, session, and regime. A pump on BTC and a pump on a microcap delisting candidate are different physical phenomena. Parameters that work on one must be **functions of what is on the wire**:

| Instead of | Derive from |
|------------|-------------|
| `shortWindow: 5`, `longWindow: 60` | Median inter-arrival gap (`timeline`, `statutil.MedianCadence`) and `WindowDepth(stamps)` |
| `scale: 2.5` | Peer leave-one-out median, symbol's own median baseline, or cross-section rank |
| `returnLag: 1` | Cadence of the ingest role that triggers the signal |
| Fixed % threshold | Spread-relative move, ATR-equivalent from recent log-return median, book touch depth |

The question from AGENTS.md remains the gate for every primitive and every signal field: *given this ticker/book/trade frame, how do we derive shortWindow, longWindow, scale, and returnLag without guessing?*

### Observed market patterns (ground truth for signal design)

Real charts (UNFI, SRM, SLX, TITCOIN, Jun 2025) show that "pump" is not one event. It is a **recurring cycle**:

```
COILED → IGNITION → EXHAUSTION → (COILED again?) → IGNITION → …
```

Example (UNFI/EUR, 1m): first leg ~18:00 (0.02→0.06), consolidation 19:00–21:00, second leg ~21:00 (0.06→0.16), dump ~22:30, third grind ~04:00. **Multiple tradable opportunities on one symbol in one session.** A one-shot "+400% fired" detector misses the consolidation entry before leg 2 and any re-ignition after exhaustion.

Two **distinct species** show up repeatedly and must not be collapsed into one scorer:

| Species | Example | What moves price | Book signature | Cross-section |
|---------|---------|------------------|----------------|---------------|
| **Vertical ignition** | UNFI, SRM, SLX | Real interval volume + price vertical together | Spread moderate; bid depth often stacks at touch; sell walls ahead (SRM 129k @ 0.00896, SLX 8k @ 0.375) | May be idiosyncratic (SRM/SLX up while BTC/ETH flat) or sector-wide (UNFI + TITCOIN + ASRR all hot) |
| **Thin-book percentage pump** | TITCOIN | Tiny dollar flow moves huge % | Spread 11–14%; hollow book; delisting/death-rattle | Same gainer board, but $7k USD volume — percentage lies |

Ticker-only scoring cannot distinguish these. It needs **book** (touch spread, depth, walls), **trade** (executed interval volume), and **cross-section** (peer lift, breadth, dollar-volume rank).

### What principled parameters look like on the wire

For a pump/ignition perspective, derive — do not configure:

- **Lift (RVOL):** interval trade qty (primary) or ticker volume delta (fallback), scaled by symbol median and peer median volume (`statistic.LeaveOneOutMedian`, cross-section volumes).
- **Precursor:** log-return vs **leg anchor** (consolidation range), not only tick-to-tick `last/prevLast`; peer-relative via cross-section returns.
- **Compression:** book touch spread tightening vs symbol's own book-spread median — not ticker bid/ask alone. Coiled Compression is a **book** story (energy building before snap).
- **Exhaustion:** lift decline + trade flow drying up; leg reset when exhaustion category dominates (store `lastExhaustionStamp`, `legAnchorLow/High` on measurement artifacts for tree replay).
- **Window depth:** `WindowDepth(stamps)` from observed timestamps — never a fixed second count.

### nomagique ↔ symm production path

Production scoring is **inline Go on signals** reading the tree (`symm/AGENTS.md`). nomagique primitives are reusable math where they fit; they are not a substitute for domain semantics.

**Signals emit measurements; they do not decide.** Candidate actions come from `market.Story` + `logic`. **Only `trader/crypto.go` decides.** See `symm/AGENTS.md` § Measurements are not decisions (full funnel + end-to-end value tracking).

When a signal comment block names a category (e.g. Coiled Compression = moderate lift + low precursor + spread tightening), the implementation checklist is:

1. Read the comment block as a **state-machine spec**, not a one-shot alarm.
2. List every **data source** the story requires (ticker, book, trade, cross-section, prior measurements).
3. For each numeric gate, write the **derivation** from stream data (which primitive or `statutil` function).
4. Confirm the measurement artifact stores enough replay fields for the next frame to rebuild baselines from the tree alone.
5. Test against a **multi-leg** chart scenario, not a single spike fixture.

Apply this checklist to **every** signal package, not only pumpdump.

### Implementation strategy

Phased plan, per-signal audit table (**12 core signals**), and Kraken backfill notes: **`symm/AGENTS.md` § Principled signal design → Implementation strategy**.

**`manifold` and `resonance` are excluded** from that track — field/latent layers with a separate function; discussed later.

Work order: (1) backfill OHLC + Trades for UNFI/SRM/SLX, (2) wire **L3 level3 ingest** for toxicity and book-honesty signals, (3) tree-only history for core microstructure signals, (4) finish pumpdump as reference, (5) fix remaining core packages against their comment blocks.

L2 book aggregates price levels; **L3 order events** (add/delete + trade correlation) are required for principled toxicity and strongly benefit depthflow, exhaust, fluid, pumpdump, and hawkes. No public L3 historical backfill — forward websocket capture only.

---

Compose with explicit packed artifact frames:

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
err := nomagique.RoundTripArtifact(wire, pipeline)
```

**Migration:** `learning/`, `probability/`, and parts of `geometry/` still expose legacy `Observe`/`Reset` APIs. Target primitives live in `adaptive/`, `statistic/`, `vector/`, and `correlation/Pearson`. See `core/dynamic.go`.

## Composability contract

| Layer | Role | Example |
|-------|------|---------|
| Boundary | packed artifact frame in, packed artifact frame out | `nomagique.RoundTripArtifact(wire, pipeline)` |
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
| `equation`    | Market equations, including capacity-bounded per-symbol ignition baselines                                                                                         |
| `geometry`    | Complex phase dials and controlled corpus phase scans; eigenmodes, PGA, Procrustes; pipeline stages `Velocity`, `Coupling`, `ModePartition`, `Rotor`, `Translator`, `Sandwich` |
| `logic`       | `Circuit`, `Rules`, `Condition`, `True`, `And`, `Or`, `Not`, `Xor`, `GreaterThan`, `LessThan`, `Equal`                                                             |
| `physics/fluid` | Standalone Sensorium periodic PIC, total-energy CPU gas reference, and coupled internal-energy Metal gas/omega-wave domain                                   |
| `physics/manifold` | A shared Metal device/library `Engine` with independently resident per-symbol `Field` solvers and measured working-set capacity                              |
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
