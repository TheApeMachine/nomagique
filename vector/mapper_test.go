package vector

import (
	"testing"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func mapperFrame() *datura.Artifact {
	frame := datura.Acquire("mapper-frame", datura.APPJSON)
	frame.Poke("features", "root")
	frame.Poke([]string{"rvol", "precursor", "compression"}, "inputs")
	frame.Merge("features", []float64{4.0, 2.0, 0.5})
	return frame
}

func TestMapperRenameCombineInvert(testingTB *testing.T) {
	config := datura.Acquire("mapper", datura.APPJSON).WithAttributes(datura.Map[any]{
		"mappings": []datura.Map[any]{
			{"inputs": []string{"rvol"}, "outputKey": "lift"},
			{"inputs": []string{"rvol", "precursor"}, "op": "product", "outputKey": "energy"},
			{"inputs": []string{"rvol", "precursor"}, "op": "ratio", "outputKey": "balance"},
			{"inputs": []string{"rvol", "compression"}, "op": "sum", "inverts": []string{"compression"}, "outputKey": "net"},
		},
	})

	frame := mapperFrame()
	defer frame.Release()

	if err := nomagique.RoundTripArtifact(frame, NewMapper(config)); err != nil {
		testingTB.Fatalf("flipflop: %v", err)
	}

	checks := map[string]float64{
		"lift":    4.0, // rename
		"energy":  8.0, // 4 * 2
		"balance": 2.0, // 4 / 2
		"net":     3.5, // 4 + (-0.5)
	}

	for key, want := range checks {
		got := datura.Peek[float64](frame, "output", key)
		if got != want {
			testingTB.Errorf("output.%s = %v, want %v", key, got, want)
		}
	}

	// original features must survive for downstream stages.
	if got := datura.Peek[[]float64](frame, "features"); len(got) != 3 {
		testingTB.Errorf("features dropped: %v", got)
	}
}

func TestMapperEMATransformAccumulates(testingTB *testing.T) {
	config := datura.Acquire("mapper-ema", datura.APPJSON).WithAttributes(datura.Map[any]{
		"mappings": []datura.Map[any]{
			{"inputs": []string{"rvol"}, "transform": "ema", "outputKey": "rvolEMA"},
		},
	})

	mapper := NewMapper(config)
	var last float64

	for _, value := range []float64{1, 2, 3, 4, 5} {
		frame := datura.Acquire("mapper-ema-frame", datura.APPJSON)
		frame.Poke("features", "root")
		frame.Poke([]string{"rvol"}, "inputs")
		frame.Merge("features", []float64{value})

		if err := nomagique.RoundTripArtifact(frame, mapper); err != nil {
			testingTB.Fatalf("flipflop: %v", err)
		}

		last = datura.Peek[float64](frame, "output", "rvolEMA")
		frame.Release()
	}

	if last <= 0 {
		testingTB.Errorf("ema output not produced: %v", last)
	}
}

func BenchmarkMapper_EMATransform(b *testing.B) {
	config := datura.Acquire("mapper-ema", datura.APPJSON).WithAttributes(datura.Map[any]{
		"mappings": []datura.Map[any]{
			{"inputs": []string{"rvol"}, "transform": "ema", "outputKey": "rvolEMA"},
		},
	})
	mapper := NewMapper(config)

	b.ReportAllocs()

	for b.Loop() {
		frame := datura.Acquire("mapper-ema-frame", datura.APPJSON)
		frame.Poke("features", "root")
		frame.Poke([]string{"rvol"}, "inputs")
		frame.Merge("features", []float64{4.0})

		if err := nomagique.RoundTripArtifact(frame, mapper); err != nil {
			b.Fatal(err)
		}

		frame.Release()
	}
}
