package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/market"
)

type markNode struct {
	*System
	flag *atomic.Bool
}

func (node *markNode) Step(envelope *Envelope) *Envelope {
	time.Sleep(5 * time.Millisecond)
	node.flag.Store(true)
	envelope.Decide(&Decision{Symbol: "BTC/USD", Action: "seen"})
	return envelope
}

func TestWorkspaceStepPublishesWithoutWaiting(t *testing.T) {
	Convey("Step returns before the last ring finishes the envelope", t, func() {
		done := &atomic.Bool{}
		ws := NewWorkspace(context.Background(), "test", []Ring{{
			Group: "only",
			Nodes: []Node{&markNode{
				System: NewSystem(context.Background(), "mark"),
				flag:   done,
			}},
		}})
		So(ws, ShouldNotBeNil)
		ws.Transition(READY)

		trade := market.NewTrade("trade", "update", []*market.TradeData{{
			Symbol: "BTC/USD",
			Price:  decimal.NewFromInt(1),
			Qty:    decimal.NewFromInt(1),
		}})

		out := ws.Step(NewEnvelope(trade))

		So(done.Load(), ShouldBeFalse)

		deadline := time.Now().Add(time.Second)
		for !done.Load() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}

		So(done.Load(), ShouldBeTrue)
		So(out.Decisions["BTC/USD"], ShouldNotBeNil)
		So(out.Decisions["BTC/USD"].Action, ShouldEqual, "seen")
	})
}
