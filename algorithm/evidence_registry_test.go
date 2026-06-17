package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestEvidenceRegistry_Flag(testingTB *testing.T) {
	Convey("Given a fresh registry", testingTB, func() {
		registry := NewEvidenceRegistry()
		expires := time.Unix(1000, 0)
		registry.Flag(100, 0.5, 0.8, expires)

		Convey("It should store churn and strength", func() {
			entry := registry.entries[100]

			So(entry.Churn, ShouldEqual, 0.5)
			So(entry.Strength, ShouldEqual, 0.8)
			So(entry.Expires, ShouldEqual, expires)
		})
	})

	Convey("Given zero evidence fields", testingTB, func() {
		registry := NewEvidenceRegistry()
		expires := time.Unix(2000, 0)
		registry.Flag(200, 0, 0, expires)
		registry.Flag(200, 0.3, 0, expires)

		Convey("It should preserve prior positive churn", func() {
			entry := registry.entries[200]
			So(entry.Churn, ShouldEqual, 0.3)
		})
	})
}

func TestEvidenceRegistry_ActiveExpiry(testingTB *testing.T) {
	Convey("Given active and expired keys", testingTB, func() {
		registry := NewEvidenceRegistry()
		now := time.Unix(5000, 0)
		registry.Flag(1, 0.4, 0.2, now.Add(time.Second))
		registry.Flag(2, 0.1, 0.9, now.Add(-time.Second))

		expiry, churn, strength, ok := registry.ActiveExpiry([]int64{2, 1}, now)

		Convey("It should skip expired entries and prune them", func() {
			So(ok, ShouldBeTrue)
			So(expiry, ShouldEqual, now.Add(time.Second))
			So(churn, ShouldEqual, 0.4)
			So(strength, ShouldEqual, 0.4)
			_, expiredPresent := registry.entries[2]
			So(expiredPresent, ShouldBeFalse)
		})
	})

	Convey("Given missing keys", testingTB, func() {
		registry := NewEvidenceRegistry()
		_, _, _, ok := registry.ActiveExpiry([]int64{99}, time.Now())

		Convey("It should report inactive", func() {
			So(ok, ShouldBeFalse)
		})
	})
}

func TestEvidenceRegistry_Clone(testingTB *testing.T) {
	Convey("Given a populated registry", testingTB, func() {
		registry := NewEvidenceRegistry()
		expires := time.Unix(3000, 0)
		registry.Flag(10, 0.2, 0.6, expires)
		clone := registry.Clone()

		Convey("It should copy entries independently", func() {
			So(clone.entries[10].Strength, ShouldEqual, 0.6)

			clone.Flag(10, 0.9, 0, expires)

			So(registry.entries[10].Churn, ShouldEqual, 0.2)
			So(clone.entries[10].Churn, ShouldEqual, 0.9)
		})
	})

	Convey("Given nil registry", testingTB, func() {
		var registry *EvidenceRegistry
		clone := registry.Clone()

		Convey("It should return an empty registry", func() {
			So(clone, ShouldNotBeNil)
			So(len(clone.entries), ShouldEqual, 0)
		})
	})
}

func TestEvidenceRegistry_NearTouchStrength(testingTB *testing.T) {
	priceFromKey := func(key int64) float64 {
		return float64(key) / 100
	}

	Convey("Given near-touch and far entries", testingTB, func() {
		registry := NewEvidenceRegistry()
		now := time.Unix(4000, 0)
		registry.Flag(10000, 0.3, 0.1, now.Add(time.Minute))
		registry.Flag(20000, 0.9, 0.2, now.Add(time.Minute))

		near, strength := registry.NearTouchStrength(100, 0.02, now, priceFromKey)

		Convey("It should aggregate max strength near mid", func() {
			So(near, ShouldBeTrue)
			So(strength, ShouldEqual, 0.3)
		})
	})

	Convey("Given expired near-touch entry", testingTB, func() {
		registry := NewEvidenceRegistry()
		now := time.Unix(6000, 0)
		registry.Flag(10000, 0.5, 0.5, now.Add(-time.Second))

		near, strength := registry.NearTouchStrength(100, 0.02, now, priceFromKey)

		Convey("It should prune and ignore expired evidence", func() {
			So(near, ShouldBeFalse)
			So(strength, ShouldEqual, 0)
			_, present := registry.entries[10000]
			So(present, ShouldBeFalse)
		})
	})
}
