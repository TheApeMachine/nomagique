package market

import (
	"encoding/json"
	"sync"
)

/*
TickerData is one symbol's public ticker quote on the desk.
*/
type TickerData struct {
	Symbol    string  `json:"symbol"`
	VolumeUSD float64 `json:"volume_usd"`
	ChangePct float64 `json:"change_pct"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Last      float64 `json:"last"`
}

/*
Ticker is venue-agnostic public ticker state keyed by symbol.
*/
type Ticker struct {
	mu       sync.RWMutex
	Rows     map[string]*TickerData `json:"rows"`
	onChange func(MarketType)
}

func NewTicker(rows map[string]*TickerData) *Ticker {
	if rows == nil {
		rows = make(map[string]*TickerData)
	}

	return &Ticker{Rows: rows}
}

func (ticker *Ticker) OnChange(fn func(MarketType)) {
	ticker.onChange = fn
}

func (ticker *Ticker) Request() map[string]any {
	return map[string]any{}
}

func (ticker *Ticker) Set(symbol string, row *TickerData) {
	if ticker == nil || symbol == "" || row == nil {
		return
	}

	ticker.mu.Lock()
	defer ticker.mu.Unlock()

	if ticker.Rows == nil {
		ticker.Rows = make(map[string]*TickerData)
	}

	copied := *row
	copied.Symbol = symbol
	ticker.Rows[symbol] = &copied
}

func (ticker *Ticker) Of(symbol string) *TickerData {
	if ticker == nil {
		return nil
	}

	ticker.mu.RLock()
	defer ticker.mu.RUnlock()

	if ticker.Rows == nil {
		return nil
	}

	row := ticker.Rows[symbol]
	if row == nil {
		return nil
	}

	copied := *row
	return &copied
}

func (ticker *Ticker) VolumeUSD(symbol string) float64 {
	row := ticker.Of(symbol)
	if row == nil {
		return 0
	}

	return row.VolumeUSD
}

func (ticker *Ticker) ChangePct(symbol string) float64 {
	row := ticker.Of(symbol)
	if row == nil {
		return 0
	}

	return row.ChangePct
}

/*
ToMarket merges this ticker's rows into other when other is a *Ticker.
*/
func (ticker *Ticker) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Ticker)

	if !ok || dst == nil || ticker == nil {
		return other
	}

	ticker.mu.RLock()
	rows := make(map[string]*TickerData, len(ticker.Rows))

	for symbol, row := range ticker.Rows {
		if row == nil {
			continue
		}

		copied := *row
		copied.Symbol = symbol
		rows[symbol] = &copied
	}
	ticker.mu.RUnlock()

	dst.mu.Lock()
	defer dst.mu.Unlock()

	if dst.Rows == nil {
		dst.Rows = make(map[string]*TickerData)
	}

	for symbol, row := range rows {
		dst.Rows[symbol] = row
	}

	return dst
}

/*
FromMarket copies other into this ticker when other is a *Ticker.
*/
func (ticker *Ticker) FromMarket(other MarketType) MarketType {
	src, ok := other.(*Ticker)

	if !ok || src == nil || ticker == nil {
		return ticker
	}

	return src.ToMarket(ticker)
}

func (ticker *Ticker) MarshalJSON() ([]byte, error) {
	ticker.mu.RLock()
	defer ticker.mu.RUnlock()

	type alias Ticker
	return json.Marshal(&struct {
		Rows map[string]*TickerData `json:"rows"`
	}{Rows: ticker.Rows})
}

func (ticker *Ticker) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Rows map[string]*TickerData `json:"rows"`
	}

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	ticker.mu.Lock()
	onChange := ticker.onChange
	ticker.Rows = decoded.Rows

	if ticker.Rows == nil {
		ticker.Rows = make(map[string]*TickerData)
	}

	ticker.onChange = onChange
	ticker.mu.Unlock()

	if onChange != nil {
		onChange(ticker)
	}

	return nil
}
