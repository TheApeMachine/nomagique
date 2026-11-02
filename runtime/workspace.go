package runtime

import (
	"context"

	"github.com/smarty/go-disruptor"
	"github.com/theapemachine/errnie"
)

const (
	bufferSize = 1024 * 64 // must be a power of 2
	bufferMask = bufferSize - 1
)

// ringBuffer is owned by the caller of go-disruptor (see README): Reserve a
// slot, write the payload, then Commit so handler groups can see it.
var ringBuffer = [bufferSize]*Envelope{}

func optionList[O any](initial ...O) []O {
	return initial
}

/*
Ring is one disruptor handler group. Its nodes run concurrently against the
same tick, and the next ring does not start until every node here has
returned. Group is the name the diagnostics page uses for the ring.
*/
type Ring struct {
	Group string
	Nodes []Node
}

type Workspace struct {
	*System
	channel  disruptor.Disruptor
	mask     int64
	capacity int
	stages   [][]*Consumer
	lastFan  int
}

/*
NewWorkspace builds the disruptor pipeline.
*/
func NewWorkspace(
	ctx context.Context,
	label string,
	stages []Ring,
) *Workspace {
	capacity := bufferSize
	mask := int64(capacity - 1)

	lastIdx := -1
	for i, stage := range stages {
		if len(stage.Nodes) > 0 {
			lastIdx = i
		}
	}

	workspace := &Workspace{
		mask:     mask,
		capacity: capacity,
		stages:   make([][]*Consumer, len(stages)),
	}

	opts := optionList(
		disruptor.Options.BufferCapacity(uint32(capacity)),
		disruptor.Options.WriterCount(1),
	)

	for stageIdx, stage := range stages {
		consumers := make([]*Consumer, 0, len(stage.Nodes))
		group := make([]disruptor.Handler, 0, len(stage.Nodes))

		for _, node := range stage.Nodes {
			consumer := NewConsumer(node, capacity, mask)
			consumers = append(consumers, consumer)

			group = append(group, &stageHandler{
				consumer:  consumer,
				workspace: workspace,
				last:      stageIdx == lastIdx,
			})
		}

		workspace.stages[stageIdx] = consumers

		if len(group) > 0 {
			opts = append(opts, disruptor.Options.NewHandlerGroup(group...))
		}
	}

	channel, err := disruptor.New(opts...)
	workspace.System = NewSystem(ctx, label, channel, workspace)

	if err != nil {
		workspace.Error(errnie.Err(
			errnie.Internal,
			"[workspace] error creating LMAX disruptor channel",
			err,
		))

		return nil
	}

	workspace.channel = channel

	if lastIdx >= 0 {
		workspace.lastFan = len(workspace.stages[lastIdx])
	}

	go workspace.channel.Listen()

	return workspace
}

/*
Step publishes the envelope into the disruptor and blocks until every node
in the last ring has finished, so the caller can safely read/marshal it.
*/
func (workspace *Workspace) Step(envelope *Envelope) *Envelope {
	if workspace.Status() != READY {
		errnie.Warn(workspace.Name() + ": Step called before READY; dropping event")
		return envelope
	}

	if envelope == nil {
		errnie.Error(errnie.Err(
			errnie.Internal,
			"[workspace] nil payload passed to Step",
			nil,
		))

		return envelope
	}

	select {
	case <-workspace.Context().Done():
		workspace.Error(workspace.Context().Err())
		return envelope
	default:
		if workspace.Error() != nil {
			workspace.Error(errnie.Err(
				errnie.Internal,
				"[workspace] internal error encountered",
				nil,
			))

			return envelope
		}
	}

	envelope.arm(workspace.lastFan)

	seq := workspace.channel.Reserve(1)
	ringBuffer[seq&workspace.mask] = envelope
	workspace.channel.Commit(seq, seq)

	envelope.await()

	return envelope
}

type stageHandler struct {
	consumer  *Consumer
	workspace *Workspace
	last      bool
}

func (sh *stageHandler) Handle(lower, upper int64) {
	for seq := lower; seq <= upper; seq++ {
		envelope := ringBuffer[seq&bufferMask]

		if envelope == nil {
			continue
		}

		sh.consume(envelope)
	}
}

func (sh *stageHandler) consume(envelope *Envelope) {
	if sh.last {
		defer envelope.release()
	}

	sh.consumer.Step(envelope)
}
