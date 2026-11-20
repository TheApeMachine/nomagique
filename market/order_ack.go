package market

import (
	"encoding/json"
	"time"
)

/*
OrderAck is a venue-agnostic acknowledgement of an order submit.
*/
type OrderAck struct {
	Method   string    `json:"method"`
	OrderID  string    `json:"order_id"`
	Success  bool      `json:"success"`
	Error    string    `json:"error"`
	ReqID    int64     `json:"req_id"`
	TimeIn   time.Time `json:"time_in"`
	TimeOut  time.Time `json:"time_out"`
}

/*
NewOrderAck maps venue submit responses into a typed ack desk tiers correlate by req_id.
*/
func NewOrderAck(data map[string]any) *OrderAck {
	out := &OrderAck{}

	if value, ok := data["method"].(string); ok {
		out.Method = value
	}

	if value, ok := data["order_id"].(string); ok {
		out.OrderID = value
	}

	if value, ok := data["success"].(bool); ok {
		out.Success = value
	}

	if value, ok := data["error"].(string); ok {
		out.Error = value
	}

	if value, ok := data["req_id"].(int64); ok {
		out.ReqID = value
	}

	return out
}


/*
Request is empty: acks arrive on the private websocket response stream.
*/
func (ack *OrderAck) Request() map[string]any {
	return map[string]any{}
}

/*
ToMarket copies ack fields into the tier slot preserving timing.
*/
func (ack *OrderAck) ToMarket(other MarketType) MarketType {
	dst, ok := other.(*OrderAck)

	if !ok || dst == nil || ack == nil {
		return other
	}

	dst.Method = ack.Method
	dst.OrderID = ack.OrderID
	dst.Success = ack.Success
	dst.Error = ack.Error
	dst.ReqID = ack.ReqID
	dst.TimeIn = ack.TimeIn
	dst.TimeOut = ack.TimeOut

	return dst
}

/*
FromMarket imports a peer ack for replay and paper order tracking.
*/
func (ack *OrderAck) FromMarket(other MarketType) MarketType {
	src, ok := other.(*OrderAck)

	if !ok || src == nil || ack == nil {
		return ack
	}

	return src.ToMarket(ack)
}

/*
MarshalJSON persists submit outcomes on diagnostic store keys.
*/
func (ack *OrderAck) MarshalJSON() ([]byte, error) {
	type alias OrderAck
	return json.Marshal((*alias)(ack))
}

/*
UnmarshalJSON reloads ack state from persisted frames.
*/
func (ack *OrderAck) UnmarshalJSON(data []byte) error {
	type alias OrderAck
	var decoded alias

	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*ack = OrderAck(decoded)


	return nil
}
