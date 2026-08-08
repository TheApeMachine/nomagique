//go:build darwin && cgo

package fluid

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFieldsValidate(t *testing.T) {
	Convey("Given complete arrays for the declared fluid lattice", t, func() {
		grid := testDomainConfig().Grid
		cells := grid.X * grid.Y * grid.Z
		fields := Fields{
			Grid:           grid,
			Density:        make([]float32, cells),
			Momentum:       make([]float32, cells*3),
			InternalEnergy: make([]float32, cells),
			WaveReal:       make([]float32, cells),
			WaveImaginary:  make([]float32, cells),
		}

		Convey("It should accept the exact scalar and vector shapes", func() {
			So(fields.Validate(), ShouldBeNil)
		})

		Convey("It should reject a truncated spatial wave", func() {
			fields.WaveImaginary = fields.WaveImaginary[:cells-1]
			So(fields.Validate(), ShouldNotBeNil)
		})
	})
}

func TestDomainFields(t *testing.T) {
	Convey("Given a resident domain after one coupled step", t, func() {
		domain, err := NewDomain(testDomainConfig())
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })
		_, err = domain.Step(testParticles())
		So(err, ShouldBeNil)

		fields, err := domain.Fields()

		Convey("It should expose the complete finite gas and spatial wave lattices", func() {
			So(err, ShouldBeNil)
			So(fields.Validate(), ShouldBeNil)
			So(finiteValues(fields.Density), ShouldBeTrue)
			So(finiteValues(fields.Momentum), ShouldBeTrue)
			So(finiteValues(fields.InternalEnergy), ShouldBeTrue)
			So(finiteValues(fields.WaveReal), ShouldBeTrue)
			So(finiteValues(fields.WaveImaginary), ShouldBeTrue)
			So(maxValue(fields.Density), ShouldBeGreaterThan, float32(0))
		})
	})
}

func BenchmarkDomainFields(b *testing.B) {
	config := DefaultConfig()
	domain, err := NewDomain(config)

	if err != nil {
		b.Fatal(err)
	}

	defer domain.Close()

	if _, err = domain.Step(benchmarkParticles(config.Grid)); err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if _, err := domain.Fields(); err != nil {
			b.Fatal(err)
		}
	}
}
