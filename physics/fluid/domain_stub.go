//go:build !darwin || !cgo

package fluid

import "fmt"

type domainHandle uintptr

/*
NewDomain reports that the Metal-only implementation is unavailable on this
platform. It intentionally does not substitute a CPU backend.
*/
func NewDomain(config Config) (*Domain, error) {
	return nil, fmt.Errorf("fluid: Metal domain requires darwin with cgo")
}

/*
Step reports that no Metal domain exists on this platform.
*/
func (domain *Domain) Step(particles []Particle) (Diagnostics, error) {
	return Diagnostics{}, fmt.Errorf("fluid: Metal domain requires darwin with cgo")
}

/*
Wave reports that no Metal domain exists on this platform.
*/
func (domain *Domain) Wave() ([]WaveMode, error) {
	return nil, fmt.Errorf("fluid: Metal domain requires darwin with cgo")
}

/*
Reading reports that no Metal domain exists on this platform.
*/
func (domain *Domain) Reading() (Reading, error) {
	return Reading{}, fmt.Errorf("fluid: Metal domain requires darwin with cgo")
}

/*
Projection reports that no Metal domain exists on this platform.
*/
func (domain *Domain) Projection() (Projection, error) {
	return Projection{}, fmt.Errorf("fluid: Metal domain requires darwin with cgo")
}

/*
Close is a no-op because a non-Metal domain cannot be constructed.
*/
func (domain *Domain) Close() error {
	return nil
}
