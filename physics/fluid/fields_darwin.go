//go:build darwin && cgo

package fluid

/*
#cgo CFLAGS: -fobjc-arc -I${SRCDIR}
#cgo CXXFLAGS: -x objective-c++ -std=c++17 -fobjc-arc -I${SRCDIR}
#cgo LDFLAGS: -framework Metal -framework Foundation -framework CoreFoundation -framework Accelerate
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

/*
Fields copies the complete post-step gas and spatial wave lattices without
advancing the resident Metal domain.
*/
func (domain *Domain) Fields() (Fields, error) {
	if domain == nil || domain.handle == nil {
		return Fields{}, fmt.Errorf("fluid: domain is closed")
	}

	cells := domain.config.Grid.X * domain.config.Grid.Y * domain.config.Grid.Z
	fields := Fields{
		Grid:           domain.config.Grid,
		Density:        make([]float32, cells),
		Momentum:       make([]float32, cells*3),
		InternalEnergy: make([]float32, cells),
		WaveReal:       make([]float32, cells),
		WaveImaginary:  make([]float32, cells),
	}
	errorBuffer := make([]byte, 4096)
	result := C.fluid_domain_read_fields(
		unsafe.Pointer(domain.handle),
		(*C.float)(unsafe.Pointer(&fields.Density[0])),
		(*C.float)(unsafe.Pointer(&fields.Momentum[0])),
		(*C.float)(unsafe.Pointer(&fields.InternalEnergy[0])),
		(*C.float)(unsafe.Pointer(&fields.WaveReal[0])),
		(*C.float)(unsafe.Pointer(&fields.WaveImaginary[0])),
		C.uint32_t(cells),
		(*C.char)(unsafe.Pointer(&errorBuffer[0])),
		C.int(len(errorBuffer)),
	)
	runtime.KeepAlive(fields)

	if result == 0 {
		return Fields{}, fmt.Errorf("fluid: %s", cString(errorBuffer))
	}

	return fields, nil
}
