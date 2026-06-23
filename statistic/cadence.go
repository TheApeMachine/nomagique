package statistic

import (
	"fmt"
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

type Observation struct {
	Value float64
	At    time.Time
}

func SeriesKey(config, state *datura.Artifact, stageKey string) string {
	memberKey := ConfigString(config, state, "memberKey")
	sourceRoot := SourceRoot(config, state)
	member := wireString(state, sourceRoot, memberKey)

	if member == "" {
		member = firstSourceString(state, sourceRoot, SourceInputs(config, state))
	}

	if member == "" {
		member, _ = state.Scope()
	}

	if member == "" {
		return stageKey
	}

	return stageKey + "/" + member
}

func EventTime(config, state *datura.Artifact) (time.Time, error) {
	timeKey := ConfigString(config, state, "timeKey")
	sourceRoot := SourceRoot(config, state)
	raw := wireString(state, sourceRoot, timeKey)

	if raw != "" {
		observed, err := time.Parse(time.RFC3339Nano, raw)

		if err != nil {
			return time.Time{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"cadence: invalid event timestamp",
				err,
			))
		}

		return observed, nil
	}

	if observed, ok := firstSourceTime(state, sourceRoot, SourceInputs(config, state)); ok {
		return observed, nil
	}

	if timestamp := state.Timestamp(); timestamp > 0 {
		return time.Unix(0, timestamp), nil
	}

	return time.Time{}, errnie.Error(errnie.Err(
		errnie.Validation,
		"cadence: event timestamp required",
		nil,
	))
}

func SourceRoot(config, state *datura.Artifact) string {
	if configured := ConfigString(config, state, "eventRoot"); configured != "" {
		return configured
	}

	if root := datura.Peek[string](state, "sourceRoot"); root != "" {
		return root
	}

	if root := ConfigString(config, state, "sourceRoot"); root != "" {
		return root
	}

	root := ConfigString(config, state, "root")

	if root == "features" || root == "output" {
		return ""
	}

	return root
}

func SourceInputs(config, state *datura.Artifact) []string {
	if inputs := datura.Peek[[]string](state, "sourceInputs"); len(inputs) > 0 {
		return inputs
	}

	if inputs := datura.Peek[[]string](state, "featureInputs"); len(inputs) > 0 {
		return inputs
	}

	if SourceRoot(config, state) == "" {
		return nil
	}

	return ConfigStringSlice(config, state, "inputs")
}

func wireString(state *datura.Artifact, rootKey, wireKey string) string {
	if wireKey == "" {
		return ""
	}

	if rootKey != "" {
		if KeyPresent(state, rootKey, wireKey) {
			return datura.Peek[string](state, rootKey, wireKey)
		}

		if KeyPresent(state, rootKey, 0, wireKey) {
			return datura.Peek[string](state, rootKey, 0, wireKey)
		}
	}

	if KeyPresent(state, wireKey) {
		return datura.Peek[string](state, wireKey)
	}

	return ""
}

func firstSourceString(state *datura.Artifact, rootKey string, inputs []string) string {
	for _, input := range inputs {
		value := wireString(state, rootKey, input)

		if value == "" {
			continue
		}

		if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
			continue
		}

		return value
	}

	return ""
}

func firstSourceTime(state *datura.Artifact, rootKey string, inputs []string) (time.Time, bool) {
	for _, input := range inputs {
		value := wireString(state, rootKey, input)

		if value == "" {
			continue
		}

		observed, err := time.Parse(time.RFC3339Nano, value)

		if err != nil {
			continue
		}

		return observed, true
	}

	return time.Time{}, false
}

func AppendObservation(
	history []Observation,
	value float64,
	observed time.Time,
) ([]Observation, error) {
	if len(history) > 0 && observed.Before(history[len(history)-1].At) {
		return history, errnie.Error(errnie.Err(
			errnie.Validation,
			"cadence: event timestamp must not regress",
			nil,
		))
	}

	return append(history, Observation{Value: value, At: observed}), nil
}

func ObservationValues(history []Observation) []float64 {
	values := make([]float64, len(history))

	for index, observation := range history {
		values[index] = observation.Value
	}

	return values
}

func RollingObservationWindows(
	history []Observation,
	shortHint int,
	longHint int,
) (shortWindow int, longWindow int, err error) {
	if len(history) == 0 {
		return 0, 0, fmt.Errorf("statistic: rolling windows require observations")
	}

	if shortHint > 0 {
		shortWindow = shortHint
	}

	if longHint > 0 {
		longWindow = longHint
	}

	if shortWindow > 0 && longWindow > 0 {
		return shortWindow, longWindow, nil
	}

	count := len(history)

	if longWindow <= 0 {
		longWindow = count
	}

	if shortWindow <= 0 {
		shortWindow = cadenceWindow(history)
	}

	if shortWindow < 1 {
		shortWindow = 1
	}

	if longWindow < 1 {
		longWindow = 1
	}

	if shortWindow > longWindow {
		shortWindow = longWindow
	}

	return shortWindow, longWindow, nil
}

func ReturnLagObservations(
	history []Observation,
	lagHint int,
	longHint int,
) (int, error) {
	if lagHint > 0 {
		return lagHint, nil
	}

	if len(history) <= 1 {
		return 1, nil
	}

	shortWindow, longWindow, err := RollingObservationWindows(history, 0, longHint)

	if err != nil {
		return 0, err
	}

	lag := shortWindow

	if lag >= longWindow {
		lag = longWindow - 1
	}

	if lag < 1 {
		lag = 1
	}

	return lag, nil
}

func TrimObservations(history []Observation, keep int) []Observation {
	if keep <= 0 || len(history) <= keep {
		return history
	}

	return history[len(history)-keep:]
}

func cadenceWindow(history []Observation) int {
	count := len(history)

	if count <= 1 {
		return 1
	}

	span := history[count-1].At.Sub(history[0].At)

	if span <= 0 {
		return count
	}

	cadence, ok := medianCadence(history)

	if !ok || cadence <= 0 {
		return count
	}

	horizon := time.Duration(math.Sqrt(float64(span) * float64(cadence)))
	cutoff := history[count-1].At.Add(-horizon)
	window := 0

	for index := count - 1; index >= 0; index-- {
		if history[index].At.Before(cutoff) {
			break
		}

		window++
	}

	if window < 1 {
		return 1
	}

	return window
}

func medianCadence(history []Observation) (time.Duration, bool) {
	deltas := make([]float64, 0, len(history)-1)

	for index := 1; index < len(history); index++ {
		delta := history[index].At.Sub(history[index-1].At)

		if delta > 0 {
			deltas = append(deltas, float64(delta))
		}
	}

	median, ok := MedianOf(deltas)

	if !ok {
		return 0, false
	}

	return time.Duration(median), true
}
