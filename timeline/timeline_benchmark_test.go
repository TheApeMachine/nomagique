package timeline

import (
	"testing"
	"time"
)

func BenchmarkTimelineNewSorted(b *testing.B) {
	start := time.Unix(0, 0)
	times := make([]time.Time, 128)

	for index := range times {
		times[index] = start.Add(time.Duration(index) * time.Millisecond)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = New(times)
	}
}

func BenchmarkTimelineGaps(b *testing.B) {
	start := time.Unix(0, 0)
	times := make([]time.Time, 128)

	for index := range times {
		times[index] = start.Add(time.Duration(index) * time.Millisecond)
	}

	eventTimeline := New(times)

	b.ReportAllocs()

	for b.Loop() {
		_ = eventTimeline.Gaps()
	}
}
