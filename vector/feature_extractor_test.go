package vector

import (
	"bytes"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var featureExtractorPayloadFixture = []byte(
	`{"channel":"ticker","type":"update","data":[{"symbol":"BTC/USD","volume":2500,"vwap":100,"last":101,"bid":100.9,"ask":101.1,"change_pct":1.0}]}`,
)

func featureExtractorSchema() *datura.Artifact {
	return datura.Acquire("feature-extractor-test", datura.APPJSON).
		Poke("data", "root").
		Poke([]string{"volume", "vwap", "last", "bid", "ask", "change_pct"}, "order").
		Poke(map[string]any{
			"volume":     map[string]any{},
			"vwap":       map[string]any{},
			"last":       map[string]any{},
			"bid":        map[string]any{},
			"ask":        map[string]any{},
			"change_pct": map[string]any{},
		}, "inputs")
}

func TestNewFeatureExtractor(t *testing.T) {
	Convey("Given a schema artifact", t, func() {
		schema := datura.Acquire("feature-extractor-schema", datura.APPJSON)

		Convey("When NewFeatureExtractor is called", func() {
			extractor := NewFeatureExtractor(schema)

			Convey("It should retain the schema and initialize transform cache", func() {
				So(extractor, ShouldNotBeNil)
				So(extractor.config, ShouldEqual, schema)
				So(extractor.transforms, ShouldNotBeNil)
			})
		})
	})
}

func TestFeatureExtractor_Write(t *testing.T) {
	Convey("Given a feature extractor", t, func() {
		extractor := NewFeatureExtractor(
			datura.Acquire("test", datura.APPJSON).
				Poke("data", "root").
				Poke([]string{"volume"}, "order").
				Poke(map[string]any{"volume": map[string]any{}}, "inputs"),
		)

		Convey("And an inbound artifact with a payload", func() {
			inbound := datura.Acquire("test", datura.APPJSON).WithPayload([]byte{1, 2, 3})

			Convey("When the inbound artifact is copied into the extractor", func() {
				_, err := io.Copy(extractor, inbound)

				Convey("Then the staged artifact should carry the inbound payload", func() {
					So(err, ShouldBeNil)

					payload := extractor.staged.DecryptPayload()
					So(len(payload), ShouldBeGreaterThan, 0)
					So(payload, ShouldResemble, []byte{1, 2, 3})
				})

				Convey("Then the schema config should remain on the config artifact", func() {
					So(datura.Peek[string](extractor.config, "root"), ShouldEqual, "data")
				})
			})
		})
	})
}

func TestFeatureExtractor_Close(t *testing.T) {
	Convey("Given a feature extractor", t, func() {
		extractor := NewFeatureExtractor(datura.Acquire("test", datura.APPJSON))

		Convey("When Close is called", func() {
			err := extractor.Close()

			Convey("It should succeed without error", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestFeatureExtractor_Read(t *testing.T) {
	Convey("Given a feature extractor with ticker payload", t, func() {
		extractor := NewFeatureExtractor(featureExtractorSchema())

		Convey("When ticker payload is written then read", func() {
			inbound := datura.Acquire("test", datura.APPJSON).WithPayload(featureExtractorPayloadFixture)
			_, err := io.Copy(extractor, inbound)
			So(err, ShouldBeNil)

			buffer := bytes.NewBuffer([]byte{})
			_, err = io.Copy(buffer, extractor)
			So(err, ShouldBeNil)

			Convey("Then the payload should carry extracted features", func() {
				decoded := datura.Acquire("feature-extractor", datura.APPJSON)
				_, err = decoded.Write(buffer.Bytes())
				So(err, ShouldBeNil)

				payload := decoded.DecryptPayload()
				So(len(payload), ShouldBeGreaterThan, 0)
				So(string(payload), ShouldEqual, `{"features":[2500,100,101,100.9,101.1,1]}`)
			})
		})
	})
}

func BenchmarkFeatureExtractor_Read(b *testing.B) {
	extractor := NewFeatureExtractor(featureExtractorSchema())
	inbound := datura.Acquire("test", datura.APPJSON).WithPayload(featureExtractorPayloadFixture)
	_, _ = io.Copy(extractor, inbound)

	buffer := make([]byte, 65536)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractor.Read(buffer)
	}
}
