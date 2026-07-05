package algorithm

import (
	"fmt"
	"math"

	"github.com/theapemachine/errnie"
)

const pearlDefaultMinHistory = 5

/*
PearlInput is one keyed numeric observation for causal evaluation.
*/
type PearlInput struct {
	Key          string
	Row          []float64
	Inverted     bool
	Contagion    float64
	Condition    float64
	Intervention float64
}

/*
PearlSample retains aligned numeric causal rows by key.
*/
type PearlSample struct {
	minHistory int
	history    int
	windows    map[string]*pearlWindow
}

/*
PearlSampleOutput is the current causal row and retained table.
*/
type PearlSampleOutput struct {
	Key  string
	Row  []float64
	Rows [][]float64
}

type pearlWindow struct {
	rows [][]float64
}

/*
NewPearlSample returns a keyed numeric causal sampler.
*/
func NewPearlSample(configs ...PearlConfig) *PearlSample {
	config := PearlConfig{}

	if len(configs) > 0 {
		config = configs[0]
	}

	minHistory := config.MinHistory

	if minHistory <= 0 {
		minHistory = pearlDefaultMinHistory
	}

	history := config.History

	if history < minHistory {
		history = minHistory
	}

	return &PearlSample{
		minHistory: minHistory,
		history:    history,
		windows:    map[string]*pearlWindow{},
	}
}

/*
Measure observes one keyed numeric causal row.
*/
func (pearlSample *PearlSample) Measure(
	input PearlInput,
) (PearlSampleOutput, bool, error) {
	if input.Key == "" {
		return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"pearl-sample: key required",
			nil,
		))
	}

	if len(input.Row) == 0 {
		return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"pearl-sample: row required",
			nil,
		))
	}

	row := make([]float64, 0, len(input.Row))

	for _, value := range input.Row {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
				errnie.Validation,
				"pearl-sample: row contains non-finite value",
				nil,
			))
		}

		row = append(row, value)
	}

	window := pearlSample.window(input.Key)

	if len(window.rows) > 0 && len(window.rows[0]) != len(row) {
		return PearlSampleOutput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf(
				"pearl-sample: row width %d differs from retained width %d",
				len(row),
				len(window.rows[0]),
			),
			nil,
		))
	}

	return pearlSample.append(input.Key, row, window),
		len(window.rows) >= pearlSample.minHistory,
		nil
}

func (pearlSample *PearlSample) append(
	key string,
	row []float64,
	window *pearlWindow,
) PearlSampleOutput {
	window.rows = append(window.rows, append([]float64(nil), row...))

	if len(window.rows) > pearlSample.history {
		window.rows = window.rows[len(window.rows)-pearlSample.history:]
	}

	rows := make([][]float64, 0, len(window.rows))

	for _, retained := range window.rows {
		rows = append(rows, append([]float64(nil), retained...))
	}

	return PearlSampleOutput{
		Key:  key,
		Row:  append([]float64(nil), row...),
		Rows: rows,
	}
}

func (pearlSample *PearlSample) window(key string) *pearlWindow {
	window, ok := pearlSample.windows[key]

	if ok {
		return window
	}

	window = &pearlWindow{}
	pearlSample.windows[key] = window

	return window
}
