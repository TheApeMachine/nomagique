package runtime

import (
	"context"
	"sync/atomic"

	"github.com/smarty/go-disruptor"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/data"
)

func optionList[O any](initial ...O) []O {
	return initial
}

/*
Workspace runs registered nodes in dependency stages using an LMAX Disruptor.
Between stages, a post-barrier runtime join handler composes completed prior
producer outputs into an immutable WORM join measurement without metric copying.
*/
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
	buffer   []*data.Value
	dones    []chan struct{}
	remain   []int32
	lastN    int32
	settled  atomic.Value // func(*data.Value)
	onTrace  atomic.Value
}

/*
NewWorkspace builds the disruptor pipeline. Each sink (store, hub, ...) runs in
a final handler group after every stage, receiving the tick's Value through
io.Copy(sink, value).
*/
func NewWorkspace(
	ctx context.Context,
	label string,
	stages []Ring,
) *Workspace {
	capacity := 8192
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
		buffer:   make([]*data.Value, capacity),
		dones:    make([]chan struct{}, capacity),
		remain:   make([]int32, capacity),
	}
	if lastIdx >= 0 {
		workspace.lastN = int32(len(stages[lastIdx].Nodes))
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
	go workspace.channel.Listen()

	return workspace
}

func (workspace *Workspace) Step(value *data.Value) *data.Value {
	if workspace.Status() != READY {
		errnie.Warn(workspace.Name() + ": Step called before READY; dropping event")
		return value
	}

	if value == nil {
		errnie.Error(errnie.Err(
			errnie.Internal,
			"[workspace] nil payload passed to Step",
			nil,
		))

		return value
	}

	select {
	case <-workspace.Context().Done():
		workspace.Error(workspace.Context().Err())
		return value
	default:
		if workspace.Error() != nil {
			workspace.Error(errnie.Err(
				errnie.Internal,
				"[workspace] internal error encountered",
				nil,
			))

			return value
		}
	}

	seq := workspace.channel.Reserve(1)

	slot := seq & workspace.mask
	done := make(chan struct{})
	if workspace.lastN > 0 {
		atomic.StoreInt32(&workspace.remain[slot], workspace.lastN)
		workspace.dones[slot] = done
	}
	workspace.buffer[slot] = value

	workspace.channel.Commit(seq, seq)

	// Step returns the tick only after every ring has handled it. The caller
	// publishes that value; a frame sent before Commit is missing its layers.
	if workspace.lastN > 0 {
		select {
		case <-done:
		case <-workspace.Context().Done():
		}
	}

	return value
}

type stageHandler struct {
	consumer  *Consumer
	workspace *Workspace
	last      bool
}

func (sh *stageHandler) Handle(lower, upper int64) {
	for seq := lower; seq <= upper; seq++ {
		value := sh.workspace.buffer[seq&sh.workspace.mask]

		if value != nil {
			sh.consumer.Step(value)
		}

		if sh.last {
			sh.workspace.finish(seq)
		}
	}
}

func (workspace *Workspace) OnSettled(fn func(*data.Value)) {
	if workspace == nil {
		return
	}
	workspace.settled.Store(fn)
}

func (workspace *Workspace) finish(seq int64) {
	slot := seq & workspace.mask
	if atomic.AddInt32(&workspace.remain[slot], -1) != 0 {
		return
	}

	if fn, _ := workspace.settled.Load().(func(*data.Value)); fn != nil {
		if value := workspace.buffer[slot]; value != nil {
			fn(value)
		}
	}

	done := workspace.dones[slot]
	if done == nil {
		return
	}

	select {
	case <-done:
	default:
		close(done)
	}
}
