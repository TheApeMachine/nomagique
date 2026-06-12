package decay

import (
	"testing"
	"time"

	"github.com/theapemachine/nomagique/timeline"
)

func BenchmarkIntensityAt(b *testing.B) {
	start := time.Unix(0, 0)
	buyTimes := make([]time.Time, 64)
	sellTimes := make([]time.Time, 64)

	for index := range buyTimes {
		buyTimes[index] = start.Add(time.Duration(index*2) * time.Millisecond)
		sellTimes[index] = start.Add(time.Duration(index*2+1) * time.Millisecond)
	}

	buyEvents := timeline.New(buyTimes)
	sellEvents := timeline.New(sellTimes)
	at := start.Add(200 * time.Millisecond)

	b.ReportAllocs()

	for b.Loop() {
		_ = IntensityAt(buyEvents, sellEvents, at, 1, 0.2, 0.2, 1)
	}
}
