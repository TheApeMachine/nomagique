package market

import (
	"context"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

/*
Balance is the book wallet: quote cash and per-asset capital / pnl / equity.
Venue polling belongs behind Feed; this type only holds and serves state.
*/
type Balance struct {
	mu       sync.RWMutex
	Quote    string   `json:"quote"`
	Assets   []*Asset `json:"assets"`
	onChange func(*Balance)
}

/*
Feed pulls a fresh asset snapshot from a venue or simulator.
*/
type Feed interface {
	Fetch(ctx context.Context) ([]*Asset, error)
}

func NewBalance(quote string) *Balance {
	return &Balance{
		Quote:  quote,
		Assets: make([]*Asset, 0),
	}
}

func (balance *Balance) OnChange(fn func(*Balance)) {
	balance.mu.Lock()
	defer balance.mu.Unlock()
	balance.onChange = fn
}

func (balance *Balance) AddAsset(asset *Asset) {
	if asset == nil {
		return
	}

	balance.mu.Lock()
	defer balance.mu.Unlock()
	balance.Assets = append(balance.Assets, asset)
}

func (balance *Balance) GetAsset(symbol string) *Asset {
	balance.mu.RLock()
	defer balance.mu.RUnlock()
	return balance.lookup(symbol)
}

func (balance *Balance) UpdateAsset(asset *Asset) {
	if asset == nil {
		return
	}

	balance.mu.Lock()
	defer balance.mu.Unlock()

	for i, existing := range balance.Assets {
		if existing.Symbol == asset.Symbol {
			balance.Assets[i] = asset
			return
		}
	}

	balance.Assets = append(balance.Assets, asset)
}

/*
Replace swaps the whole book under lock and notifies listeners.
*/
func (balance *Balance) Replace(assets []*Asset) {
	balance.mu.Lock()
	balance.Assets = assets
	fn := balance.onChange
	balance.mu.Unlock()

	if fn != nil {
		fn(balance)
	}
}

/*
Seed installs starting quote capital when the book is empty.
*/
func (balance *Balance) Seed(bankroll decimal.Decimal) {
	if bankroll.Sign() <= 0 || balance.Quote == "" {
		return
	}

	balance.mu.Lock()
	defer balance.mu.Unlock()

	if balance.lookup(balance.Quote) != nil {
		return
	}

	balance.Assets = append(balance.Assets, NewAsset(
		nil, balance.Quote, bankroll, decimal.Zero, bankroll,
	))
}

/*
Cash is available capital for an asset (exact symbol, then venue alias).
*/
func (balance *Balance) Cash(symbol string) *decimal.Decimal {
	balance.mu.RLock()
	defer balance.mu.RUnlock()

	if asset := balance.lookup(symbol); asset != nil {
		capital := asset.Capital
		return &capital
	}

	return nil
}

/*
Reading is the dashboard cash / equity / unrealized map for the quote asset.
*/
func (balance *Balance) Reading() map[string]any {
	balance.mu.RLock()
	defer balance.mu.RUnlock()

	asset := balance.lookup(balance.Quote)

	if asset == nil {
		return nil
	}

	cash := asset.Capital
	equity := asset.Equity
	unrealized := asset.PnL

	return map[string]any{
		"cash":       &cash,
		"equity":     &equity,
		"unrealized": &unrealized,
	}
}

/*
Sync replaces assets from feed.
*/
func (balance *Balance) Sync(ctx context.Context, feed Feed) error {
	if feed == nil {
		return nil
	}

	assets, err := feed.Fetch(ctx)

	if err != nil {
		return err
	}

	balance.Replace(assets)
	return nil
}

/*
TimedFeed is a Feed that knows how often it should be polled.
*/
type TimedFeed interface {
	Feed
	Interval() time.Duration
}

/*
Follow starts Watch using feed.Interval when feed is a TimedFeed, else 1s.
*/
func (balance *Balance) Follow(ctx context.Context, feed Feed) {
	if feed == nil {
		return
	}

	interval := time.Second

	if timed, ok := feed.(TimedFeed); ok {
		interval = timed.Interval()
	}

	balance.Watch(ctx, interval, feed)
}

/*
Watch polls feed on interval until ctx is done.
*/
func (balance *Balance) Watch(
	ctx context.Context, interval time.Duration, feed Feed,
) {
	if feed == nil || interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		_ = balance.Sync(ctx, feed)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = balance.Sync(ctx, feed)
			}
		}
	}()
}

func (balance *Balance) lookup(symbol string) *Asset {
	if symbol == "" {
		return nil
	}

	for _, asset := range balance.Assets {
		if asset != nil && asset.Symbol == symbol {
			return asset
		}
	}

	if alias := Alias(symbol); alias != "" {
		for _, asset := range balance.Assets {
			if asset != nil && asset.Symbol == alias {
				return asset
			}
		}
	}

	return nil
}

/*
Alias maps venue legacy currency codes to their counterparts.
*/
func Alias(symbol string) string {
	switch symbol {
	case "USD":
		return "ZUSD"
	case "ZUSD":
		return "USD"
	case "EUR":
		return "ZEUR"
	case "ZEUR":
		return "EUR"
	case "GBP":
		return "ZGBP"
	case "ZGBP":
		return "GBP"
	case "CAD":
		return "ZCAD"
	case "ZCAD":
		return "CAD"
	case "JPY":
		return "ZJPY"
	case "ZJPY":
		return "JPY"
	case "BTC":
		return "XXBT"
	case "XXBT", "XBT":
		return "BTC"
	case "ETH":
		return "XETH"
	case "XETH":
		return "ETH"
	default:
		return ""
	}
}
