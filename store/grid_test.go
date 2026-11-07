package store

import (
	"context"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type gridFixture struct {
	*Grid
}

func newGridFixture() *gridFixture {
	return &gridFixture{Grid: NewGrid(context.Background())}
}

func (fix *gridFixture) zeroSources(sources map[string]map[string]float64) map[string]map[string]float64 {
	out := make(map[string]map[string]float64, len(sources))

	for source, labels := range sources {
		out[source] = make(map[string]float64, len(labels))

		for label := range labels {
			out[source][label] = 0
		}
	}

	return out
}

func (fix *gridFixture) observe(
	symbol string,
	sources map[string]map[string]float64,
	open bool,
	volume, day, spread, book, peak bool,
) ([]*Region, error) {
	return fix.Observe(symbol, sources, open, volume, day, spread, book, peak)
}

func (fix *gridFixture) pulse(
	symbol string,
	sources map[string]map[string]float64,
	open bool,
	volume, day, spread, book, peak bool,
) ([]*Region, error) {
	if _, err := fix.observe(
		symbol, fix.zeroSources(sources), open,
		volume, day, spread, book, peak,
	); err != nil {
		return nil, err
	}

	return fix.observe(symbol, sources, open, volume, day, spread, book, peak)
}

func (fix *gridFixture) byKey(regions []*Region, key uint8) *Region {
	for _, region := range regions {
		if region != nil && region.Key == key {
			return region
		}
	}

	return nil
}

func TestGridObserve(t *testing.T) {
	Convey("Given a grid with priors seeded by a flat tick", t, func() {
		Convey("Unpinned labels do not light regions or inflate counts", func() {
			fix := newGridFixture()

			withJunk, err := fix.pulse("BTC/USD", map[string]map[string]float64{
				"pumpdump": {
					"compression":     0.5,
					"unpinned_metric": 0.9,
				},
			}, false, true, true, true, true, true)
			So(err, ShouldBeNil)

			clean := newGridFixture()
			onlyPinned, err := clean.pulse("BTC/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 0.5},
			}, false, true, true, true, true, true)
			So(err, ShouldBeNil)

			So(fix.byKey(withJunk, 0).Counts, ShouldEqual, 1)
			So(fix.byKey(withJunk, 0).Counts, ShouldEqual, clean.byKey(onlyPinned, 0).Counts)
			So(
				fix.byKey(withJunk, 0).Brightness,
				ShouldAlmostEqual,
				clean.byKey(onlyPinned, 0).Brightness,
				1e-12,
			)
		})

		Convey("Flat publishes enter detector/support/opposition", func() {
			fix := newGridFixture()
			regions, err := fix.pulse("BTC/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 1.5},
				"cvd":      {"drive": 1.0},
			}, false, true, true, true, true, true)

			So(err, ShouldBeNil)
			So(len(regions), ShouldEqual, 11)
			So(regions[0].Key, ShouldEqual, uint8(0))
			So(regions[1].Key, ShouldEqual, uint8(1))
			So(regions[2].Key, ShouldEqual, uint8(2))
			So(fix.byKey(regions, 0).Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("Corroborating metrics land on the same region", func() {
			fix := newGridFixture()
			regions, err := fix.pulse("PEP/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 2.0},
				"cvd":      {"drive": 1.0},
			}, false, true, true, true, true, true)

			So(err, ShouldBeNil)
			So(fix.byKey(regions, 0).Counts, ShouldEqual, 2)
			So(fix.byKey(regions, 0).Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("Brightness is tanh of deformation over catalog pin count", func() {
			fix := newGridFixture()
			solo, err := fix.pulse("SOLO/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 1.0},
			}, false, true, true, true, true, true)
			So(err, ShouldBeNil)
			So(fix.byKey(solo, 0).Counts, ShouldEqual, 1)

			pair, err := fix.pulse("PAIR/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 1.0},
				"cvd":      {"drive": 1.0},
			}, false, true, true, true, true, true)
			So(err, ShouldBeNil)
			So(fix.byKey(pair, 0).Counts, ShouldEqual, 2)

			pins := float64(pinCounts[0])
			So(fix.byKey(pair, 0).Brightness, ShouldBeGreaterThan, fix.byKey(solo, 0).Brightness)
			So(
				fix.byKey(solo, 0).Brightness,
				ShouldAlmostEqual,
				math.Tanh(fix.byKey(solo, 0).Deformation/pins),
				1e-12,
			)
		})

		Convey("SNM from a near-zero prior is floored below unit magnitude", func() {
			fix := newGridFixture()
			symbol := "EPS/USD"
			_, err := fix.observe(symbol, map[string]map[string]float64{
				"depthflow": {"thin_score": 0},
			}, false, true, true, true, true, true)
			So(err, ShouldBeNil)

			snm := fix.SNM(
				"depthflow", symbol, "thin_score",
				map[string]float64{"thin_score": 1e-4},
			)

			So(math.Abs(snm), ShouldBeLessThan, 1.0)
			So(math.Abs(snm), ShouldAlmostEqual, 0.5, 1e-9)
		})

		Convey("Open publishes hold when hold support dominates", func() {
			fix := newGridFixture()
			regions, err := fix.pulse("HOLD/USD", map[string]map[string]float64{
				"morphology": {"shape_stable": 2.0},
			}, true, true, true, true, true, true)

			So(err, ShouldBeNil)
			So(len(regions), ShouldEqual, 11)
			So(regions[0].Key, ShouldEqual, holdFirst)
			So(regions[1].Key, ShouldEqual, holdFirst+1)
			So(regions[2].Key, ShouldEqual, holdLast)
			So(fix.byKey(regions, 7).Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("Open publishes exit when exit support dominates", func() {
			fix := newGridFixture()
			regions, err := fix.pulse("EXIT/USD", map[string]map[string]float64{
				"toxicity": {"bluff_score": 2.0},
			}, true, true, true, true, true, true)

			So(err, ShouldBeNil)
			So(len(regions), ShouldEqual, 11)
			So(regions[0].Key, ShouldEqual, exitFirst)
			So(regions[1].Key, ShouldEqual, exitFirst+1)
			So(regions[2].Key, ShouldEqual, exitLast)
			So(fix.byKey(regions, 4).Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("Hold opposition alone makes exit win (support − opposition)", func() {
			fix := newGridFixture()
			regions, err := fix.pulse("OPP/USD", map[string]map[string]float64{
				"sentiment": {"slump_score": 2.0},
			}, true, true, true, true, true, true)

			So(err, ShouldBeNil)
			So(regions[0].Key, ShouldEqual, exitFirst)
			So(fix.byKey(regions, 8), ShouldBeNil)
		})

		Convey("Open hold/exit tie fails closed to hold", func() {
			fix := newGridFixture()
			regions, err := fix.Observe(
				"TIE/USD",
				map[string]map[string]float64{},
				true,
				true, true, true, true, true,
			)

			So(err, ShouldBeNil)
			So(regions[0].Key, ShouldEqual, holdFirst)
		})

		Convey("Desk false sets veto; desk true sets approve", func() {
			vetoFix := newGridFixture()
			vetoes, err := vetoFix.Observe(
				"DESK/USD",
				map[string]map[string]float64{},
				false,
				false, false, false, false, false,
			)
			So(err, ShouldBeNil)
			So(vetoFix.byKey(vetoes, 9).Brightness, ShouldEqual, 1)
			So(vetoFix.byKey(vetoes, 10), ShouldBeNil)
			So(vetoFix.byKey(vetoes, 11).Brightness, ShouldEqual, 1)
			So(vetoFix.byKey(vetoes, 23).Brightness, ShouldEqual, 1)
			So(vetoFix.byKey(vetoes, 24), ShouldBeNil)

			okFix := newGridFixture()
			ok, err := okFix.Observe(
				"DESK/USD",
				map[string]map[string]float64{},
				false,
				true, true, true, true, true,
			)
			So(err, ShouldBeNil)
			So(okFix.byKey(ok, 10).Brightness, ShouldEqual, 1)
			So(okFix.byKey(ok, 9), ShouldBeNil)
			So(okFix.byKey(ok, 24).Brightness, ShouldEqual, 1)
			So(okFix.byKey(ok, 23), ShouldBeNil)
		})

		Convey("Idle metric oracles fail closed to veto", func() {
			fix := newGridFixture()
			regions, err := fix.Observe(
				"IDLE/USD",
				map[string]map[string]float64{},
				false,
				true, true, true, true, true,
			)

			So(err, ShouldBeNil)
			So(fix.byKey(regions, 17).Brightness, ShouldEqual, 0)
			So(fix.byKey(regions, 18), ShouldBeNil)
			So(fix.byKey(regions, 19).Brightness, ShouldEqual, 0)
			So(fix.byKey(regions, 20), ShouldBeNil)
			So(fix.byKey(regions, 21).Brightness, ShouldEqual, 0)
			So(fix.byKey(regions, 22), ShouldBeNil)
		})

		Convey("Metric board approve wins over idle veto", func() {
			fix := newGridFixture()
			regions, err := fix.pulse("BOARD/USD", map[string]map[string]float64{
				"correlation": {"peak_score": 2.0},
			}, false, true, true, true, true, true)

			So(err, ShouldBeNil)
			So(fix.byKey(regions, 18).Brightness, ShouldBeGreaterThan, 0)
			So(fix.byKey(regions, 17), ShouldBeNil)
		})

		Convey("SNM brightness fades when the metric is flat on repeat", func() {
			fix := newGridFixture()
			sources := map[string]map[string]float64{
				"pumpdump": {"compression": 0.8},
			}

			regions, err := fix.pulse("BTC/USD", sources, false, true, true, true, true, true)
			So(err, ShouldBeNil)
			So(fix.byKey(regions, 0).Brightness, ShouldBeGreaterThan, 0)

			flat, err := fix.observe("BTC/USD", sources, false, true, true, true, true, true)
			So(err, ShouldBeNil)
			So(fix.byKey(flat, 0).Brightness, ShouldEqual, 0)
		})
	})
}

func TestGridMaturity(t *testing.T) {
	Convey("Maturity penalizes incomplete metric sets", t, func() {
		fix := newGridFixture()
		symbol := "PEP/USD"

		So(fix.Maturity("pumpdump", symbol), ShouldEqual, 0)

		for range 4 {
			_, err := fix.observe(symbol, map[string]map[string]float64{
				"pumpdump": {"compression": 0.5},
			}, false, true, true, true, true, true)
			So(err, ShouldBeNil)
		}

		samples := 3.0
		completeness := 1.0 / float64(len(regionMetricPins["pumpdump"]))
		expected := (1.0 - 1.0/samples) * completeness

		So(fix.Maturity("pumpdump", symbol), ShouldAlmostEqual, expected, 1e-12)
	})
}
