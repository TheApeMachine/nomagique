package geometry

import (
	"math"
	"math/cmplx"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewCorpus(t *testing.T) {
	Convey("Given an explicit corpus capacity", t, func() {
		corpus, err := NewCorpus[string](5)

		Convey("It should allocate exactly that capacity", func() {
			So(err, ShouldBeNil)
			So(corpus.maxSize, ShouldEqual, 5)
			So(corpus.Size(), ShouldEqual, 0)
		})

		Convey("It should reject a missing capacity instead of inventing one", func() {
			invalid, invalidErr := NewCorpus[string](0)
			So(invalid, ShouldBeNil)
			So(invalidErr, ShouldNotBeNil)
		})
	})
}

func TestCorpusInsert(t *testing.T) {
	Convey("Given a finite-capacity phase corpus", t, func() {
		corpus, err := NewCorpus[string](2)
		So(err, ShouldBeNil)
		baseTime := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
		firstDial := PhaseDial{1, 0}

		Convey("It should own normalized copies and evict the oldest entry", func() {
			So(corpus.Insert(CorpusEntry[string]{
				Dial:    firstDial,
				Outcome: "first",
				At:      baseTime,
			}), ShouldBeNil)
			firstDial[0] = 0

			So(corpus.Insert(CorpusEntry[string]{
				Dial:    PhaseDial{0, 1},
				Outcome: "second",
				At:      baseTime.Add(time.Second),
			}), ShouldBeNil)
			So(corpus.Insert(CorpusEntry[string]{
				Dial:    PhaseDial{-1, 0},
				Outcome: "third",
				At:      baseTime.Add(2 * time.Second),
			}), ShouldBeNil)

			So(corpus.Size(), ShouldEqual, 2)
			So(corpus.entries[0].Outcome, ShouldEqual, "third")
			So(corpus.entries[1].Outcome, ShouldEqual, "second")
			So(corpus.entries[1].Dial[1], ShouldEqual, complex(1, 0))
		})

		Convey("It should reject invalid and incompatible fingerprints", func() {
			So(corpus.Insert(CorpusEntry[string]{Dial: nil}), ShouldNotBeNil)
			So(corpus.Insert(CorpusEntry[string]{Dial: PhaseDial{complex(math.NaN(), 0)}}), ShouldNotBeNil)
			So(corpus.Insert(CorpusEntry[string]{Dial: PhaseDial{1, 0}}), ShouldBeNil)
			So(corpus.Insert(CorpusEntry[string]{Dial: PhaseDial{1}}), ShouldNotBeNil)
		})
	})
}

func TestCorpusScan(t *testing.T) {
	Convey("Given aligned, quadrature, and antipodal fingerprints", t, func() {
		corpus, err := NewCorpus[string](3)
		So(err, ShouldBeNil)
		baseTime := time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC)
		query := PhaseDial{1, 1}.CopyAndNormalize()
		entries := []CorpusEntry[string]{
			{Dial: query.Rotate(math.Pi), Outcome: "antipode", At: baseTime.Add(2 * time.Second)},
			{Dial: query.Rotate(math.Pi / 2), Outcome: "quadrature", At: baseTime.Add(time.Second)},
			{Dial: query, Outcome: "aligned", At: baseTime},
		}

		for _, entry := range entries {
			So(corpus.Insert(entry), ShouldBeNil)
		}

		matches, scanErr := corpus.Scan(query, len(entries))

		Convey("It should rank the complete signed interference response", func() {
			So(scanErr, ShouldBeNil)
			So(matches[0].Outcome, ShouldEqual, "aligned")
			So(matches[0].Similarity, ShouldAlmostEqual, 1)
			So(matches[1].Outcome, ShouldEqual, "quadrature")
			So(matches[1].Similarity, ShouldAlmostEqual, 0)
			So(matches[2].Outcome, ShouldEqual, "antipode")
			So(matches[2].Similarity, ShouldAlmostEqual, -1)
		})

		Convey("It should reject invalid scan requests explicitly", func() {
			_, countErr := corpus.Scan(query, 0)
			_, dimensionErr := corpus.Scan(PhaseDial{1}, 1)
			So(countErr, ShouldNotBeNil)
			So(dimensionErr, ShouldNotBeNil)
		})

		Convey("It should expose the outcome without leaking resident fingerprints", func() {
			So(matches[0].Outcome, ShouldEqual, "aligned")
		})
	})
}

func TestCorpusScanExcluding(t *testing.T) {
	Convey("Given a self-match and a historical analogue", t, func() {
		corpus, err := NewCorpus[string](2)
		So(err, ShouldBeNil)
		baseTime := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
		query := PhaseDial{1, complex(0, 1)}.CopyAndNormalize()
		So(corpus.Insert(CorpusEntry[string]{
			Dial: query, Outcome: "self", At: baseTime,
		}), ShouldBeNil)
		So(corpus.Insert(CorpusEntry[string]{
			Dial:    query.Rotate(0.1),
			Outcome: "history",
			At:      baseTime.Add(-time.Second),
		}), ShouldBeNil)

		matches, scanErr := corpus.ScanExcluding(query, 2, baseTime)

		Convey("It should remove the specified observation before ranking", func() {
			So(scanErr, ShouldBeNil)
			So(matches, ShouldHaveLength, 1)
			So(matches[0].Outcome, ShouldEqual, "history")
		})
	})
}

func TestCorpusScanPhases(t *testing.T) {
	Convey("Given fingerprints occupying the four phase quadrants", t, func() {
		angles := []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2, 2 * math.Pi}
		query := PhaseDial{1, complex(0, 1), -1, complex(0, -1)}.CopyAndNormalize()
		baseTime := time.Date(2026, 7, 20, 13, 0, 0, 0, time.UTC)
		categories := []string{"zero", "quarter", "half", "three-quarter"}
		corpus, err := NewCorpus[string](len(categories))
		So(err, ShouldBeNil)

		for index, category := range categories {
			So(corpus.Insert(CorpusEntry[string]{
				Dial:    query.Rotate(float64(index) * math.Pi / 2),
				Outcome: category,
				At:      baseTime.Add(time.Duration(index) * time.Second),
			}), ShouldBeNil)
		}

		responses, scanErr := corpus.ScanPhases(query, angles, len(categories))

		Convey("It should traverse the phase topology and return periodically", func() {
			So(scanErr, ShouldBeNil)
			So(responses, ShouldHaveLength, len(angles))
			So(responses[0][0].Outcome, ShouldEqual, "zero")
			So(responses[1][0].Outcome, ShouldEqual, "quarter")
			So(responses[2][0].Outcome, ShouldEqual, "half")
			So(responses[3][0].Outcome, ShouldEqual, "three-quarter")
			So(responses[4][0].Outcome, ShouldEqual, "zero")

			for _, response := range responses {
				So(response[0].Similarity, ShouldAlmostEqual, 1)
				So(response[len(response)-1].Similarity, ShouldAlmostEqual, -1)
			}
		})

		Convey("It should be invariant to corpus insertion order", func() {
			reversed, reversedErr := NewCorpus[string](len(categories))
			So(reversedErr, ShouldBeNil)

			for index := len(categories) - 1; index >= 0; index-- {
				So(reversed.Insert(CorpusEntry[string]{
					Dial:    query.Rotate(float64(index) * math.Pi / 2),
					Outcome: categories[index],
					At:      baseTime.Add(time.Duration(index) * time.Second),
				}), ShouldBeNil)
			}

			reversedResponses, reverseScanErr := reversed.ScanPhases(query, angles, 1)
			So(reverseScanErr, ShouldBeNil)

			for index := range responses {
				So(
					reversedResponses[index][0].Outcome,
					ShouldEqual,
					responses[index][0].Outcome,
				)
			}
		})

		Convey("It should reject an absent or non-finite phase path", func() {
			_, emptyErr := corpus.ScanPhases(query, nil, 1)
			_, nonFiniteErr := corpus.ScanPhases(query, []float64{math.Inf(1)}, 1)
			So(emptyErr, ShouldNotBeNil)
			So(nonFiniteErr, ShouldNotBeNil)
		})
	})
}

func TestCorpusScanPhasesExcluding(t *testing.T) {
	Convey("Given a current dial and one rotated historical analogue", t, func() {
		corpus, err := NewCorpus[string](2)
		So(err, ShouldBeNil)
		currentAt := time.Date(2026, 7, 20, 13, 30, 0, 0, time.UTC)
		query := PhaseDial{1, complex(0, 1)}.CopyAndNormalize()
		So(corpus.Insert(CorpusEntry[string]{
			Dial: query, Outcome: "self", At: currentAt,
		}), ShouldBeNil)
		So(corpus.Insert(CorpusEntry[string]{
			Dial:    query.Rotate(math.Pi / 2),
			Outcome: "history",
			At:      currentAt.Add(-time.Second),
		}), ShouldBeNil)

		responses, scanErr := corpus.ScanPhasesExcluding(
			query,
			[]float64{0, math.Pi / 2},
			1,
			currentAt,
		)

		Convey("It should traverse the historical phase without selecting itself", func() {
			So(scanErr, ShouldBeNil)
			So(responses, ShouldHaveLength, 2)
			So(responses[0][0].Outcome, ShouldEqual, "history")
			So(responses[0][0].Similarity, ShouldAlmostEqual, 0)
			So(responses[1][0].Outcome, ShouldEqual, "history")
			So(responses[1][0].Similarity, ShouldAlmostEqual, 1)
		})
	})
}

func TestCorpusSize(t *testing.T) {
	Convey("Given a newly allocated corpus", t, func() {
		corpus, err := NewCorpus[struct{}](1)
		So(err, ShouldBeNil)

		Convey("It should report inserted population", func() {
			So(corpus.Size(), ShouldEqual, 0)
			So(corpus.Insert(CorpusEntry[struct{}]{Dial: PhaseDial{1}}), ShouldBeNil)
			So(corpus.Size(), ShouldEqual, 1)
		})
	})
}

func BenchmarkCorpusScan(benchmarkTB *testing.B) {
	corpus, err := NewCorpus[struct{}](1000)

	if err != nil {
		benchmarkTB.Fatal(err)
	}

	baseTime := time.Now()

	for entryIndex := range 1000 {
		dial := make(PhaseDial, PhaseDialDimensions)

		for modeIndex := range dial {
			phase := float64(entryIndex+modeIndex) / float64(PhaseDialDimensions)
			dial[modeIndex] = cmplx.Rect(1, phase)
		}

		err = corpus.Insert(CorpusEntry[struct{}]{
			Dial: dial,
			At:   baseTime.Add(time.Duration(entryIndex) * time.Second),
		})

		if err != nil {
			benchmarkTB.Fatal(err)
		}
	}

	query := corpus.entries[0].Dial.CopyAndNormalize()
	benchmarkTB.ResetTimer()
	benchmarkTB.ReportAllocs()

	for benchmarkTB.Loop() {
		if _, err := corpus.Scan(query, 10); err != nil {
			benchmarkTB.Fatal(err)
		}
	}
}

func BenchmarkCorpusScanPhases(benchmarkTB *testing.B) {
	corpus, err := NewCorpus[struct{}](1000)

	if err != nil {
		benchmarkTB.Fatal(err)
	}

	baseTime := time.Now()
	angles := make([]float64, 24)

	for angleIndex := range angles {
		angles[angleIndex] = 2 * math.Pi * float64(angleIndex) / float64(len(angles))
	}

	for entryIndex := range 1000 {
		dial := make(PhaseDial, PhaseDialDimensions)

		for modeIndex := range dial {
			phase := float64(entryIndex+modeIndex) / float64(PhaseDialDimensions)
			dial[modeIndex] = cmplx.Rect(1, phase)
		}

		err = corpus.Insert(CorpusEntry[struct{}]{
			Dial: dial,
			At:   baseTime.Add(time.Duration(entryIndex) * time.Second),
		})

		if err != nil {
			benchmarkTB.Fatal(err)
		}
	}

	query := corpus.entries[0].Dial.CopyAndNormalize()
	benchmarkTB.ResetTimer()
	benchmarkTB.ReportAllocs()

	for benchmarkTB.Loop() {
		if _, err := corpus.ScanPhases(query, angles, 10); err != nil {
			benchmarkTB.Fatal(err)
		}
	}
}
