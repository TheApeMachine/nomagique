package market

import (
	"encoding/json"
	"sync"
	"time"
)

/*
TickerData is one symbol's public ticker quote on the desk.
*/
type TickerData struct {
	Symbol    string    `json:"symbol"`
	VolumeUSD float64   `json:"volume_usd"`
	ChangePct float64   `json:"change_pct"`
	High24h   float64  `json:"high_24h"`
	Low24h    float64  `json:"low_24h"`
	Last      float64   `json:"last"`
	Open      float64   `json:"open"`
	UpdatedAt time.Time `json:"updated_at"`
}

/*
Ticker is venue-agnostic public ticker state keyed by symbol.
*/
type Ticker struct {
	mu       sync.RWMutex
	Rows     map[string]*TickerData `json:"rows"`
}

/*
NewTicker seeds oracle inputs (volume, day move, 24h range) Price polls into rows.
*/
func NewTicker(rows map[string]*TickerData) *Ticker {
	if rows == nil {
		rows = make(map[string]*TickerData)
	}

	return &Ticker{Rows: rows}
}


/*
Request is empty: ticker is a broadcast push channel.
*/
func (ticker *Ticker) Request() map[string]any {
	return map[string]any{}
}

/*
Set stores one symbol's quote under lock for grid day/volume/peak desk bools.
*/
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

	if copied.UpdatedAt.IsZero() {
		copied.UpdatedAt = time.Now().UTC()
	}

	ticker.Rows[symbol] = &copied
}

/*
Of returns a copy of one row so callers cannot mutate shared ticker state.
*/
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

/*
VolumeUSD feeds liquid_quote / quiet_tape oracle lamps on Observe.
*/
func (ticker *Ticker) VolumeUSD(symbol string) float64 {
	row := ticker.Of(symbol)
	if row == nil {
		return 0
	}

	return row.VolumeUSD
}

/*
ChangePct feeds session_move / flat_session oracle lamps on Observe.
*/
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

/*
MarshalJSON snapshots ticker rows for UI and persisted diagnostics.
*/
func (ticker *Ticker) MarshalJSON() ([]byte, error) {
	ticker.mu.RLock()
	defer ticker.mu.RUnlock()

	type alias Ticker
	return json.Marshal(&struct {
		Rows map[string]*TickerData `json:"rows"`
	}{Rows: ticker.Rows})
}

/*
UnmarshalJSON reloads ticker rows from store; reloads from store.
*/
func (ticker *Ticker) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Rows map[string]*TickerData `json:"rows"`
	}

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	ticker.mu.Lock()
	ticker.Rows = decoded.Rows

	if ticker.Rows == nil {
		ticker.Rows = make(map[string]*TickerData)
	}

	ticker.mu.Unlock()


	return nil
}
