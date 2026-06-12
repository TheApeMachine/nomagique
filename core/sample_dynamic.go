package core

/*
SampleDynamic observes a raw float64 sample without interface input vectors.
*/
type SampleDynamic interface {
	Number
	ObserveSample(float64) float64
}
