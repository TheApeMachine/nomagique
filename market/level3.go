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

type Level3Data struct {
	Symbol   string         `json:"symbol"`
	Checksum int            `json:"checksum"`
	Bids     []*Level3Order `json:"bids"`
	Asks     []*Level3Order `json:"asks"`
}

type Level3 struct {
	Channel  string        `json:"channel"`
	Type     string        `json:"type"`
	Data     []*Level3Data `json:"data"`
	onChange func(MarketType)
}

func NewLevel3() *Level3 {
	return &Level3{
		Channel: "level3",
		Type:    "update",
		Data:    make([]*Level3Data, 0),
	}
}

func (level3 *Level3) OnChange(fn func(MarketType)) {
	level3.onChange = fn
}

func (level3 *Level3) Request() map[string]any {
	return map[string]any{}
}

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

func (level3 *Level3) FromMarket(other MarketType) MarketType {
	src, ok := other.(*Level3)

	if !ok || src == nil || level3 == nil {
		return level3
	}

	return src.ToMarket(level3)
}

func (level3 *Level3) MarshalJSON() ([]byte, error) {
	type alias Level3
	return sonic.Marshal((*alias)(level3))
}

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
