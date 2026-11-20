package market

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

/*
Side is the aggressor direction on a print; embedded on Envelope.Trade for signals.
*/
type Side string

/*
OrderType tags market versus limit fills on the public tape frame.
*/
type OrderType string

/*
SideBuy, SideSell and order kinds mirror venue trade payloads desk normalizes.
*/
const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"

	OrderTypeMarket OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
)

/*
String implements fmt for logging without importing fmt on the hot path.
*/
func (side Side) String() string {
	return string(side)
}

/*
String implements fmt for order type labels on persisted trade JSON.
*/
func (orderType OrderType) String() string {
	return string(orderType)
}

/*
TradeData is one print on the batch frame the disruptor embeds in runtime.Envelope.
*/
type TradeData struct {
	Symbol    string          `json:"symbol"`
	Side      Side            `json:"side"`
	Price     decimal.Decimal `json:"price"`
	Qty       decimal.Decimal `json:"qty"`
	OrderType OrderType       `json:"ord_type"`
	TradeID   int64           `json:"trade_id"`
	Timestamp time.Time       `json:"timestamp"`
}

/*
Trade is the venue-agnostic tape batch; desk pumps it into Workspace after coalesce.
*/
type Trade struct {
	Channel  string       `json:"channel"`
	Type     string       `json:"type"`
	Data     []*TradeData `json:"data"`
}

/*
NewTrade wraps decoded prints without allocating maps each frame.
*/
func NewTrade(
	channel string,
	tradeType string,
	data []*TradeData,
) *Trade {
	return &Trade{
		Channel: channel,
		Type:    tradeType,
		Data:    data,
	}
}

/*
Batches groups prints by symbol in first-seen order. Prints for one symbol
stay in array order so a single goroutine can apply them sequentially.
*/
func (trade *Trade) Batches() [][]*TradeData {
	if trade == nil || len(trade.Data) == 0 {
		return nil
	}

	index := make(map[string]int, len(trade.Data))
	batches := make([][]*TradeData, 0)

	for _, data := range trade.Data {
		symbol := ""

		if data != nil {
			symbol = data.Symbol
		}

		slot, ok := index[symbol]

		if !ok {
			slot = len(batches)
			index[symbol] = slot
			batches = append(batches, nil)
		}

		batches[slot] = append(batches[slot], data)
	}

	return batches
}


/*
Request is empty: trades arrive on venue push subscriptions.
*/
func (trade *Trade) Request() map[string]any {
	return map[string]any{}
}

/*
ToMarket copies prints into the tier-local Trade.
*/
func (trade *Trade) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Trade)

	if !ok || dst == nil || trade == nil {
		return other
	}

	dst.Channel = trade.Channel
	dst.Type = trade.Type
	dst.Data = append([]*TradeData(nil), trade.Data...)

	return dst
}

/*
FromMarket pulls a peer frame into this Trade for replay and paper tiers.
*/
func (trade *Trade) FromMarket(other MarketType) MarketType {
	src, ok := other.(*Trade)

	if !ok || src == nil || trade == nil {
		return trade
	}

	return src.ToMarket(trade)
}

/*
MarshalJSON persists tape batches inside envelope ticks uploaded to store.
*/
func (trade *Trade) MarshalJSON() ([]byte, error) {
	type alias Trade
	return json.Marshal((*alias)(trade))
}

/*
UnmarshalJSON reloads trade history from store keys; reloads from store.
*/
func (trade *Trade) UnmarshalJSON(data []byte) error {
	type alias Trade
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*trade = Trade(decoded)


	return nil
}
