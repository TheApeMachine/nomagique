package algorithm

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestBookflowSample_Read(t *testing.T) {
	Convey("Given no inbound frame", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		buffer := make([]byte, 4096)

		n, err := encoder.Read(buffer)

		Convey("It should report no frame without trying to unpack stale payload", func() {
			So(n, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})
	})

	Convey("Given an empty inbound frame", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		buffer := make([]byte, 4096)

		n, writeErr := encoder.Write(nil)
		readN, readErr := encoder.Read(buffer)

		Convey("It should drain cleanly without validation noise", func() {
			So(n, ShouldEqual, 0)
			So(writeErr, ShouldBeNil)
			So(readN, ShouldEqual, 0)
			So(readErr, ShouldEqual, io.EOF)
		})
	})

	Convey("Given repeated bid-heavy book frames", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		bookflow := equation.NewBookflow(bookflowAlgoConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("depthflow-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs":    []string{"loadedScore", "spoofScore", "thinScore", "neutralScore"},
				"scoreRoot": "output",
			}),
		)
		pipeline := transport.NewPipeline(encoder, bookflow, classifier)

		frames := [][]byte{
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`),
		}

		var result *datura.Artifact

		for index, frame := range frames {
			state := datura.Acquire("measurement", datura.APPJSON).
				WithRole("measurement").
				WithScope("update").
				WithPayload(frame)

			err := nomagique.RoundTripArtifact(state, pipeline)

			if index == len(frames)-1 {
				So(err, ShouldBeNil)
			}

			if result != nil {
				result.Release()
			}

			result = state
		}

		Convey("It should emit calibrated depth-flow output", func() {
			So(result, ShouldNotBeNil)
			So(datura.Peek[bool](result, "output", "ready"), ShouldBeTrue)
			So(datura.Peek[float64](result, "output", "value"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](result, "output", "confidence"), ShouldBeGreaterThan, 0.25)

			result.Release()
		})
	})
}

func TestBookflowSample_ReadRowArtifacts(t *testing.T) {
	Convey("Given repeated bid-heavy row-level book artifacts", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		bookflow := equation.NewBookflow(bookflowAlgoConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("depthflow-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs": []string{
					"loadedScore",
					"spoofScore",
					"thinScore",
					"neutralScore",
				},
			}),
		)
		pipeline := transport.NewPipeline(encoder, bookflow, classifier)
		frame := []byte(`{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}`)
		var result *datura.Artifact

		for index := range 6 {
			state := datura.Acquire("measurement", datura.APPJSON).
				WithRole("measurement").
				WithScope("BTC/USD").
				WithPayload(frame)

			err := nomagique.RoundTripArtifact(state, pipeline)

			if index == 5 {
				So(err, ShouldBeNil)
			}

			if result != nil {
				result.Release()
			}

			result = state
		}

		Convey("It should emit classified depth-flow output without a Kraken envelope", func() {
			So(result, ShouldNotBeNil)
			So(datura.Peek[string](result, "symbol"), ShouldEqual, "BTC/USD")
			So(datura.Peek[string](result, "root"), ShouldEqual, "output")
			So(datura.Peek[float64](result, "output", "category"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](result, "output", "confidence"), ShouldBeGreaterThan, 0)

			result.Release()
		})
	})
}

func TestBookflowSample_ReadStagesColdStart(t *testing.T) {
	Convey("Given a valid first book frame before feature history is ready", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		frame := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`)
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

func TestBookflowSample_ReadPipelineColdStart(t *testing.T) {
	Convey("Given the full bookflow pipeline before feature history is ready", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		bookflow := equation.NewBookflow(bookflowAlgoConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("depthflow-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs": []string{
					"loadedScore",
					"spoofScore",
					"thinScore",
					"neutralScore",
				},
			}),
		)
		pipeline := transport.NewPipeline(encoder, bookflow, classifier)
		frame := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20},{"price":99,"qty":18}],"asks":[{"price":101,"qty":8},{"price":102,"qty":6}]}]}`)
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(frame)

		err := nomagique.RoundTripArtifact(state, pipeline)

		Convey("It should stop cleanly before the classifier", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[string](state, "channel"), ShouldEqual, "book")
			So(datura.Peek[string](state, "root"), ShouldEqual, "")
			So(datura.Peek[string](state, "data", 0, "symbol"), ShouldEqual, "BTC/USD")

			state.Release()
		})
	})
}

func TestBookflowSample_ReadRejectsMissingSymbol(t *testing.T) {
	Convey("Given a book frame without symbol", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		frame := []byte(`{"channel":"book","type":"update","data":[{"bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(frame)

		err := nomagique.RoundTripArtifact(state, encoder)

		So(err, ShouldNotBeNil)
		state.Release()
	})
}

func BenchmarkBookflowSample_Read(b *testing.B) {
	encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
	bookPayload := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20}],"asks":[{"price":101,"qty":8}]}]}`)
	frame := make([]byte, 262144)

	b.ReportAllocs()

	for b.Loop() {
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(bookPayload)
		packed := state.Pack()

		if len(packed) == 0 {
			b.Fatal("bookflow_sample: artifact pack failed")
		}

		_, _ = encoder.Write(packed)
		_, _ = encoder.Read(frame)
	}
}
