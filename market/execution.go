package market

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

/*
ExecutionData is one fill or order event on the desk, venue-agnostic.
*/
type ExecutionData struct {
	OrderID       string           `json:"order_id"`
	ClientOrderID string           `json:"cl_ord_id"`
	OrderUserref  int              `json:"order_userref"`
	ExecID        string           `json:"exec_id"`
	ExecType      string           `json:"exec_type"`
	TradeID       int              `json:"trade_id"`
	Symbol        string           `json:"symbol"`
	Side          string           `json:"side"`
	LastQty       *decimal.Decimal `json:"last_qty"`
	LastPrice     *decimal.Decimal `json:"last_price"`
	LiquidityInd  string           `json:"liquidity_ind"`
	Cost          *decimal.Decimal `json:"cost"`
	OrderType     string           `json:"order_type"`
	Timestamp     time.Time        `json:"timestamp"`
	OrderStatus   string           `json:"order_status"`
	CumQty        *decimal.Decimal `json:"cum_qty"`
	CumCost       *decimal.Decimal `json:"cum_cost"`
	AvgPrice      *decimal.Decimal `json:"avg_price"`
	FeeUsdEquiv   *decimal.Decimal `json:"fee_usd_equiv"`
}

type Execution struct {
	Channel  string           `json:"channel"`
	Type     string           `json:"type"`
	Data     []*ExecutionData `json:"data"`
	Sequence int              `json:"sequence"`
	onChange func(MarketType)
}

func NewExecution(data map[string]any) *Execution {
	channel, _ := data["channel"].(string)
	kind, _ := data["type"].(string)
	rows, _ := data["data"].([]*ExecutionData)

	if channel == "" {
		channel = "executions"
	}

	if kind == "" {
		kind = "update"
	}

	if rows == nil {
		rows = []*ExecutionData{}
	}

	return &Execution{
		Channel: channel,
		Type:    kind,
		Data:    rows,
	}
}

func (execution *Execution) OnChange(fn func(MarketType)) {
	execution.onChange = fn
}

func (execution *Execution) Request() map[string]any {
	return map[string]any{}
}

func (execution *Execution) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*Execution)

	if !ok || dst == nil || execution == nil {
		return other
	}

	onChange := dst.onChange
	dst.Channel = execution.Channel
	dst.Type = execution.Type
	dst.Sequence = execution.Sequence
	dst.Data = append([]*ExecutionData(nil), execution.Data...)
	dst.onChange = onChange

	return dst
}

func (execution *Execution) FromMarket(other MarketType) MarketType {
	src, ok := other.(*Execution)

	if !ok || src == nil || execution == nil {
		return execution
	}

	return src.ToMarket(execution)
}

func (execution *Execution) MarshalJSON() ([]byte, error) {
	type alias Execution
	return json.Marshal((*alias)(execution))
}

func (execution *Execution) UnmarshalJSON(data []byte) error {
	type alias Execution
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	onChange := execution.onChange
	*execution = Execution(decoded)
	execution.onChange = onChange

	if execution.onChange != nil {
		execution.onChange(execution)
	}

	return nil
}
