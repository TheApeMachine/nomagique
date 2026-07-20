//go:build darwin && cgo

package fluid

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/geometry"
)

func testDomainConfig() Config {
	return Config{
		Grid: Grid{
			X:       4,
			Y:       4,
			Z:       4,
			Spacing: 0.25,
		},
		MaxDelta: 0.015,
		OmegaMin: -4,
		OmegaMax: 4,
	}
}

func testParticles() []Particle {
	return []Particle{
		{
			Position: Vector{X: -0.75, Y: 0.25, Z: 0.25},
			Velocity: Vector{X: 0.1},
			Mass:     1,
			Heat:     0.4,
			Energy:   0.2,
			Phase:    0,
			Omega:    0.5,
		},
		{
			Position: Vector{X: 0.75, Y: 0.75, Z: 0.75},
			Velocity: Vector{Y: -0.1},
			Mass:     1,
			Heat:     0.6,
			Energy:   0.3,
			Phase:    math.Pi / 2,
			Omega:    1.5,
		},
	}
}

func TestConfigValidate(t *testing.T) {
	Convey("Given a topology larger than the Metal wave reduction capacity", t, func() {
		config := Config{
			Grid: Grid{
				X:       256,
				Y:       1,
				Z:       1,
				Spacing: 1.0 / 256.0,
			},
			MaxDelta: 0.015,
			OmegaMin: -4,
			OmegaMax: 4,
		}

		Convey("It should reject the unsupported derived omega lattice explicitly", func() {
			So(config.Validate(), ShouldNotBeNil)
		})
	})

	Convey("Given omega bounds that do not define an increasing interval", t, func() {
		config := testDomainConfig()
		config.OmegaMax = config.OmegaMin

		Convey("It should reject an ambiguous shared spectral coordinate", func() {
			So(config.Validate(), ShouldNotBeNil)
		})
	})
}

func TestDomainStep(t *testing.T) {
	Convey("Given the standalone Sensorium Metal fluid domain", t, func() {
		domain, err := NewDomain(testDomainConfig())
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })
		particles := testParticles()

		diagnostics, stepErr := domain.Step(particles)

		Convey("It should advance through an accepted dynamically sized gas step", func() {
			So(stepErr, ShouldBeNil)
			So(diagnostics.DeltaDerived, ShouldBeGreaterThan, 0)
			So(diagnostics.DeltaUsed, ShouldBeGreaterThan, 0)
			So(diagnostics.DeltaUsed, ShouldBeLessThanOrEqualTo, diagnostics.DeltaDerived)
			So(diagnostics.DeltaUsed, ShouldBeLessThanOrEqualTo, testDomainConfig().MaxDelta)
		})

		Convey("It should return finite periodic particle state", func() {
			for _, particle := range particles {
				So(finiteParticle(particle), ShouldBeTrue)
				So(particle.Position.X, ShouldBeGreaterThanOrEqualTo, 0)
				So(particle.Position.X, ShouldBeLessThan, 1)
				So(particle.Position.Y, ShouldBeGreaterThanOrEqualTo, 0)
				So(particle.Position.Y, ShouldBeLessThan, 1)
				So(particle.Position.Z, ShouldBeGreaterThanOrEqualTo, 0)
				So(particle.Position.Z, ShouldBeLessThan, 1)
			}
		})

		Convey("It should evolve a finite omega wave field", func() {
			wave, waveErr := domain.Wave()
			So(waveErr, ShouldBeNil)
			So(len(wave), ShouldEqual, 4)
			So(wave[0].Omega, ShouldAlmostEqual, testDomainConfig().OmegaMin)
			So(wave[len(wave)-1].Omega, ShouldAlmostEqual, testDomainConfig().OmegaMax)
			var magnitude float64

			for _, mode := range wave {
				So(math.IsNaN(float64(mode.Real)), ShouldBeFalse)
				So(math.IsNaN(float64(mode.Imaginary)), ShouldBeFalse)
				So(mode.Linewidth, ShouldBeGreaterThan, 0)
				magnitude += math.Hypot(float64(mode.Real), float64(mode.Imaginary))
			}

			So(magnitude, ShouldBeGreaterThan, 0)
			So(diagnostics.PsiRMS, ShouldBeGreaterThan, 0)
		})

		Convey("It should execute finite spatial pilot-wave guidance", func() {
			So(math.IsNaN(float64(diagnostics.GuidanceRMS)), ShouldBeFalse)
			So(math.IsInf(float64(diagnostics.GuidanceRMS), 0), ShouldBeFalse)
			So(diagnostics.GuidanceRMS, ShouldBeGreaterThan, 0)
		})

		Convey("It should remain admissible across two periods of the slowest oscillator", func() {
			slowestPeriod := 2 * math.Pi / float64(particles[0].Omega)
			steps := int(math.Ceil(
				2 * slowestPeriod / float64(testDomainConfig().Grid.Spacing),
			))

			for range steps {
				nextDiagnostics, nextErr := domain.Step(particles)
				So(nextErr, ShouldBeNil)
				So(nextDiagnostics.DeltaUsed, ShouldBeGreaterThan, 0)

				for _, particle := range particles {
					So(finiteParticle(particle), ShouldBeTrue)
				}
			}
		})

		Convey("It should accept a changing complete population without resetting the wave", func() {
			particles = append(particles, Particle{
				Position: Vector{X: 0.5, Y: 0.5, Z: 0.5},
				Mass:     1,
				Heat:     0.5,
				Energy:   1,
				Phase:    math.Pi,
				Omega:    2,
			})

			nextDiagnostics, nextErr := domain.Step(particles)
			So(nextErr, ShouldBeNil)
			So(nextDiagnostics.PsiRMS, ShouldBeGreaterThan, 0)
			So(finiteParticle(particles[2]), ShouldBeTrue)

			shrunkDiagnostics, shrinkErr := domain.Step(particles[:2])
			So(shrinkErr, ShouldBeNil)
			So(shrunkDiagnostics.PsiRMS, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a shared market-scale particle population", t, func() {
		config := DefaultConfig()
		domain, err := NewDomain(config)
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })
		particles := benchmarkParticles(config.Grid)
		var diagnostics Diagnostics

		for range config.Grid.Z {
			diagnostics, err = domain.Step(particles)
			So(err, ShouldBeNil)
		}

		Convey("It should remain finite across a topology-scale trajectory", func() {
			allFinite := true

			for _, particle := range particles {
				allFinite = allFinite && finiteParticle(particle)
			}

			So(allFinite, ShouldBeTrue)
			So(diagnostics.DeltaUsed, ShouldBeGreaterThan, 0)
			So(diagnostics.PsiRMS, ShouldBeGreaterThan, 0)
			So(diagnostics.GuidanceRMS, ShouldBeGreaterThan, 0)
		})
	})
}

func TestDomainWave(t *testing.T) {
	Convey("Given the resident complex omega field", t, func() {
		domain, err := NewDomain(testDomainConfig())
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })
		particles := testParticles()
		_, err = domain.Step(particles)
		So(err, ShouldBeNil)
		wave, err := domain.Wave()
		So(err, ShouldBeNil)
		waveDial := make(geometry.PhaseDial, len(wave))

		for index, mode := range wave {
			waveDial[index] = complex(float64(mode.Real), float64(mode.Imaginary))
		}

		corpus, err := geometry.NewCorpus[string](4)
		So(err, ShouldBeNil)
		baseTime := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
		categories := []string{"zero", "quarter", "half", "three-quarter"}

		for index, category := range categories {
			So(corpus.Insert(geometry.CorpusEntry[string]{
				Dial:    waveDial.Rotate(float64(index) * math.Pi / 2),
				Outcome: category,
				At:      baseTime.Add(time.Duration(index) * time.Second),
			}), ShouldBeNil)
		}

		responses, err := corpus.ScanPhases(
			waveDial,
			[]float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2},
			1,
		)

		Convey("It should retain enough phase information for a complete dial scan", func() {
			So(err, ShouldBeNil)
			So(responses, ShouldHaveLength, len(categories))

			for index, category := range categories {
				So(responses[index][0].Outcome, ShouldEqual, category)
				So(responses[index][0].Similarity, ShouldAlmostEqual, 1)
			}
		})
	})
}

func TestDomainReading(t *testing.T) {
	Convey("Given a fluid domain that has not advanced", t, func() {
		domain, err := NewDomain(testDomainConfig())
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })

		Convey("It should reject observations without resident physical state", func() {
			_, err = domain.Reading()
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a coupled gas and wave step", t, func() {
		domain, err := NewDomain(testDomainConfig())
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })
		particles := testParticles()
		_, err = domain.Step(particles)
		So(err, ShouldBeNil)

		Convey("It should expose finite reductions of the resident fields", func() {
			reading, readErr := domain.Reading()
			So(readErr, ShouldBeNil)
			So(reading.IsFinite(), ShouldBeTrue)
			So(reading.CoherenceMag2, ShouldBeGreaterThan, 0)
			So(reading.GuidanceSpeed, ShouldBeGreaterThan, 0)
		})
	})
}

func TestDomainProjection(t *testing.T) {
	Convey("Given a fluid domain that has not advanced", t, func() {
		domain, err := NewDomain(testDomainConfig())
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })

		Convey("It should reject a projection without resident physical state", func() {
			_, err = domain.Projection()
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a coupled gas and wave step", t, func() {
		domain, err := NewDomain(testDomainConfig())
		So(err, ShouldBeNil)
		Reset(func() { So(domain.Close(), ShouldBeNil) })
		particles := testParticles()
		_, err = domain.Step(particles)
		So(err, ShouldBeNil)

		Convey("It should expose finite X-Z projections from the same state", func() {
			projection, projectionErr := domain.Projection()
			So(projectionErr, ShouldBeNil)
			expected := testDomainConfig().Grid.X * testDomainConfig().Grid.Z
			So(projection.Grid, ShouldResemble, testDomainConfig().Grid)
			So(projection.Density, ShouldHaveLength, expected)
			So(projection.Coherence, ShouldHaveLength, expected)
			So(projection.GuidanceX, ShouldHaveLength, expected)
			So(projection.GuidanceZ, ShouldHaveLength, expected)
			So(finiteValues(projection.Density), ShouldBeTrue)
			So(finiteValues(projection.Coherence), ShouldBeTrue)
			So(finiteValues(projection.GuidanceX), ShouldBeTrue)
			So(finiteValues(projection.GuidanceZ), ShouldBeTrue)
			So(maxValue(projection.Density), ShouldBeGreaterThan, 0)
			So(maxValue(projection.Coherence), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkDomainStep(b *testing.B) {
	marketConfig := DefaultConfig()
	fixtures := []struct {
		name      string
		config    Config
		particles []Particle
	}{
		{name: "two_particles", config: testDomainConfig(), particles: testParticles()},
		{
			name:      "market_population",
			config:    marketConfig,
			particles: benchmarkParticles(marketConfig.Grid),
		},
	}

	for _, fixture := range fixtures {
		b.Run(fixture.name, func(b *testing.B) {
			domain, err := NewDomain(fixture.config)

			if err != nil {
				b.Fatal(err)
			}

			defer domain.Close()
			b.ResetTimer()
			steps := 0

			for b.Loop() {
				steps++

				if _, err := domain.Step(fixture.particles); err != nil {
					b.Fatalf("step %d: %v", steps, err)
				}
			}
		})
	}
}

func BenchmarkDomainReading(b *testing.B) {
	config := DefaultConfig()
	domain, err := NewDomain(config)

	if err != nil {
		b.Fatal(err)
	}

	defer domain.Close()
	particles := benchmarkParticles(config.Grid)

	if _, err = domain.Step(particles); err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if _, err := domain.Reading(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDomainProjection(b *testing.B) {
	config := DefaultConfig()
	domain, err := NewDomain(config)

	if err != nil {
		b.Fatal(err)
	}

	defer domain.Close()
	particles := benchmarkParticles(config.Grid)

	if _, err = domain.Step(particles); err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if _, err := domain.Projection(); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkParticles(grid Grid) []Particle {
	count := grid.X * grid.Y
	particles := make([]Particle, count)

	for index := range particles {
		cellX := index % grid.X
		cellY := (index / grid.X) % grid.Y
		cellZ := (cellX + cellY) % grid.Z
		phase := 2 * math.Pi * float64(cellX) / float64(grid.X)

		particles[index] = Particle{
			Position: Vector{
				X: (float32(cellX) + 0.5) * grid.Spacing,
				Y: (float32(cellY) + 0.5) * grid.Spacing,
				Z: (float32(cellZ) + 0.5) * grid.Spacing,
			},
			Velocity: Vector{X: 0.05 * float32(math.Sin(phase))},
			Mass:     1,
			Heat:     0.4,
			Energy:   1,
			Phase:    float32(phase),
			Omega:    -4 + 8*float32(cellX)/float32(grid.X-1),
		}
	}

	return particles
}

func finiteParticle(particle Particle) bool {
	values := []float32{
		particle.Position.X,
		particle.Position.Y,
		particle.Position.Z,
		particle.Velocity.X,
		particle.Velocity.Y,
		particle.Velocity.Z,
		particle.Mass,
		particle.Heat,
		particle.Energy,
		particle.Phase,
		particle.Omega,
	}

	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}

	return true
}

func finiteValues(values []float32) bool {
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}

	return true
}

func maxValue(values []float32) float32 {
	var maximum float32

	for _, value := range values {
		maximum = max(maximum, value)
	}

	return maximum
}
