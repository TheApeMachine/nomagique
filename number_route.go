package nomagique

import (
	"sync"
	"unsafe"

	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
)

/*
stageObserveKey identifies a retained stage set by the dynamics instance pointers.
*/
type stageObserveKey struct {
	first  uintptr
	second uintptr
}

type observeBinder struct {
	stages []core.Number
	apply  func(float64) float64
}

var (
	stageObserve sync.Map
	boundObserve sync.Map
)

func registerObserveBinder(
	boundary core.Float64, stages []core.Number,
) {
	apply := adaptive.BindObserveSample(stages)

	if apply == nil {
		return
	}

	key, keyed := stageObserveKeyFrom(stages)

	if keyed {
		stageObserve.Store(key, apply)
	}

	copied := make([]core.Number, len(stages))
	copy(copied, stages)

	boundObserve.Store(boundary, &observeBinder{
		stages: copied,
		apply:  apply,
	})
}

func stageObserveApply(stages []core.Number) (func(float64) float64, bool) {
	key, keyed := stageObserveKeyFrom(stages)

	if !keyed {
		return nil, false
	}

	value, loaded := stageObserve.Load(key)

	if !loaded {
		return nil, false
	}

	apply, isApply := value.(func(float64) float64)

	if !isApply {
		return nil, false
	}

	return apply, true
}

func boundObserveApply(
	boundary core.Float64, stages []core.Number,
) (func(float64) float64, bool) {
	value, loaded := boundObserve.Load(boundary)

	if !loaded {
		return nil, false
	}

	binder, isBinder := value.(*observeBinder)

	if !isBinder {
		return nil, false
	}

	if !stagesMatch(binder.stages, stages) {
		return nil, false
	}

	return binder.apply, true
}

func stageObserveKeyFrom(stages []core.Number) (stageObserveKey, bool) {
	switch len(stages) {
	case 1:
		first, ok := stagePointer(stages[0])

		return stageObserveKey{first: first}, ok
	case 2:
		first, okFirst := stagePointer(stages[0])
		second, okSecond := stagePointer(stages[1])

		return stageObserveKey{first: first, second: second}, okFirst && okSecond
	default:
		return stageObserveKey{}, false
	}
}

func stagePointer(stage core.Number) (uintptr, bool) {
	words := (*[2]uintptr)(unsafe.Pointer(&stage))
	data := words[1]

	return data, data != 0
}

func stagesMatch(registered []core.Number, passed []core.Number) bool {
	if len(registered) != len(passed) {
		return false
	}

	for index := range registered {
		if registered[index] != passed[index] {
			return false
		}
	}

	return true
}
