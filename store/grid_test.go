package store

import (
	"context"
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

func (fix *gridFixture) pulse(symbol string, sources map[string]map[string]float64) ([]*Region, error) {
	if _, err := fix.Observe(symbol, fix.zeroSources(sources)); err != nil {
		return nil, err
	}

	return fix.Observe(symbol, sources)
}

func (fix *gridFixture) litBrightness(regions []*Region) map[uint8]float64 {
	bright := make(map[uint8]float64)

	for _, r := range regions {
		if r.Brightness > 0 {
			bright[r.Key] = r.Brightness
		}
	}

	return bright
}

func TestGridObserve(t *testing.T) {
	Convey("Given a grid with priors seeded by a flat tick", t, func() {
		fix := newGridFixture()

		Convey("Unpinned labels fail validation", func() {
			_, err := fix.Observe("BTC/USD", map[string]map[string]float64{
				"pumpdump": {
					"coiled_compression": 0.5,
					"unpinned_metric":    0.9,
				},
			})

			So(err, ShouldNotBeNil)
		})

		Convey("A metric pulse yields lit regions", func() {
			regions, err := fix.pulse("BTC/USD", map[string]map[string]float64{
				"pumpdump": {
					"coiled_compression": 1.5,
					"rejection_tick":     0.3,
				},
				"correlation": {
					"structural_alignment": 0.8,
				},
				"depthflow": {
					"bid_ladder_build": 2.0,
				},
			})

			So(err, ShouldBeNil)
			So(len(regions), ShouldBeGreaterThan, 0)

			lit := 0
			for _, r := range regions {
				if r.Brightness > 0 {
					lit++
				}
			}

			So(lit, ShouldBeGreaterThan, 0)
		})

		Convey("Corroborating metrics land on the same region", func() {
			regions, err := fix.pulse("PEP/USD", map[string]map[string]float64{
				"pumpdump": {"coiled_compression": 2.0},
				"cvd":      {"aggressor_backing": 1.0},
			})

			So(err, ShouldBeNil)

			var r00 *Region
			for _, r := range regions {
				if r.Key == 0 {
					r00 = r
					break
				}
			}

			So(r00, ShouldNotBeNil)
			So(r00.Counts, ShouldEqual, 2)
			So(r00.Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("coiled_compression lights enter detector R00 only", func() {
			regions, err := fix.pulse("PEP/USD", map[string]map[string]float64{
				"pumpdump": {"coiled_compression": 1.2},
			})

			So(err, ShouldBeNil)

			bright := fix.litBrightness(regions)

			So(bright[0], ShouldBeGreaterThan, 0)
			So(bright[6], ShouldEqual, 0)
		})

		Convey("board_coiled lights enter confidence R09", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"pumpdump": {"board_coiled": 1.4},
			})

			So(err, ShouldBeNil)
			So(fix.litBrightness(regions)[9], ShouldBeGreaterThan, 0)
		})

		Convey("ask_ladder_release lights exit opposition R05 not hold support R07", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"depthflow": {"ask_ladder_release": 1.9},
			})

			So(err, ShouldBeNil)

			bright := fix.litBrightness(regions)

			So(bright[5], ShouldBeGreaterThan, 0)
			So(bright[7], ShouldEqual, 0)
		})

		Convey("bid_ladder_build lights hold support R07 not exit opposition R05", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"depthflow": {"bid_ladder_build": 2.0},
			})

			So(err, ShouldBeNil)

			bright := fix.litBrightness(regions)

			So(bright[7], ShouldBeGreaterThan, 0)
			So(bright[5], ShouldEqual, 0)
		})

		Convey("bid_ladder_release lights exit support R04 only", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"depthflow": {"bid_ladder_release": 2.63},
			})

			So(err, ShouldBeNil)

			bright := fix.litBrightness(regions)

			So(bright[4], ShouldBeGreaterThan, 0)
			So(bright[2], ShouldEqual, 0)
			So(bright[8], ShouldEqual, 0)
		})

		Convey("SNM brightness fades when the metric is flat on repeat", func() {
			sources := map[string]map[string]float64{
				"pumpdump": {"coiled_compression": 0.8},
			}

			regions, err := fix.pulse("BTC/USD", sources)
			So(err, ShouldBeNil)

			litR00 := false
			for _, r := range regions {
				if r.Key == 0 && r.Brightness > 0 {
					litR00 = true
				}
			}

			So(litR00, ShouldBeTrue)

			flat, err := fix.Observe("BTC/USD", sources)
			So(err, ShouldBeNil)

			for _, r := range flat {
				if r.Key == 0 {
					So(r.Brightness, ShouldEqual, 0)
				}
			}
		})
	})
}

func TestGridMaturity(t *testing.T) {
	Convey("Given a grid observing one source for a pair", t, func() {
		fix := newGridFixture()
		sig := &Signal{Source: "pumpdump", Symbol: "PEP/USD"}

		So(fix.Maturity(sig), ShouldEqual, 0)

		for range 4 {
			_, err := fix.Observe("PEP/USD", map[string]map[string]float64{
				"pumpdump": {"coiled_compression": 0.5},
			})
			So(err, ShouldBeNil)
		}

		So(fix.Maturity(sig), ShouldBeGreaterThan, 0)
	})
}
