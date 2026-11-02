package market

import "github.com/shopspring/decimal"

type Asset struct {
	updater func(asset *Asset)
	Symbol  string          `json:"symbol"`
	Capital decimal.Decimal `json:"capital"`
	PnL     decimal.Decimal `json:"pnl"`
	Equity  decimal.Decimal `json:"equity"`
}

func NewAsset(
	updater func(asset *Asset),
	symbol string,
	capital decimal.Decimal,
	pnl decimal.Decimal,
	equity decimal.Decimal,
) *Asset {
	return &Asset{
		updater: updater,
		Symbol:  symbol,
		Capital: capital,
		PnL:     pnl,
		Equity:  equity,
	}
}
