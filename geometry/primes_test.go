package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPhaseDialPrimes(testingTB *testing.T) {
	Convey("Given the phase-dial prime table", testingTB, func() {
		Convey("It should expose 512 ascending primes starting at two", func() {
			So(PhaseDialPrimeCount, ShouldEqual, 512)
			So(PhaseDialPrimes[0], ShouldEqual, uint64(2))
			So(PhaseDialPrimes[1], ShouldEqual, uint64(3))

			for index := 1; index < PhaseDialPrimeCount; index++ {
				So(PhaseDialPrimes[index], ShouldBeGreaterThan, PhaseDialPrimes[index-1])
			}
		})
	})
}
