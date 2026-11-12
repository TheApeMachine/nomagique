package market

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/shopspring/decimal"
)

/*
Level3Order is one resting order on a venue level-3 frame (not a book Level).
*/
type Level3Order struct {
	OrderID    string           `json:"order_id"`
	LimitPrice *decimal.Decimal `json:"limit_price"`
	OrderQty   *decimal.Decimal `json:"order_qty"`
	Timestamp  time.Time        `json:"timestamp"`
	// Event is add|modify|delete on update frames; empty on snapshots.
	Event string `json:"event,omitempty"`
}

/*
Level3Data is one symbol's L3 snapshot or delta batch broker converts into Book updates.
*/
type Level3Data struct {
	Symbol   string         `json:"symbol"`
	Checksum int            `json:"checksum"`
	Bids     []*Level3Order `json:"bids"`
	Asks     []*Level3Order `json:"asks"`
}

/*
Level3 is the venue frame type; gaps corrupt books so desk blocks instead of dropping.
*/
type Level3 struct {
	Channel  string        `json:"channel"`
	Type     string        `json:"type"`
	Data     []*Level3Data `json:"data"`
	onChange func(MarketType)
}

/*
NewLevel3 seeds an empty update frame before the first websocket decode.
*/
func NewLevel3() *Level3 {
	return &Level3{
		Channel: "level3",
		Type:    "update",
		Data:    make([]*Level3Data, 0),
	}
}

/*
OnChange wires BookManager Apply from desk without venue types in broker/book.
*/
func (level3 *Level3) OnChange(fn func(MarketType)) {
	level3.onChange = fn
}

/*
Request is empty: L3 is a push channel after SubscribeBooks.
*/
func (level3 *Level3) Request() map[string]any {
	return map[string]any{}
}

/*
ToMarket merges frames into the tier buffer used before checksum and Book.Update.
*/
func (level3 *Level3) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Level3)

	if !ok || dst == nil || level3 == nil {
		return other
	}

	onChange := dst.onChange
	dst.Channel = level3.Channel
	dst.Type = level3.Type
	dst.Data = append([]*Level3Data(nil), level3.Data...)
	dst.onChange = onChange

	return dst
}

/*
FromMarket copies a stored or relayed L3 frame into this slot for replay.
*/
func (level3 *Level3) FromMarket(other MarketType) MarketType {
	src, ok := other.(*Level3)

	if !ok || src == nil || level3 == nil {
		return level3
	}

	return src.ToMarket(level3)
}

/*
MarshalJSON uses Sonic for large L3 bodies queued to store without heap retention.
*/
func (level3 *Level3) MarshalJSON() ([]byte, error) {
	type alias Level3
	return sonic.Marshal((*alias)(level3))
}

/*
UnmarshalJSON reloads L3 from object storage; keeps onChange for live desk wiring.
*/
func (level3 *Level3) UnmarshalJSON(data []byte) error {
	type alias Level3
	var decoded alias

	if err := sonic.Unmarshal(data, &decoded); err != nil {
		return err
	}

	onChange := level3.onChange
	*level3 = Level3(decoded)
	level3.onChange = onChange

	if level3.onChange != nil {
		level3.onChange(level3)
	}

	return nil
}
