package vector

/*
L1 book input channel indices for NewL1BookExtractor.

These name the four raw touch fields written before Extract:
  - L1BidPrice, L1AskPrice — best bid and ask prices
  - L1BidQty, L1AskQty — displayed size at those prices
*/
const (
	L1BidPrice = iota
	L1AskPrice
	L1BidQty
	L1AskQty
	L1InputCount
)

/*
L1 book feature indices returned by NewL1BookExtractor.

After Extract:
  - L1MidPrice — average of bid and ask
  - L1SpreadBPS — bid–ask distance in basis points of mid
  - L1Imbalance — (bid qty − ask qty) / (bid qty + ask qty), in [−1, 1]
*/
const (
	L1MidPrice = iota
	L1SpreadBPS
	L1Imbalance
)

/*
NewL1BookExtractor builds a four-input, three-feature book-touch preset.

It is the standard microstructure vector for level-one order books: feed bid/ask
price and quantity through InputSlot or SetInput, then read mid, spread, and
imbalance via FeatureNode inside composed Numbers.

Spread and imbalance return zero when mid or total size is non-positive, rather
than dividing by zero.
*/
func NewL1BookExtractor() (*FeatureExtractor, error) {
	return NewFeatureExtractor(L1InputCount,
		func(inputs []float64) float64 {
			return (inputs[L1BidPrice] + inputs[L1AskPrice]) / 2
		},
		func(inputs []float64) float64 {
			mid := (inputs[L1BidPrice] + inputs[L1AskPrice]) / 2

			if mid <= 0 {
				return 0
			}

			return (inputs[L1AskPrice] - inputs[L1BidPrice]) / mid * 10000
		},
		func(inputs []float64) float64 {
			total := inputs[L1BidQty] + inputs[L1AskQty]

			if total <= 0 {
				return 0
			}

			return (inputs[L1BidQty] - inputs[L1AskQty]) / total
		},
	)
}
