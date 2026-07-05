package vector

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
NamedValue carries a scalar feature with its schema key.
*/
type NamedValue struct {
	Name  string
	Value float64
}

/*
FeatureRow carries one market observation as named scalar fields.
*/
type FeatureRow struct {
	Fields []NamedValue
}

/*
FeatureInput carries a typed frame into the feature extractor.
*/
type FeatureInput struct {
	Channel string
	Rows    []FeatureRow
	Row     FeatureRow
}

/*
FeatureVector carries extracted feature columns and derived named outputs.
*/
type FeatureVector struct {
	Features     []float64
	Inputs       []string
	SourceRoot   string
	SourceInputs []string
	Values       []NamedValue
}

/*
NewFeatureRow builds one row from named scalar values.
*/
func NewFeatureRow(values ...NamedValue) FeatureRow {
	return FeatureRow{
		Fields: append([]NamedValue(nil), values...),
	}
}

/*
Value returns the named scalar from the row.
*/
func (row FeatureRow) Value(name string) (float64, bool) {
	for _, field := range row.Fields {
		if field.Name != name {
			continue
		}

		return field.Value, true
	}

	return 0, false
}

/*
Value resolves a scalar by schema key from features or derived values.
*/
func (vector FeatureVector) Value(name string) (float64, bool) {
	for index, input := range vector.Inputs {
		if input != name || index >= len(vector.Features) {
			continue
		}

		return vector.Features[index], true
	}

	for _, value := range vector.Values {
		if value.Name != name {
			continue
		}

		return value.Value, true
	}

	return 0, false
}

/*
WithValue returns a copy with one named output inserted or replaced.
*/
func (vector FeatureVector) WithValue(value NamedValue) FeatureVector {
	next := FeatureVector{
		Features:     append([]float64(nil), vector.Features...),
		Inputs:       append([]string(nil), vector.Inputs...),
		SourceRoot:   vector.SourceRoot,
		SourceInputs: append([]string(nil), vector.SourceInputs...),
		Values:       append([]NamedValue(nil), vector.Values...),
	}

	for index, existing := range next.Values {
		if existing.Name != value.Name {
			continue
		}

		next.Values[index] = value
		return next
	}

	next.Values = append(next.Values, value)

	return next
}

func finiteVector(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return errnie.Error(errnie.Err(
			errnie.Validation,
			name+": value must be finite",
			nil,
		))
	}

	return nil
}
