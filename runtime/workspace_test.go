package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/theapemachine/nomagique/data"
)

type marker struct {
	*System
}

func (marker *marker) Step(value *data.Value) *data.Value {
	value.Append([]float64{0.25}, "liquidity", "aggregate", "market_friction")
	return value
}

func TestStepSettlesAfterPipelineWrites(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	node := &marker{System: NewSystem(ctx, "marker")}
	workspace := NewWorkspace(ctx, "ws", []Ring{{
		Group: "signal",
		Nodes: []Node{node},
	}})
	if workspace == nil {
		t.Fatal("workspace is nil")
	}
	workspace.Transition(READY)

	settled := make(chan []float64, 1)
	workspace.OnSettled(func(value *data.Value) {
		var got []float64
		value.Lookup(&got, "liquidity/aggregate/market_friction")
		settled <- got
	})

	workspace.Step(data.NewValue("workspace", "tick", "aggregate"))

	select {
	case got := <-settled:
		if len(got) != 1 || got[0] != 0.25 {
			t.Fatalf("pipeline layer missing after settle: %#v", got)
		}
	case <-ctx.Done():
		t.Fatal("pipeline did not settle")
	}
}
