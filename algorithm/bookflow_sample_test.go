package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
)

func TestBookflowSample_Read(t *testing.T) {
	Convey("Given repeated bid-heavy book frames", t, func() {
		encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
		bookflow := equation.NewBookflow()
		classifier := probability.NewClassifier(
			datura.Acquire("depthflow-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
				"inputs": []string{"loadedScore", "spoofScore", "thinScore", "neutralScore"},
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

			err := transport.NewFlipFlop(state, pipeline)

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
			So(datura.Peek[float64](result, "output", "value"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](result, "output", "confidence"), ShouldBeGreaterThan, 0.25)

			result.Release()
		})
	})
}

func BenchmarkBookflowSample_Read(b *testing.B) {
	encoder := NewBookflowSample(datura.Acquire("bookflow-sample", datura.APPJSON))
	bookPayload := []byte(`{"channel":"book","type":"update","data":[{"symbol":"BTC/USD","bids":[{"price":100,"qty":20}],"asks":[{"price":101,"qty":8}]}]}`)
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = encoder.Write(bookPayload)
		_, _ = encoder.Read(frame)
	}
}
