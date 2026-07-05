package equation

import (
	"sort"

	"github.com/bytedance/sonic"
	"github.com/theapemachine/errnie"
)

/*
FeatureFrame carries a semantic feature vector with its ordered schema.
*/
type FeatureFrame struct {
	Inputs   []string  `json:"inputs"`
	Features []float64 `json:"features"`
	Root     string    `json:"root"`
}

/*
NewFeatureFrame copies feature schema and values into a typed frame.
*/
func NewFeatureFrame(inputs []string, features []float64) FeatureFrame {
	return FeatureFrame{
		Inputs:   append([]string(nil), inputs...),
		Features: append([]float64(nil), features...),
		Root:     "features",
	}
}

/*
FeatureSlice reads a contiguous segment from the feature vector.
*/
func (frame FeatureFrame) FeatureSlice(offset, count int) ([]float64, error) {
	if count < 0 || offset < 0 || offset+count > len(frame.Features) {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"equation: feature slice out of range",
			nil,
		))
	}

	return append([]float64(nil), frame.Features[offset:offset+count]...), nil
}

/*
FeatureFields reads named scalars in order; missing keys return validation errors.
*/
func (frame FeatureFrame) FeatureFields(keys []string) ([]float64, error) {
	values := make([]float64, len(keys))

	for index, key := range keys {
		found := false

		for column, input := range frame.Inputs {
			if input != key || column >= len(frame.Features) {
				continue
			}

			values[index] = frame.Features[column]
			found = true
			break
		}

		if !found {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"feature-column: key not found",
				nil,
			))
		}
	}

	return values, nil
}

/*
MarshalFeaturesPayload encodes a feature vector as JSON payload bytes.
Prefer MarshalFeatureSchema with explicit input keys for new tests.
*/
func MarshalFeaturesPayload(samples []float64) []byte {
	return MarshalFeatureSchema(nil, samples)
}

/*
MarshalFeatureSchema encodes semantic features for typed boundary tests.
*/
func MarshalFeatureSchema(inputs []string, values []float64) []byte {
	frame := NewFeatureFrame(inputs, values)

	if len(frame.Inputs) == 0 {
		frame.Inputs = nil
	}

	payload := map[string]any{
		"features": frame.Features,
		"inputs":   frame.Inputs,
		"root":     frame.Root,
	}

	return errnie.Does(func() ([]byte, error) {
		return sonic.Marshal(payload)
	}).Or(func(err error) {
		errnie.Error(errnie.Err(errnie.IO, "equation: marshal feature schema payload", err))
	}).Value()
}

func outputKeys(fields map[string]float64) []string {
	keys := make([]string, 0, len(fields)+1)

	for key := range fields {
		keys = append(keys, key)
	}

	if _, hasStrength := fields["strength"]; !hasStrength {
		if _, hasValue := fields["value"]; hasValue {
			keys = append(keys, "strength")
		}
	}

	sort.Strings(keys)

	return keys
}
