package probability

import "math"

/*
Rank tracks P(history <= current sample) over retained observations.
*/
type Rank struct {
	history []float64
	minimum float64
	maximum float64
}

/*
RankOutput reports the empirical probability and retained sample count.
*/
type RankOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewRank returns a typed empirical rank probability tracker.
*/
func NewRank() *Rank {
	return &Rank{}
}

/*
Measure adds one sample and returns its empirical rank probability.
*/
func (rank *Rank) Measure(sample float64) (RankOutput, error) {
	if err := finiteProbability("rank", sample); err != nil {
		return RankOutput{}, err
	}

	if len(rank.history) == 0 {
		rank.minimum = sample
		rank.maximum = sample
	} else {
		rank.minimum = math.Min(rank.minimum, sample)
		rank.maximum = math.Max(rank.maximum, sample)
	}

	rank.history = append(rank.history, sample)
	atOrBelow := 0

	for _, observed := range rank.history {
		if observed <= sample {
			atOrBelow++
		}
	}

	return RankOutput{
		Value: float64(atOrBelow) / float64(len(rank.history)),
		Ready: true,
		Count: len(rank.history),
	}, nil
}

/*
Reset clears retained rank history.
*/
func (rank *Rank) Reset() {
	rank.history = nil
	rank.minimum = 0
	rank.maximum = 0
}
