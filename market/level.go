package market

import (
	"sort"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"
)

/*
Level is one price with its resting order queue.
*/
type Level struct {
	Price      *decimal.Decimal `json:"price,omitempty"`
	Quantity   *decimal.Decimal `json:"quantity,omitempty"`
	Timestamp  time.Time        `json:"time,omitempty"`
	Lower      *Level           `json:"-"`
	Higher     *Level           `json:"-"`
	orders     map[string]*Order
	queue      []*Order
	queueDirty atomic.Bool
}

func NewLevel() *Level {
	return &Level{
		orders: make(map[string]*Order),
	}
}

func (level *Level) update(delta *Delta) {
	px := *delta.Price
	level.Price = &px

	if delta.ID == "" {
		level.orders = make(map[string]*Order)
		level.queue = nil
		qty := *delta.Quantity
		level.Quantity = &qty
		level.Timestamp = delta.Timestamp
		level.queueDirty.Store(true)
		return
	}

	order, existed := level.orders[delta.ID]

	if delta.Quantity.Sign() <= 0 {
		if existed {
			without := level.Quantity.Sub(*order.Quantity)
			level.Quantity = &without
			delete(level.orders, delta.ID)
		}

		level.Timestamp = delta.Timestamp
		level.queueDirty.Store(true)
		return
	}

	if existed {
		without := level.Quantity.Sub(*order.Quantity)
		sum := without.Add(*delta.Quantity)
		level.Quantity = &sum
		qty := *delta.Quantity
		order.Quantity = &qty
		order.Timestamp = delta.Timestamp
	} else {
		limit := *delta.Price
		qty := *delta.Quantity
		level.orders[delta.ID] = &Order{
			ID:         delta.ID,
			LimitPrice: &limit,
			Quantity:   &qty,
			Timestamp:  delta.Timestamp,
			Level:      level,
		}

		if level.Quantity == nil {
			level.Quantity = &qty
		} else {
			sum := level.Quantity.Add(qty)
			level.Quantity = &sum
		}
	}

	level.Timestamp = delta.Timestamp
	level.queueDirty.Store(true)
}

/*
Queue returns resting orders in time priority.
*/
func (level *Level) Queue() []*Order {
	if !level.queueDirty.Load() {
		return level.queue
	}

	queue := make([]*Order, 0, len(level.orders))

	for _, order := range level.orders {
		queue = append(queue, order)
	}

	sort.Slice(queue, func(i, j int) bool {
		return queue[i].Timestamp.Before(queue[j].Timestamp)
	})

	level.queue = queue
	level.queueDirty.Store(false)
	return level.queue
}

/*
Order is one resting limit on a level.
*/
type Order struct {
	ID         string           `json:"id,omitempty"`
	LimitPrice *decimal.Decimal `json:"limitPrice,omitempty"`
	Quantity   *decimal.Decimal `json:"quantity,omitempty"`
	Timestamp  time.Time        `json:"timestamp,omitempty"`
	Level      *Level           `json:"-"`
}
