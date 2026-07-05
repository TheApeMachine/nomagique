package statistic

/*
PriceRing publishes the current positive price sample.
*/
type PriceRing struct {
	count int
}

/*
NewPriceRing returns a typed positive-price gate.
*/
func NewPriceRing() *PriceRing {
	return &PriceRing{}
}

/*
Measure validates and publishes the current price sample.
*/
func (priceRing *PriceRing) Measure(sample float64) (ScalarOutput, error) {
	if err := finitePositiveStatistic("price-ring", sample); err != nil {
		return ScalarOutput{}, err
	}

	priceRing.count++

	return ScalarOutput{
		Value: sample,
		Ready: true,
		Count: priceRing.count,
	}, nil
}
