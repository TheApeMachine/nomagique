package probability_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/algorithm"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/probability"
)

type fixedScore struct {
	artifact *datura.Artifact
	value    float64
}

func newFixedScore(value float64) *fixedScore {
	return &fixedScore{
		artifact: datura.Acquire("fixed-score", datura.Artifact_Type_json),
		value:    value,
	}
}

func (fixedScore *fixedScore) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (fixedScore *fixedScore) Read(p []byte) (int, error) {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, math.Float64bits(fixedScore.value))
	_ = fixedScore.artifact.SetPayload(payload)

	return fixedScore.artifact.Read(p)
}

func (fixedScore *fixedScore) Close() error {
	return nil
}

func readScalar(stage io.ReadWriter, samples ...float64) float64 {
	inbound := datura.Acquire("test-in", datura.Artifact_Type_json)
	payload := make([]byte, 8*len(samples))

	for index, sample := range samples {
		offset := index * 8
		binary.BigEndian.PutUint64(payload[offset:offset+8], math.Float64bits(sample))
	}

	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = stage.Write(buf)

	var outBuf bytes.Buffer
	chunk := make([]byte, 4096)

	for {
		readCount, readErr := stage.Read(chunk)

		if readCount > 0 {
			outBuf.Write(chunk[:readCount])
		}

		if readErr == io.EOF {
			break
		}

		if readErr != nil {
			break
		}
	}

	outbound := datura.Acquire("test-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(outBuf.Bytes())
	readPayload, _ := outbound.Payload()

	if len(readPayload) != 8 {
		return 0
	}

	return math.Float64frombits(binary.BigEndian.Uint64(readPayload))
}

func observeNodeRing(nodeRing *algorithm.NodeRing, values ...float64) {
	payload := make([]byte, 8*len(values))

	for index, value := range values {
		offset := index * 8
		binary.BigEndian.PutUint64(payload[offset:offset+8], math.Float64bits(value))
	}

	inbound := datura.Acquire("node-ring-in", datura.Artifact_Type_json)
	_ = inbound.SetPayload(payload)
	buf, _ := inbound.Message().Marshal()
	_, _ = nodeRing.Write(buf)
	_, _ = nodeRing.Read(make([]byte, len(buf)))
}

func TestNewClassifier(testingTB *testing.T) {
	Convey("Given no score sources", testingTB, func() {
		classifier := probability.NewClassifier()

		Convey("It should reject an empty classifier", func() {
			So(classifier, ShouldBeNil)
		})
	})
}

func TestClassifier_Read(testingTB *testing.T) {
	Convey("Given four wired score sources", testingTB, func() {
		classifier := probability.NewClassifier(
			newFixedScore(0.2),
			newFixedScore(0.1),
			newFixedScore(0.9),
			newFixedScore(0.05),
		)

		So(classifier, ShouldNotBeNil)

		got := readScalar(classifier)

		Convey("It should return a 1-based winning category index", func() {
			So(got, ShouldEqual, 3)
			So(classifier.CategoryIndex(), ShouldEqual, 3)
		})

		Convey("It should expose normalized probabilities", func() {
			confidence, confidenceErr := classifier.Confidence(3)

			So(confidenceErr, ShouldBeNil)
			So(confidence, ShouldBeGreaterThan, 0)
			So(confidence, ShouldBeLessThan, 1)
		})
	})

	Convey("Given non-finite score sources", testingTB, func() {
		classifier := probability.NewClassifier(
			newFixedScore(1),
			newFixedScore(math.NaN()),
		)

		So(classifier, ShouldNotBeNil)

		got := readScalar(classifier)

		Convey("It should leave output unchanged", func() {
			So(got, ShouldEqual, 0)
		})
	})
}

func TestClassifier_Number(testingTB *testing.T) {
	Convey("Given Number composed with Classifier", testingTB, func() {
		classifier := probability.NewClassifier(
			newFixedScore(0.1),
			newFixedScore(0.8),
			newFixedScore(0.2),
		)

		So(classifier, ShouldNotBeNil)

		inbound := datura.Acquire("number-in", datura.Artifact_Type_json)
		buf, _ := inbound.Message().Marshal()
		_, _ = classifier.Write(buf)
		err := nomagique.Number(classifier)

		So(err, ShouldBeNil)

		got := readScalar(classifier)

		Convey("It should return the winning category as float64", func() {
			So(got, ShouldEqual, 2)
		})
	})
}

func TestClassifier_Pearl(testingTB *testing.T) {
	Convey("Given Pearl ladder readings wired into Classifier", testingTB, func() {
		nodeZero := make([]float64, 16)
		nodeOne := make([]float64, 16)
		nodeTwo := make([]float64, 16)
		nodeThree := make([]float64, 16)

		for index := range nodeZero {
			nodeZero[index] = float64(index) * 0.1
			nodeOne[index] = float64(index) * 0.2
			nodeTwo[index] = float64(index) * 0.5
			nodeThree[index] = float64(index) * 0.05
		}

		config := causal.LadderConfig{
			TreatmentNormal:   2,
			ControlsNormal:    []int{0, 1},
			TreatmentInverted: 1,
			ControlsInverted:  []int{0},
			ConditionLeft:     1,
			ConditionRight:    2,
			MinHistory:        12,
		}

		nodes := algorithm.NewNodeRing(4, 16)

		for index := range nodeZero {
			observeNodeRing(
				nodes,
				nodeZero[index],
				nodeOne[index],
				nodeTwo[index],
				nodeThree[index],
			)
		}

		ladder := algorithm.NewPearl(3, config, nodes, newFixedScore(0), nil)
		classifier := probability.NewClassifier(
			ladder.UpliftReading(),
			ladder.ContagionReading(),
			ladder.AssociationReading(),
			ladder.InterventionReading(),
		)

		So(classifier, ShouldNotBeNil)

		inbound := datura.Acquire("pearl-in", datura.Artifact_Type_json)
		buf, _ := inbound.Message().Marshal()
		_, _ = ladder.Write(buf)
		_, _ = ladder.Read(make([]byte, len(buf)))
		_, _ = classifier.Write(buf)
		err := nomagique.Number(ladder, classifier)

		So(err, ShouldBeNil)

		got := readScalar(classifier)

		Convey("It should classify from Pearl outcome readings", func() {
			So(got, ShouldBeGreaterThanOrEqualTo, 1)
			So(got, ShouldBeLessThanOrEqualTo, 4)
			So(classifier.CategoryIndex(), ShouldBeGreaterThanOrEqualTo, 1)
			So(classifier.CategoryIndex(), ShouldBeLessThanOrEqualTo, 4)
		})
	})
}

func BenchmarkClassifier_Read(b *testing.B) {
	classifier := probability.NewClassifier(
		newFixedScore(0.2),
		newFixedScore(0.4),
		newFixedScore(0.7),
		newFixedScore(0.1),
	)

	if classifier == nil {
		b.Fatal("classifier required")
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = readScalar(classifier)
	}
}
