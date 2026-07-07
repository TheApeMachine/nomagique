package geometry

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCorpus(t *testing.T) {
	Convey("Given a Corpus with capacity 5", t, func() {
		corpus := NewCorpus(5)
		encoder := NewFloatEncoder()
		baseTime := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

		entries := []struct {
			features map[string]float64
			outcome  CorpusOutcome
		}{
			{
				features: map[string]float64{"spectralRadius": 0.90, "asymmetry": 0.10},
				outcome:  CorpusOutcome{ReturnBps: 15, Category: "organic"},
			},
			{
				features: map[string]float64{"spectralRadius": 0.92, "asymmetry": 0.12},
				outcome:  CorpusOutcome{ReturnBps: 20, Category: "organic"},
			},
			{
				features: map[string]float64{"spectralRadius": 0.99, "asymmetry": 0.85},
				outcome:  CorpusOutcome{ReturnBps: -200, Category: "frenzy"},
			},
			{
				features: map[string]float64{"spectralRadius": 0.50, "asymmetry": 0.05},
				outcome:  CorpusOutcome{ReturnBps: 2, Category: "laminar"},
			},
		}

		for index, entry := range entries {
			dial := encoder.Encode(entry.features)
			encoder.Update(entry.features)

			corpus.Insert(CorpusEntry{
				Dial:    dial,
				Outcome: entry.outcome,
				At:      baseTime.Add(time.Duration(index) * time.Second),
			})
		}

		Convey("It should contain all inserted entries", func() {
			So(corpus.Size(), ShouldEqual, 4)
		})

		Convey("When scanning with a state similar to the organic cluster", func() {
			queryDial := encoder.Encode(map[string]float64{
				"spectralRadius": 0.91,
				"asymmetry":      0.11,
			})
			matches := corpus.Scan(queryDial, 2)

			Convey("It should return the closest matches first", func() {
				So(len(matches), ShouldEqual, 2)
				So(matches[0].Similarity, ShouldBeGreaterThanOrEqualTo, matches[1].Similarity)
			})
		})

		Convey("When scanning with a state similar to frenzy", func() {
			queryDial := encoder.Encode(map[string]float64{
				"spectralRadius": 0.98,
				"asymmetry":      0.80,
			})
			matches := corpus.Scan(queryDial, 1)

			Convey("It should find the frenzy entry as closest", func() {
				So(len(matches), ShouldEqual, 1)
				So(matches[0].Entry.Outcome.Category, ShouldEqual, "frenzy")
			})
		})

		Convey("When using ScanExcluding to omit a specific timestamp", func() {
			queryDial := encoder.Encode(map[string]float64{
				"spectralRadius": 0.90,
				"asymmetry":      0.10,
			})
			excludeTime := baseTime

			matches := corpus.ScanExcluding(queryDial, 10, excludeTime)

			Convey("It should not include the excluded entry", func() {
				for _, match := range matches {
					So(match.Entry.At.UnixNano(), ShouldNotEqual, excludeTime.UnixNano())
				}
			})
		})

		Convey("When the corpus exceeds capacity", func() {
			for index := range 3 {
				corpus.Insert(CorpusEntry{
					Dial: encoder.Encode(map[string]float64{
						"spectralRadius": float64(index) * 0.1,
					}),
					Outcome: CorpusOutcome{ReturnBps: float64(index)},
					At:      baseTime.Add(time.Duration(index+10) * time.Second),
				})
			}

			Convey("It should evict oldest entries and stay at capacity", func() {
				So(corpus.Size(), ShouldEqual, 5)
			})
		})
	})
}

func TestWeightedOutcome(t *testing.T) {
	Convey("Given a set of corpus matches", t, func() {
		matches := []CorpusMatch{
			{
				Entry:      CorpusEntry{Outcome: CorpusOutcome{ReturnBps: 100}},
				Similarity: 0.95,
			},
			{
				Entry:      CorpusEntry{Outcome: CorpusOutcome{ReturnBps: -50}},
				Similarity: 0.60,
			},
			{
				Entry:      CorpusEntry{Outcome: CorpusOutcome{ReturnBps: 20}},
				Similarity: 0.80,
			},
		}

		Convey("It should compute a similarity-weighted average", func() {
			result := WeightedOutcome(matches)
			expected := (0.95*100 + 0.60*(-50) + 0.80*20) / (0.95 + 0.60 + 0.80)
			So(result, ShouldAlmostEqual, expected, 0.01)
		})
	})

	Convey("Given empty matches", t, func() {
		Convey("It should return zero", func() {
			So(WeightedOutcome(nil), ShouldEqual, 0)
		})
	})
}

func BenchmarkCorpusScan(benchmarkTB *testing.B) {
	corpus := NewCorpus(1000)
	encoder := NewFloatEncoder()
	baseTime := time.Now()

	for index := range 1000 {
		features := map[string]float64{
			"spectralRadius": float64(index%100) * 0.01,
			"asymmetry":      float64(index%50) * 0.02,
			"reynolds":       float64(index * 10),
		}

		encoder.Update(features)
		corpus.Insert(CorpusEntry{
			Dial:    encoder.Encode(features),
			Outcome: CorpusOutcome{ReturnBps: float64(index % 200)},
			At:      baseTime.Add(time.Duration(index) * time.Second),
		})
	}

	queryDial := encoder.Encode(map[string]float64{
		"spectralRadius": 0.50,
		"asymmetry":      0.25,
		"reynolds":       500,
	})

	benchmarkTB.ResetTimer()
	benchmarkTB.ReportAllocs()

	for benchmarkTB.Loop() {
		corpus.Scan(queryDial, 10)
	}
}
