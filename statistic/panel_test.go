package statistic

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestNewPanel(testingTB *testing.T) {
	Convey("Given NewPanel", testingTB, func() {
		panel := NewPanel()

		Convey("It should return an empty registry", func() {
			So(panel, ShouldNotBeNil)
			So(observeInputs(panel), ShouldEqual, 0)
		})
	})
}

func TestPanel_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		key    float64
		value  float64
		expect float64
	}{
		{"register member", 1, 0.02, 0.02},
		{"update member", 2, 0.04, 0.04},
		{"negative sample", 3, -0.01, -0.01},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			panel := NewPanel()
			got := observeWithWork(panel, testCase.key, testCase.value)

			Convey("It should store and echo the sample", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given fewer than two scalar inputs", testingTB, func() {
		panel := NewPanel()
		_ = observeWithWork(panel, 1, 0.02)

		Convey("It should pass through a single scalar input", func() {
			So(observeInputs(panel, 1), ShouldEqual, 1)
		})
	})

	Convey("Given a non-scalar input", testingTB, func() {
		panel := NewPanel()
		_ = observeWithWork(panel, 1, 0.02)

		Convey("It should not overwrite stored values", func() {
			So(observeWithoutSample(panel, 0), ShouldEqual, 0.02)
		})
	})
}

func TestPanel_Reset(testingTB *testing.T) {
	Convey("Given a populated panel", testingTB, func() {
		panel := NewPanel()
		_ = observeWithWork(panel, 1, 0.02)

		So(panel.Reset(), ShouldBeNil)

		Convey("It should clear stored members", func() {
			median := NewMedian(nil, panel)

			So(observeInputs(median, 1), ShouldEqual, 0)
		})
	})
}

func TestMedian_PanelPeers(testingTB *testing.T) {
	Convey("Given a populated panel", testingTB, func() {
		panel := NewPanel()
		median := NewMedian(nil, panel)

		_ = observeWithWork(panel, 1, 0.02)
		_ = observeWithWork(panel, 2, 0.04)
		_ = observeWithWork(panel, 3, 0.06)

		got := observeInputs(median, 1)

		Convey("It should median peer values", func() {
			So(got, ShouldEqual, 0.05)
		})
	})

	Convey("Given a composed macro number", testingTB, func() {
		panel := NewPanel()
		median := NewMedian(nil, panel)

		_ = observeWithWork(panel, 1, 0.02)
		_ = observeWithWork(panel, 2, 0.04)
		_ = observeWithWork(panel, 3, 0.06)

		macro, _ := tests.PipelineSample([]io.ReadWriter{median}, 1)

		Convey("It should match direct observation", func() {
			So(macro, ShouldEqual, 0.05)
		})
	})

	Convey("Given an empty panel", testingTB, func() {
		panel := NewPanel()
		median := NewMedian(nil, panel)

		Convey("It should return zero", func() {
			So(observeInputs(median, 1), ShouldEqual, 0)
		})
	})
}

func TestMedian_PanelPeers_Reset(testingTB *testing.T) {
	Convey("Given an observed panel median stage", testingTB, func() {
		panel := NewPanel()
		median := NewMedian(nil, panel)

		_ = observeWithWork(panel, 1, 0.02)
		_ = observeWithWork(panel, 2, 0.04)
		_ = observeInputs(median, 1)

		So(median.Reset(), ShouldBeNil)

		Convey("It should clear derived output but keep panel data", func() {
			So(observeInputs(median), ShouldEqual, 0)
			So(observeInputs(median, 2), ShouldEqual, 0.02)
		})
	})
}

func BenchmarkMedian_PanelPeers(b *testing.B) {
	panel := NewPanel()
	median := NewMedian(nil, panel)

	_ = observeWithWork(panel, 1, 0.02)
	_ = observeWithWork(panel, 2, 0.04)
	_ = observeWithWork(panel, 3, 0.06)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(median, 1)
	}
}
