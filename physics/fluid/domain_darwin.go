//go:build darwin && cgo

//go:generate go run ../manifold/metallibgen

package fluid

/*
#cgo CFLAGS: -fobjc-arc -I${SRCDIR}
#cgo CXXFLAGS: -x objective-c++ -std=c++17 -fobjc-arc -I${SRCDIR}
#cgo LDFLAGS: -framework Metal -framework Foundation -framework CoreFoundation
#include "bridge.h"
*/
import "C"

import (
	_ "embed"
	"fmt"
	"runtime"
	"unsafe"
)

//go:embed kernels.metallib
var fluidMetallib []byte

type domainHandle unsafe.Pointer

/*
NewDomain loads the generated Sensorium Metal library and creates one resident
coupled domain. Loading is deliberately fail-fast; there is no CPU backend.
*/
func NewDomain(config Config) (*Domain, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	bridgeConfig := C.FluidConfig{
		grid_x:    C.uint32_t(config.Grid.X),
		grid_y:    C.uint32_t(config.Grid.Y),
		grid_z:    C.uint32_t(config.Grid.Z),
		spacing:   C.float(config.Grid.Spacing),
		max_delta: C.float(config.MaxDelta),
		omega_min: C.float(config.OmegaMin),
		omega_max: C.float(config.OmegaMax),
	}
	errorBuffer := make([]byte, 4096)
	handle := C.fluid_domain_new(
		&bridgeConfig,
		unsafe.Pointer(&fluidMetallib[0]),
		C.size_t(len(fluidMetallib)),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)
	runtime.KeepAlive(fluidMetallib)

	if handle == nil {
		return nil, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return &Domain{
		handle: domainHandle(handle),
		config: config,
	}, nil
}

/*
Step advances gas thermodynamics, omega-wave coupling, and spatial pilot-wave
transport in Sensorium order. Particle state is updated in place; each call
supplies the complete current population while resident gas and wave fields
survive changes in population size.
*/
func (domain *Domain) Step(particles []Particle) (Diagnostics, error) {
	if domain == nil || domain.handle == nil {
		return Diagnostics{}, fmt.Errorf("fluid: domain is closed")
	}

	if err := validateParticles(particles, domain.config); err != nil {
		return Diagnostics{}, err
	}

	bridgeParticles := make([]C.FluidParticle, len(particles))

	for index, particle := range particles {
		bridgeParticles[index] = C.FluidParticle{
			position_x: C.float(particle.Position.X),
			position_y: C.float(particle.Position.Y),
			position_z: C.float(particle.Position.Z),
			velocity_x: C.float(particle.Velocity.X),
			velocity_y: C.float(particle.Velocity.Y),
			velocity_z: C.float(particle.Velocity.Z),
			mass:       C.float(particle.Mass),
			heat:       C.float(particle.Heat),
			energy:     C.float(particle.Energy),
			phase:      C.float(particle.Phase),
			omega:      C.float(particle.Omega),
		}
	}

	var bridgeDiagnostics C.FluidDiagnostics
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_step(
		unsafe.Pointer(domain.handle),
		&bridgeParticles[0],
		C.uint32_t(len(bridgeParticles)),
		&bridgeDiagnostics,
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return Diagnostics{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	for index, particle := range bridgeParticles {
		particles[index] = Particle{
			Position: Vector{
				X: float32(particle.position_x),
				Y: float32(particle.position_y),
				Z: float32(particle.position_z),
			},
			Velocity: Vector{
				X: float32(particle.velocity_x),
				Y: float32(particle.velocity_y),
				Z: float32(particle.velocity_z),
			},
			Mass:   float32(particle.mass),
			Heat:   float32(particle.heat),
			Energy: float32(particle.energy),
			Phase:  float32(particle.phase),
			Omega:  float32(particle.omega),
		}
	}

	return Diagnostics{
		CFLRate:      float32(bridgeDiagnostics.cfl_rate),
		DeltaAdv:     float32(bridgeDiagnostics.delta_adv),
		DeltaDiffuse: float32(bridgeDiagnostics.delta_diffuse),
		DeltaDerived: float32(bridgeDiagnostics.delta_derived),
		DeltaUsed:    float32(bridgeDiagnostics.delta_used),
		Halvings:     uint32(bridgeDiagnostics.halvings),
		PsiRMS:       float32(bridgeDiagnostics.psi_rms),
		PsiDeltaRMS:  float32(bridgeDiagnostics.psi_delta_rms),
		GuidanceRMS:  float32(bridgeDiagnostics.guidance_rms),
	}, nil
}

/*
Wave reads the resident omega lattice without advancing it.
*/
func (domain *Domain) Wave() ([]WaveMode, error) {
	if domain == nil || domain.handle == nil {
		return nil, fmt.Errorf("fluid: domain is closed")
	}

	modeCount := uint32(C.fluid_domain_mode_count(unsafe.Pointer(domain.handle)))
	modes := make([]C.FluidWaveMode, modeCount)
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_wave(
		unsafe.Pointer(domain.handle),
		&modes[0],
		C.uint32_t(modeCount),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return nil, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	wave := make([]WaveMode, modeCount)

	for index, mode := range modes {
		wave[index] = WaveMode{
			Omega:     float32(mode.omega),
			Real:      float32(mode.real),
			Imaginary: float32(mode.imaginary),
			Linewidth: float32(mode.linewidth),
		}
	}

	return wave, nil
}

/*
Reading reads the post-step gas and wave observables without advancing the
resident domain.
*/
func (domain *Domain) Reading() (Reading, error) {
	if domain == nil || domain.handle == nil {
		return Reading{}, fmt.Errorf("fluid: domain is closed")
	}

	var bridgeReading C.FluidReading
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read(
		unsafe.Pointer(domain.handle),
		&bridgeReading,
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)

	if result == 0 {
		return Reading{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return Reading{
		PressureGradX:    float64(bridgeReading.pressure_grad_x),
		PressureGradY:    float64(bridgeReading.pressure_grad_y),
		PressureGradZ:    float64(bridgeReading.pressure_grad_z),
		PressureGradNorm: float64(bridgeReading.pressure_grad_norm),
		Divergence:       float64(bridgeReading.divergence),
		CoherenceMag2:    float64(bridgeReading.coherence_mag2),
		GuidanceSpeed:    float64(bridgeReading.guidance_speed),
		ViscosityProxy:   float64(bridgeReading.viscosity_proxy),
	}, nil
}

/*
Projection reads the X-Z maximum projection used for field inspection without
advancing the resident domain.
*/
func (domain *Domain) Projection() (Projection, error) {
	if domain == nil || domain.handle == nil {
		return Projection{}, fmt.Errorf("fluid: domain is closed")
	}

	length := domain.config.Grid.X * domain.config.Grid.Z
	projection := Projection{
		Grid:      domain.config.Grid,
		Density:   make([]float32, length),
		Coherence: make([]float32, length),
		GuidanceX: make([]float32, length),
		GuidanceZ: make([]float32, length),
	}
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_projection(
		unsafe.Pointer(domain.handle),
		(*C.float)(unsafe.Pointer(&projection.Density[0])),
		(*C.float)(unsafe.Pointer(&projection.Coherence[0])),
		(*C.float)(unsafe.Pointer(&projection.GuidanceX[0])),
		(*C.float)(unsafe.Pointer(&projection.GuidanceZ[0])),
		C.uint32_t(length),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)
	runtime.KeepAlive(projection)

	if result == 0 {
		return Projection{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return projection, nil
}

/*
Close releases the resident Metal library, pipelines, and buffers.
*/
func (domain *Domain) Close() error {
	if domain == nil || domain.handle == nil {
		return nil
	}

	C.fluid_domain_free(unsafe.Pointer(domain.handle))
	domain.handle = nil
	return nil
}

func cString(buffer []byte) string {
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index])
		}
	}

	return string(buffer)
}
