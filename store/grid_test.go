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

func (fix *gridFixture) pulse(
	symbol string, sources map[string]map[string]float64,
) ([]*Region, error) {
	if _, err := fix.Observe(symbol, fix.zeroSources(sources)); err != nil {
		return nil, err
	}

	return fix.Observe(symbol, sources)
}

func TestGridObserve(t *testing.T) {
	Convey("Given a grid with priors seeded by a flat tick", t, func() {
		fix := newGridFixture()

		Convey("Unpinned labels fail validation", func() {
			_, err := fix.Observe("BTC/USD", map[string]map[string]float64{
				"pumpdump": {
					"compression":     0.5,
					"unpinned_metric": 0.9,
				},
			})

			So(err, ShouldNotBeNil)
		})

		Convey("Observe returns the full region set", func() {
			regions, err := fix.pulse("BTC/USD", map[string]map[string]float64{
				"pumpdump": {
					"compression": 1.5,
					"exhaustion":  0.3,
				},
				"correlation": {
					"herd_score": 0.8,
				},
				"depthflow": {
					"loaded_score": 2.0,
				},
			})

			So(err, ShouldBeNil)
			So(len(regions), ShouldEqual, RegionCount)

			for key, region := range regions {
				So(region, ShouldNotBeNil)
				So(region.Key, ShouldEqual, uint8(key))
				So(region.Symbol, ShouldEqual, "BTC/USD")
			}

			So(regions[0].Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("Corroborating metrics land on the same region", func() {
			regions, err := fix.pulse("PEP/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 2.0},
				"cvd":      {"drive": 1.0},
			})

			So(err, ShouldBeNil)
			So(regions[0].Counts, ShouldEqual, 2)
			So(regions[0].Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("Brightness divides by target pins, not observed counts", func() {
			solo, err := fix.pulse("SOLO/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 1.0},
			})
			So(err, ShouldBeNil)
			So(solo[0].Counts, ShouldEqual, 1)

			pair, err := fix.pulse("PAIR/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 1.0},
				"cvd":      {"drive": 1.0},
			})
			So(err, ShouldBeNil)
			So(pair[0].Counts, ShouldEqual, 2)

			So(pair[0].Brightness, ShouldBeGreaterThan, solo[0].Brightness)
			So(solo[0].Brightness, ShouldAlmostEqual, solo[0].Deformation/TargetPinsPerRegion, 1e-12)
		})

		Convey("SNM from a near-zero prior is floored below unit magnitude", func() {
			symbol := "EPS/USD"
			_, err := fix.Observe(symbol, map[string]map[string]float64{
				"depthflow": {"thin_score": 0},
			})
			So(err, ShouldBeNil)

			snm := fix.SNM(&Signal{
				Source: "depthflow",
				Symbol: symbol,
				Labels: []string{"thin_score"},
				Values: []float64{1e-4},
			}, "thin_score")

			So(math.Abs(snm), ShouldBeLessThan, 1.0)
			So(math.Abs(snm), ShouldAlmostEqual, 0.5, 1e-9)
		})

		Convey("compression lights enter detector R00", func() {
			regions, err := fix.pulse("PEP/USD", map[string]map[string]float64{
				"pumpdump": {"compression": 1.2},
			})

			So(err, ShouldBeNil)
			So(regions[0].Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("peak_score lights board confidence approve R18", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"correlation": {"peak_score": 1.4},
			})

			So(err, ShouldBeNil)
			So(regions[18].Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("ask_vacuum lights exit opposition R05", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"morphology": {"ask_vacuum": 1.9},
			})

			So(err, ShouldBeNil)
			So(regions[5].Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("shape_stable lights hold support R07", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"morphology": {"shape_stable": 2.0},
			})

			So(err, ShouldBeNil)
			So(regions[7].Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("thin_score lights exit support R04", func() {
			regions, err := fix.pulse("PEAQ/USD", map[string]map[string]float64{
				"depthflow": {"thin_score": 2.63},
			})

			So(err, ShouldBeNil)
			So(regions[4].Brightness, ShouldBeGreaterThan, 0)
		})

		Convey("SNM brightness fades when the metric is flat on repeat", func() {
			sources := map[string]map[string]float64{
				"pumpdump": {"compression": 0.8},
			}

			regions, err := fix.pulse("BTC/USD", sources)
			So(err, ShouldBeNil)
			So(regions[0].Brightness, ShouldBeGreaterThan, 0)

			flat, err := fix.Observe("BTC/USD", sources)
			So(err, ShouldBeNil)
			So(flat[0].Brightness, ShouldEqual, 0)
		})

		Convey("spectral_radius lights hold detector and board confidence approve", func() {
			symbol := "CFG/USD"

			_, err := fix.Observe(symbol, map[string]map[string]float64{
				"hawkes": {
					"spectral_radius":     0.4,
					"branching_asymmetry": 0.2,
				},
			})
			So(err, ShouldBeNil)

			regions, err := fix.Observe(symbol, map[string]map[string]float64{
				"hawkes": {
					"spectral_radius":     1.6,
					"branching_asymmetry": 0.9,
				},
			})
			So(err, ShouldBeNil)
			So(regions[6].Brightness, ShouldBeGreaterThan, 0)
			So(regions[18].Brightness, ShouldBeGreaterThan, 0)
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
				"pumpdump": {"compression": 0.5},
			})
			So(err, ShouldBeNil)
		}

		So(fix.Maturity(sig), ShouldBeGreaterThan, 0)
	})
}

func TestPathToken(t *testing.T) {
	Convey("PathToken is the brightest action regime; oracles ignored", t, func() {
		regions := make([]*Region, RegionCount)
		for key := uint8(0); key < RegionCount; key++ {
			regions[key] = &Region{Symbol: "BTC/USD", Key: key}
		}

		regions[10].Brightness = 1
		regions[14].Brightness = 1
		regions[0].Brightness = 0.4
		regions[2].Brightness = 0.9

		token, brightness := PathToken(regions)
		So(token, ShouldEqual, "R02")
		So(brightness, ShouldEqual, 0.9)
	})
}

func TestGridEmit(t *testing.T) {
	Convey("Given a full region set with action and oracle brightness", t, func() {
		regions := make([]*Region, RegionCount)
		for key := uint8(0); key < RegionCount; key++ {
			regions[key] = &Region{
				Symbol:      "BTC/USD",
				Key:         key,
				Brightness:  0.5,
				Deformation: 1.5,
				Counts:      1,
			}
		}

		regions[17].Brightness = 0.9
		regions[18].Brightness = 0.4

		Convey("Flat keeps Enter, zeros Exit/Hold, oracle winner only", func() {
			fix := newGridFixture()
			fix.Emit(regions, false)

			So(regions[0].Brightness, ShouldEqual, 0.5)
			So(regions[2].Brightness, ShouldEqual, 0.5)
			So(regions[3].Brightness, ShouldEqual, 0)
			So(regions[6].Brightness, ShouldEqual, 0)
			So(regions[8].Brightness, ShouldEqual, 0)

			So(regions[17].Brightness, ShouldEqual, 0.9)
			So(regions[18].Brightness, ShouldEqual, 0)
		})

		Convey("Open keeps Exit/Hold, zeros Enter, oracle winner only", func() {
			regions := make([]*Region, RegionCount)
			for key := uint8(0); key < RegionCount; key++ {
				regions[key] = &Region{
					Symbol:      "BTC/USD",
					Key:         key,
					Brightness:  0.5,
					Deformation: 1.5,
					Counts:      1,
				}
			}

			regions[13].Brightness = 0.2
			regions[14].Brightness = 0.7

			fix := newGridFixture()
			fix.Emit(regions, true)

			So(regions[0].Brightness, ShouldEqual, 0)
			So(regions[2].Brightness, ShouldEqual, 0)
			So(regions[3].Brightness, ShouldEqual, 0.5)
			So(regions[7].Brightness, ShouldEqual, 0.5)

			So(regions[13].Brightness, ShouldEqual, 0)
			So(regions[14].Brightness, ShouldEqual, 0.7)
		})
	})
}
