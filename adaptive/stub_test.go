package adaptive

import "github.com/theapemachine/nomagique/core"

type blankNumber struct{}

func (blankNumber) Observe(inputs ...core.Number) core.Float64 {
	return 0
}

func (blankNumber) Reset() error {
	return nil
}
