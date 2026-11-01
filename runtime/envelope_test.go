package runtime_test

import (
	"encoding/json"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/runtime"
)

func TestEnvelope_Publish(testingTB *testing.T) {
	Convey("Given an envelope with region rows", testingTB, func() {
		envelope := runtime.NewEnvelope(nil)
		envelope.Regions["BTC/USD"] = []*runtime.Region{{
			Symbol:     "BTC/USD",
			Key:        1,
			Brightness: 0.9,
			Counts:     2,
		}}

		Convey("It wraps the payload under the regions store key", func() {
			raw := envelope.Publish("regions")
			So(raw, ShouldNotBeNil)

			var frame map[string]any
			So(json.Unmarshal(raw, &frame), ShouldBeNil)
			So(frame["regions"], ShouldNotBeNil)

			rows, ok := frame["regions"].(map[string]any)
			So(ok, ShouldBeTrue)
			So(rows["BTC/USD"], ShouldNotBeNil)
		})
	})
}

func TestSonic_EncodeNullForInfOrNan(testingTB *testing.T) {
	Convey("Given non-finite float values", testingTB, func() {
		raw, err := runtime.Sonic.Marshal(map[string]float64{
			"nan": math.NaN(),
			"inf": math.Inf(1),
		})

		Convey("It encodes them as JSON null instead of failing", func() {
			So(err, ShouldBeNil)

			var decoded map[string]any
			So(json.Unmarshal(raw, &decoded), ShouldBeNil)
			So(decoded["nan"], ShouldBeNil)
			So(decoded["inf"], ShouldBeNil)
		})
	})
}
