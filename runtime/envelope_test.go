package runtime_test

import (
	"encoding/json"
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
