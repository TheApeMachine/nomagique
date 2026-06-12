package nomagique

import (
	"sync"

	"github.com/theapemachine/nomagique/core"
)

var stageSlicePool = sync.Pool{
	New: func() any {
		return make([]core.Number, 0, 8)
	},
}
