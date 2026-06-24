package learning

import (
	"fmt"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/vector"
)

func tickerFrame(volume, last float64) *datura.Artifact {
	payload := fmt.Sprintf(
		`{"channel":"ticker","type":"update","data":[{"symbol":"BTC/USD","volume":%g,"last":%g}]}`,
		volume, last,
	)
	frame := datura.Acquire("pipeline-wire-ticker", datura.APPJSON)
	frame.WithPayload([]byte(payload))

	return frame
}

func TestPipelineWireChain(testingTB *testing.T) {
	Convey("Given FeatureExtractor through Classifier on shared wire", testingTB, func() {
		extractorSchema := datura.Acquire("pipeline-extractor", datura.APPJSON).WithAttributes(datura.Map[any]{
			"ticker": datura.Map[any]{
				"root":   "data",
				"inputs": []string{"volume", "last"},
			},
		})

		meanMedianConfig := datura.Acquire("pipeline-mmr", datura.APPJSON).WithAttributes(datura.Map[any]{
			"root":   "features",
			"inputs": []string{"volume", "last"},
			"order":  []string{"rvol"},
			"rvol": datura.Map[any]{
				"input":       "volume",
				"shortWindow": 3.0,
				"longWindow":  5.0,
				"outputKey":   "rvol",
			},
		})

		logitConfig := datura.Acquire("pipeline-logit", datura.APPJSON).WithAttributes(datura.Map[any]{
			"root":      "output",
			"scoreRoot": "output",
			"inputs":    []string{"rvol"},
			"order":     []string{"rvol"},
			"outputs": []string{
				"ignition",
				"exhaustion",
			},
			"threshold": 1.0,
			"rvol": datura.Map[any]{
				"source": "rvol",
				"scale":  2.0,
			},
			"ignition": datura.Map[any]{
				"terms": []string{"rvol"},
			},
			"exhaustion": datura.Map[any]{
				"terms":   []string{"rvol"},
				"inverts": []string{"rvol"},
			},
		})

		classifierConfig := datura.Acquire("pipeline-classifier", datura.APPJSON).
			Poke("output", "scoreRoot").
			Poke([]string{"ignition", "exhaustion", "strength"}, "inputs")

		pipeline := nomagique.Number(
			vector.NewFeatureExtractor(extractorSchema),
			statistic.NewMeanMedianRatio(meanMedianConfig),
			NewLogitScores(logitConfig),
			probability.NewClassifier(classifierConfig),
		)

		volumes := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 50}
		var lastFrame *datura.Artifact

		for index, volume := range volumes {
			frame := tickerFrame(volume, 100+float64(index)*0.01)
			err := transport.NewFlipFlop(frame, pipeline)

			if index < 4 {
				So(err, ShouldBeIn, nil, io.EOF)
			}

			if index >= 4 {
				So(err, ShouldBeNil)
			}

			if lastFrame != nil {
				lastFrame.Release()
			}

			lastFrame = frame
		}

		defer lastFrame.Release()

		Convey("It should classify from chained wire outputs", func() {
			category := int(datura.Peek[float64](lastFrame, "output", "category"))
			ignition := datura.Peek[float64](lastFrame, "output", "ignition")
			exhaustion := datura.Peek[float64](lastFrame, "output", "exhaustion")

			So(category, ShouldBeBetweenOrEqual, 1, 2)
			So(ignition, ShouldBeGreaterThan, 0)
			So(exhaustion, ShouldBeGreaterThanOrEqualTo, 0)
		})
	})
}
