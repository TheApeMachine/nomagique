package algorithm

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestDecaySample_Read(t *testing.T) {
	Convey("Given deteriorating bid depth on repeated book frames", t, func() {
		encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
		decay := equation.NewDecay(equation.DecayConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("exhaust-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs":    []string{"mechanical", "fragile", "thermal", "reversal"},
				"scoreRoot": "output",
			}),
		)
		pipeline := transport.NewPipeline(encoder, decay, classifier)

		quantities := []float64{20, 18, 16, 14, 12, 10, 8, 6, 4}

		var result *datura.Artifact

		for index, bidQty := range quantities {
			frame := []byte(fmt.Sprintf(
				`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":%g}],"asks":[{"price":101,"qty":10}]}]}`,
				bidQty,
			))
			state := datura.Acquire("measurement", datura.APPJSON).
				WithRole("measurement").
				WithScope("update").
				WithPayload(frame)

			err := nomagique.RoundTripArtifact(state, pipeline)

			if index == len(quantities)-1 {
				So(err, ShouldBeNil)
			}

			if result != nil {
				result.Release()
			}

			result = state
		}

		Convey("It should emit calibrated decay output", func() {
			So(result, ShouldNotBeNil)
			So(datura.Peek[bool](result, "output", "ready"), ShouldBeTrue)
			So(datura.Peek[float64](result, "output", "value"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](result, "output", "confidence"), ShouldBeGreaterThan, 0.25)

			result.Release()
		})
	})

	Convey("Given stable bid depth on repeated book frames", t, func() {
		encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
		decay := equation.NewDecay(equation.DecayConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("exhaust-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs":    []string{"mechanical", "fragile", "thermal", "reversal"},
				"scoreRoot": "output",
			}),
		)
		pipeline := transport.NewPipeline(encoder, decay, classifier)

		frame := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)

		var lastErr error

		for range 12 {
			state := datura.Acquire("measurement", datura.APPJSON).
				WithRole("measurement").
				WithScope("update").
				WithPayload(frame)

			lastErr = nomagique.RoundTripArtifact(state, pipeline)
			state.Release()
		}

		Convey("It should complete the pipeline after history accumulates", func() {
			So(lastErr, ShouldBeNil)
		})
	})

	Convey("Given deteriorating row-level book artifacts", t, func() {
		encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
		decay := equation.NewDecay(equation.DecayConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("exhaust-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs": []string{"mechanical", "fragile", "thermal", "reversal"},
			}),
		)
		pipeline := transport.NewPipeline(encoder, decay, classifier)
		quantities := []float64{20, 18, 16, 14, 12, 10, 8, 6, 4}

		var result *datura.Artifact

		for index, bidQty := range quantities {
			frame := []byte(fmt.Sprintf(
				`{"symbol":"BTC/USD","bids":[{"price":100,"qty":%g}],"asks":[{"price":101,"qty":10}]}`,
				bidQty,
			))
			state := datura.Acquire("measurement", datura.APPJSON).
				WithRole("measurement").
				WithScope("BTC/USD").
				WithPayload(frame)

			err := nomagique.RoundTripArtifact(state, pipeline)

			if index == len(quantities)-1 {
				So(err, ShouldBeNil)
			}

			if result != nil {
				result.Release()
			}

			result = state
		}

		Convey("It should emit classified decay output without a Kraken envelope", func() {
			So(result, ShouldNotBeNil)
			So(datura.Peek[string](result, "symbol"), ShouldEqual, "BTC/USD")
			So(datura.Peek[string](result, "root"), ShouldEqual, "output")
			So(datura.Peek[float64](result, "output", "category"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](result, "output", "confidence"), ShouldBeGreaterThan, 0)

			result.Release()
		})
	})
}

func TestDecaySample_ReadStagesColdStart(t *testing.T) {
	Convey("Given a valid first book frame before feature history is ready", t, func() {
		encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
		frame := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(frame)

		err := nomagique.RoundTripArtifact(state, encoder)

		Convey("It should be a nonfatal not-ready sample", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[bool](state, "output", "ready"), ShouldBeFalse)
			So(len(datura.Peek[[]float64](state, "features")), ShouldEqual, 0)

			state.Release()
		})
	})
}

func TestDecaySample_ReadRejectsMissingSymbol(t *testing.T) {
	Convey("Given a book frame without symbol", t, func() {
		encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
		frame := []byte(`{"channel":"book","type":"update","data":[{"bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(frame)

		err := nomagique.RoundTripArtifact(state, encoder)

		So(err, ShouldNotBeNil)
		state.Release()
	})
}

func BenchmarkDecaySample_Read(b *testing.B) {
	encoder := NewDecaySample(datura.Acquire("decay-sample", datura.APPJSON))
	bookPayload := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)
	frame := make([]byte, 262144)

	b.ReportAllocs()

	for b.Loop() {
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(bookPayload)
		packed := state.Pack()

		if len(packed) == 0 {
			b.Fatal("decay_sample: artifact pack failed")
		}

		_, _ = encoder.Write(packed)
		_, _ = encoder.Read(frame)
	}
}
