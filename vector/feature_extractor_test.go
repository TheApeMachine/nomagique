package vector

import (
	"bytes"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var featureExtractorPayloadFixture = []byte(`{"inputs":["price","volume"],"data":{"price":[{"label":"price","value":10.5,"transform":"ema"}],"volume":[{"label":"pad","value":0,"transform":"ema"},{"label":"volume","value":2500,"transform":"ema"}]}}`)

func featureExtractorSchema() *datura.Artifact {
	return datura.Acquire("feature-extractor-test", datura.APPJSON).WithPayload(featureExtractorPayloadFixture)
}

func WithFeatureExtractor(
	schema *datura.Artifact,
	block func(extractor *FeatureExtractor),
) func() {
	return func() {
		block(NewFeatureExtractor(schema))
	}
}

func TestNewFeatureExtractor(t *testing.T) {
	Convey("Given a schema artifact", t, func() {
		schema := datura.Acquire("feature-extractor-schema", datura.APPJSON)

		Convey("When NewFeatureExtractor is called", func() {
			extractor := NewFeatureExtractor(schema)

			Convey("It should retain the schema and initialize transform cache", func() {
				So(extractor, ShouldNotBeNil)
				So(extractor.artifact, ShouldEqual, schema)
				So(extractor.transforms, ShouldNotBeNil)
			})
		})
	})
}

func TestFeatureExtractor_Write(t *testing.T) {
	Convey("Given a feature extractor", t, func() {
		extractor := NewFeatureExtractor(datura.Acquire("test", datura.Artifact_Type_json))

		Convey("And an inbound artifact with a payload", func() {
			inbound := datura.Acquire("test", datura.Artifact_Type_json).WithPayload([]byte{1, 2, 3})

			Convey("When the inbound artifact is copied into the extractor", func() {
				_, err := io.Copy(extractor, inbound)

				Convey("Then the extractor artifact should carry the inbound payload", func() {
					So(err, ShouldBeNil)

					var payload []byte

					payload, err = extractor.artifact.Payload()
					So(err, ShouldBeNil)
					So(payload, ShouldResemble, []byte{1, 2, 3})
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
	Convey("Given a feature extractor with schema payload", t, func() {
		extractor := NewFeatureExtractor(featureExtractorSchema())

		Convey("When the extractor is read", func() {
			buffer := bytes.NewBuffer([]byte{})
			_, err := io.Copy(buffer, extractor)
			So(err, ShouldBeNil)

			Convey("Then the output artifact payload should carry extracted features", func() {
				decoded := datura.Acquire("feature-extractor", datura.APPJSON)
				_, err = decoded.Write(buffer.Bytes())
				So(err, ShouldBeNil)

				var payload []byte

				payload, err = decoded.DecryptPayload()
				So(err, ShouldBeNil)
				So(string(payload), ShouldEqual, `{"features":[10.5,2500]}`)
			})
		})
	})
}

func BenchmarkFeatureExtractor_Read(b *testing.B) {
	extractor := NewFeatureExtractor(
		datura.Acquire("feature-extractor-bench", datura.APPJSON).
			WithPayload(featureExtractorPayloadFixture),
	)

	buffer := make([]byte, 65536)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = extractor.Read(buffer)
	}
}
