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

func bookQualitySampleConfig() *datura.Artifact {
	return datura.Acquire("book-quality", datura.APPJSON).WithAttributes(datura.Map[any]{
		"vacuumGate": datura.Map[any]{
			"percentile": 0.9,
			"minSamples": 3.0,
		},
		"churnGate": datura.Map[any]{
			"percentile": 0.75,
			"minSamples": 3.0,
		},
		"cancelQtyGate": datura.Map[any]{
			"percentile": 0.5,
			"minSamples": 3.0,
		},
		"levelSizeGate": datura.Map[any]{
			"percentile": 0.75,
			"minSamples": 3.0,
		},
		"vacuumLowPercentile": 0.25,
	})
}

func TestNewBookQualitySample(t *testing.T) {
	Convey("Given a book quality sample stage", t, func() {
		stage := NewBookQualitySample(bookQualitySampleConfig())

		Convey("It should be constructible", func() {
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestBookQualitySample_Read(t *testing.T) {
	Convey("Given a liquidity vacuum book replay", t, func() {
		encoder := NewBookQualitySample(bookQualitySampleConfig())
		bookQuality := equation.NewBookQuality(equation.BookQualityConfig())
		classifier := probability.NewClassifier(
			datura.Acquire("toxicity-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs":    []string{"bluffScore", "vacuumScore", "supportScore"},
				"scoreRoot": "output",
			}),
		)
		pipeline := transport.NewPipeline(encoder, bookQuality, classifier)

		frames := [][]byte{
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":12}],"asks":[{"price":101,"qty":10}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":12}],"asks":[{"price":101,"qty":10}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":12}],"asks":[{"price":101,"qty":10}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":3}],"asks":[{"price":101,"qty":10}]}]}`),
			[]byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":1}],"asks":[{"price":101,"qty":10}]}]}`),
		}

		var (
			result         *datura.Artifact
			bestConfidence float64
			bestEvidence   float64
		)

		for _, frame := range frames {
			state := datura.Acquire("measurement", datura.APPJSON).
				WithRole("measurement").
				WithScope("update").
				WithPayload(frame)

			err := nomagique.RoundTripArtifact(state, pipeline)

			if err != nil {
				state.Release()
				continue
			}

			confidence := datura.Peek[float64](state, "output", "confidence")
			evidence := datura.Peek[float64](state, "output", "bluffScore") +
				datura.Peek[float64](state, "output", "vacuumScore") +
				datura.Peek[float64](state, "output", "supportScore")

			if confidence > bestConfidence {
				bestConfidence = confidence
			}

			if evidence > bestEvidence {
				bestEvidence = evidence
			}

			if result != nil {
				result.Release()
			}

			result = state
		}

		Convey("It should emit non-uniform classifier output", func() {
			So(result, ShouldNotBeNil)

			category := datura.Peek[float64](result, "output", "category")

			So(category, ShouldBeGreaterThan, 0)
			So(bestConfidence, ShouldBeGreaterThan, 0)
			So(bestConfidence, ShouldNotAlmostEqual, 1.0/3.0, 0.0001)
			So(bestEvidence, ShouldBeGreaterThan, 0)

			result.Release()
		})
	})
}

func readBookQualitySample(
	encoder *BookQualitySample,
	payload string,
) (*datura.Artifact, error) {
	state := datura.Acquire("measurement", datura.APPJSON).
		WithRole("measurement").
		WithScope("update").
		WithPayload([]byte(payload))
	packed := state.Pack()
	state.Release()

	if _, err := encoder.Write(packed); err != nil {
		return nil, err
	}

	buffer := make([]byte, 65536)
	n, err := encoder.Read(buffer)

	if err != nil && err != io.EOF {
		return nil, err
	}

	if n == 0 {
		return nil, err
	}

	outbound := datura.Acquire("book-quality-read", datura.APPJSON)
	_, unpackErr := outbound.Unpack(buffer[:n])

	if unpackErr != nil {
		outbound.Release()

		return nil, unpackErr
	}

	return outbound, err
}

func TestBookQualitySample_ReadsLevel3RowArtifacts(t *testing.T) {
	Convey("Given level3 row artifacts for multiple symbols", t, func() {
		encoder := NewBookQualitySample(bookQualitySampleConfig())

		btc, btcErr := readBookQualitySample(
			encoder,
			`{"symbol":"BTC/USD","bids":[{"order_id":"B1","limit_price":100,"order_qty":20}],"asks":[{"order_id":"A1","limit_price":101,"order_qty":20}]}`,
		)

		Convey("It should emit L3 features for the first symbol", func() {
			So(btcErr, ShouldEqual, io.EOF)
			So(btc, ShouldNotBeNil)
			So(datura.Peek[float64](btc, "output", "l3"), ShouldEqual, 1)
			So(datura.Peek[[]float64](btc, "features")[12], ShouldEqual, 100.5)

			btc.Release()
		})

		eth, ethErr := readBookQualitySample(
			encoder,
			`{"symbol":"ETH/USD","bids":[{"order_id":"B1","limit_price":200,"order_qty":20}],"asks":[{"order_id":"A1","limit_price":201,"order_qty":20}]}`,
		)

		Convey("It should keep per-symbol books isolated", func() {
			So(ethErr, ShouldEqual, io.EOF)
			So(eth, ShouldNotBeNil)
			So(datura.Peek[[]float64](eth, "features")[12], ShouldEqual, 200.5)

			eth.Release()
		})
	})
}

func BenchmarkBookQualitySample_Read(b *testing.B) {
	encoder := NewBookQualitySample(datura.Acquire("book-quality", datura.APPJSON))
	frame := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":10}],"asks":[{"price":101,"qty":10}]}]}`)
	buf := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		state := datura.Acquire("measurement", datura.APPJSON).WithPayload(frame)
		packed := state.Pack()

		if len(packed) == 0 {
			b.Fatal("book_quality_sample: artifact pack failed")
		}

		_, _ = encoder.Write(packed)
		_, _ = encoder.Read(buf)
	}
}
