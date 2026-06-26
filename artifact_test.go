package nomagique

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

type artifactEchoStage struct {
	artifact *datura.Artifact
}

func (stage *artifactEchoStage) Write(payload []byte) (int, error) {
	if _, err := stage.artifact.Unpack(payload); err != nil {
		return 0, err
	}

	stage.artifact.MergeOutput("value", 7.0)

	return len(payload), nil
}

func (stage *artifactEchoStage) Read(payload []byte) (int, error) {
	return stage.artifact.PackInto(payload)
}

func TestRoundTripArtifact(testingTB *testing.T) {
	Convey("Given an artifact and a stage", testingTB, func() {
		artifact := datura.Acquire("roundtrip-test", datura.APPJSON).
			WithPayload([]byte(`{"sample":1}`))
		stage := &artifactEchoStage{
			artifact: datura.Acquire("roundtrip-stage", datura.APPJSON),
		}

		Convey("It should send a packed frame and unpack the response", func() {
			err := RoundTripArtifact(artifact, stage)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 7)
		})
	})
}

func BenchmarkRoundTripArtifact(benchmark *testing.B) {
	artifact := datura.Acquire("roundtrip-bench", datura.APPJSON).
		WithPayload([]byte(`{"sample":1}`))
	stage := &artifactEchoStage{
		artifact: datura.Acquire("roundtrip-stage", datura.APPJSON),
	}

	for benchmark.Loop() {
		if err := RoundTripArtifact(artifact, stage); err != nil && err != io.EOF {
			benchmark.Fatal(err)
		}
	}
}
