package algorithm_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/algorithm"
)

func pearlConfig() *datura.Artifact {
	return datura.Acquire("pearl-config", datura.APPJSON).
		Poke(float64(3), "target").
		Poke(float64(12), "minHistory").
		Poke(float64(12), "history").
		Poke(float64(2), "treatmentNormal").
		Poke([]float64{0, 1}, "controlsNormal").
		Poke(float64(1), "treatmentInverted").
		Poke([]float64{0}, "controlsInverted").
		Poke(float64(1), "conditionLeft").
		Poke(float64(2), "conditionRight").
		Poke([]float64{0, 3}, "contagionSkip").
		Poke(0.35, "kernelBandwidth").
		Poke(0.8, "contagionBreak").
		Poke("rawInverted", "input").
		Poke(float64(3), "window")
}

func pearlInbound() *datura.Artifact {
	nodeCount := 4
	rowCount := 16
	flat := make([]float64, 0, rowCount*nodeCount)

	for rowIndex := range rowCount {
		flat = append(flat,
			float64(rowIndex)*0.1,
			float64(rowIndex)*0.2,
			float64(rowIndex)*0.5,
			float64(rowIndex)*0.05,
		)
	}

	return datura.Acquire("pearl-inbound", datura.APPJSON).
		Poke(0.0, "paired").
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}

func pearlTicker(index int) *datura.Artifact {
	bidQty := 740.0 + float64(index)
	askQty := 720.0 + float64(index)

	payload := fmt.Appendf(
		nil,
		`{"channel":"ticker","type":"update","data":[{"symbol":"BTC/USD","bid":%g,"bid_qty":%g,"ask":%g,"ask_qty":%g,"last":%g,"volume":%g,"change_pct":%g}]}`,
		49990.0+float64(index),
		bidQty,
		50010.0+float64(index),
		askQty,
		50000.0+float64(index),
		1000.0+float64(index*10),
		0.01*float64(index),
	)

	return datura.Acquire("kraken:public", datura.APPJSON).
		WithRole("ticker").
		WithScope("update").
		WithPayload(payload)
}

func pearlTrade(index int) *datura.Artifact {
	side := "buy"

	if index%2 == 1 {
		side = "sell"
	}

	payload := fmt.Appendf(
		nil,
		`{"channel":"trade","type":"update","data":[{"symbol":"BTC/USD","side":%q,"price":%g,"qty":%g}]}`,
		side,
		50000.0+float64(index),
		1.0+float64(index)*0.1,
	)

	return datura.Acquire("kraken:public", datura.APPJSON).
		WithRole("trade").
		WithScope("update").
		WithPayload(payload)
}

func pearlRow(symbol string, row datura.Map[any]) *datura.Artifact {
	row["symbol"] = symbol

	return datura.Acquire("pearl-row", datura.APPJSON).
		WithScope(symbol).
		WithPayload(row.Marshal())
}

func TestPearl_Read(testingTB *testing.T) {
	Convey("Given aligned node streams with causal structure", testingTB, func() {
		pearl := algorithm.NewPearl(pearlConfig())
		artifact := pearlInbound()
		err := nomagique.RoundTripArtifact(artifact, pearl)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "intervention"), ShouldBeGreaterThan, 0)
	})
}

func TestPearl_ReadPipelineColdStart(testingTB *testing.T) {
	Convey("Given the full Pearl pipeline before causal history is ready", testingTB, func() {
		pearl := algorithm.NewPearl(pearlConfig())
		artifact := pearlTicker(0)

		err := nomagique.RoundTripArtifact(artifact, pearl)
		payload := strings.TrimSpace(string(artifact.DecryptPayload()))

		Convey("It should stop cleanly before classification", func() {
			So(err, ShouldBeNil)
			So(payload, ShouldNotEqual, "")
			So(payload[:1], ShouldEqual, "{")
			So(datura.Peek[string](artifact, "channel"), ShouldEqual, "ticker")
			So(datura.Peek[string](artifact, "root"), ShouldEqual, "")
		})
	})
}

func TestPearlSample_ReadFrameContract(testingTB *testing.T) {
	Convey("Given the Pearl sample stage without an inbound frame", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		buffer := make([]byte, 1024)

		read, err := sample.Read(buffer)

		Convey("It should stop without attempting to unpack stale state", func() {
			So(read, ShouldEqual, 0)
			So(err, ShouldEqual, io.EOF)
		})
	})

	Convey("Given the Pearl sample stage after an emitted frame is drained", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		artifact := pearlTicker(0)
		_, writeErr := nomagique.WriteArtifact(sample, artifact)
		buffer := make([]byte, 262144)
		read, readErr := sample.Read(buffer)
		nextRead, nextErr := sample.Read(buffer)

		Convey("It should expose exactly one frame for that write", func() {
			So(writeErr, ShouldBeNil)
			So(read, ShouldBeGreaterThan, 0)
			So(readErr, ShouldEqual, io.EOF)
			So(nextRead, ShouldEqual, 0)
			So(nextErr, ShouldEqual, io.EOF)
		})
	})

	Convey("Given an empty Pearl sample write", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		written, writeErr := sample.Write(nil)
		buffer := make([]byte, 1024)
		read, readErr := sample.Read(buffer)

		Convey("It should stop instead of reporting a validation error", func() {
			So(written, ShouldEqual, 0)
			So(writeErr, ShouldBeNil)
			So(read, ShouldEqual, 0)
			So(readErr, ShouldEqual, io.EOF)
		})
	})
}

func TestPearl_ReadsRawTickerFrames(testingTB *testing.T) {
	Convey("Given the Pearl sample stage and raw Kraken ticker frames", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		var artifact *datura.Artifact

		for index := range 16 {
			artifact = pearlTicker(index)
			_ = nomagique.RoundTripArtifact(artifact, sample)
		}

		Convey("It should retain aligned node streams", func() {
			So(artifact, ShouldNotBeNil)
			So(datura.Peek[float64](artifact, "streams", "nodeCount"), ShouldEqual, 4)
		})
	})

	Convey("Given raw Kraken ticker and trade frames", testingTB, func() {
		pearl := algorithm.NewPearl(pearlConfig())
		var artifact *datura.Artifact
		var streamCount float64
		var tableRows float64

		for index := range 16 {
			artifact = pearlTicker(index)
			_ = nomagique.RoundTripArtifact(artifact, pearl)
			artifact = pearlTrade(index)
			_ = nomagique.RoundTripArtifact(artifact, pearl)
			streamCount = datura.Peek[float64](artifact, "streams", "nodeCount")
			tableRows = datura.Peek[float64](artifact, "table", "rowCount")
		}

		Convey("It should emit causal output after history is available", func() {
			So(artifact, ShouldNotBeNil)
			So(streamCount, ShouldEqual, 4)
			So(tableRows, ShouldBeGreaterThanOrEqualTo, 12)
			So(datura.Peek[float64](artifact, "output", "confidence"), ShouldBeGreaterThan, 0)
		})
	})
}

func TestPearlSample_ReadsRowArtifacts(testingTB *testing.T) {
	Convey("Given row artifacts for multiple symbols", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		var btc *datura.Artifact

		for index := range 6 {
			btc = pearlRow("BTC/USD", datura.Map[any]{
				"bid":        49990.0 + float64(index),
				"bid_qty":    20.0 + float64(index),
				"ask":        50010.0 + float64(index),
				"ask_qty":    18.0 + float64(index),
				"last":       50000.0 + float64(index),
				"change_pct": 0.01 * float64(index),
			})
			_ = nomagique.RoundTripArtifact(btc, sample)
		}

		eth := pearlRow("ETH/USD", datura.Map[any]{
			"bid":        2990.0,
			"bid_qty":    9.0,
			"ask":        3010.0,
			"ask_qty":    8.0,
			"last":       3000.0,
			"change_pct": 0.02,
		})
		_ = nomagique.RoundTripArtifact(eth, sample)

		Convey("It should retain separate node streams per symbol", func() {
			So(hasSample(datura.Peek[[]float64](btc, "streams", "0"), 0.0005), ShouldBeTrue)
			So(hasSample(datura.Peek[[]float64](eth, "streams", "0"), 0.0002), ShouldBeTrue)
			So(hasSample(datura.Peek[[]float64](eth, "streams", "0"), 0.0005), ShouldBeFalse)
		})
	})

	Convey("Given a book row artifact", testingTB, func() {
		sample := algorithm.NewPearlSample(pearlConfig())
		book := pearlRow("BTC/USD", datura.Map[any]{
			"bids": []datura.Map[any]{
				{"price": 100.0, "qty": 4.0},
				{"price": 99.0, "qty": 10.0},
			},
			"asks": []datura.Map[any]{
				{"price": 101.0, "qty": 3.0},
				{"price": 102.0, "qty": 10.0},
			},
		})
		err := nomagique.RoundTripArtifact(book, sample)

		Convey("It should encode book liquidity stress into Pearl node streams", func() {
			So(err, ShouldBeNil)
			So(datura.Peek[float64](book, "streams", "nodeCount"), ShouldEqual, 4)
			So(hasPositive(datura.Peek[[]float64](book, "streams", "1")), ShouldBeTrue)
		})
	})
}

func hasSample(values []float64, expected float64) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}

	return false
}

func hasPositive(values []float64) bool {
	for _, value := range values {
		if value > 0 {
			return true
		}
	}

	return false
}

func BenchmarkPearl_Read(testingTB *testing.B) {
	pearl := algorithm.NewPearl(pearlConfig())

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact := pearlInbound()
		_ = nomagique.RoundTripArtifact(artifact, pearl)
	}
}
