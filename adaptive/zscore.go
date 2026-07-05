package adaptive

import "math"

/*
ZScore tracks adaptive scale for a normalized surprise score.
*/
type ZScore struct {
	mean     float64
	variance float64
	prev     float64
	min      float64
	max      float64
	count    int
}

/*
ZScoreOutput reports adaptive surprise.
*/
type ZScoreOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewZScore returns a typed adaptive z-score tracker.
*/
func NewZScore() *ZScore {
	return &ZScore{}
}

/*
Measure adds one sample and returns the adaptive z-score.
*/
func (surprise *ZScore) Measure(sample float64) (ZScoreOutput, error) {
	return surprise.MeasureAnchored(sample, 0, false)
}

/*
MeasureAnchored adds one sample and optionally scores relative to an external anchor.
*/
func (surprise *ZScore) MeasureAnchored(sample float64, anchor float64, hasAnchor bool) (ZScoreOutput, error) {
	if err := finiteAdaptive("zscore", sample); err != nil {
		return ZScoreOutput{}, err
	}

	if hasAnchor {
		if err := finiteAdaptive("zscore", anchor); err != nil {
			return ZScoreOutput{}, err
		}
	}

	if surprise.count == 0 {
		surprise.mean = sample
		surprise.variance = 0
		surprise.prev = sample
		surprise.min = sample
		surprise.max = sample
		surprise.count = 1

		return ZScoreOutput{
			Ready: false,
			Count: surprise.count,
		}, nil
	}

	surprise.min = math.Min(surprise.min, sample)
	surprise.max = math.Max(surprise.max, sample)
	surprise.count++
	span := surprise.max - surprise.min

	if span == 0 {
		surprise.prev = sample

		return ZScoreOutput{
			Ready: false,
			Count: surprise.count,
		}, nil
	}

	rate := math.Abs(sample-surprise.prev) / span
	level := surprise.mean

	if hasAnchor {
		level = anchor
	}

	deviation := sample - level

	if !hasAnchor {
		surprise.mean += rate * (sample - surprise.mean)
	}

	surprise.variance += rate * (deviation*deviation - surprise.variance)
	surprise.prev = sample

	if surprise.variance <= 0 {
		return ZScoreOutput{
			Ready: false,
			Count: surprise.count,
		}, nil
	}

	value := deviation / math.Sqrt(surprise.variance)

	if err := finiteAdaptive("zscore", value); err != nil {
		return ZScoreOutput{}, err
	}

	return ZScoreOutput{
		Value: value,
		Ready: true,
		Count: surprise.count,
	}, nil
}
