package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/theapemachine/errnie"
)

/*
RuntimeSystem is the lifecycle surface Grid, Workspace, and broker tiers embed
via *System without pulling in disruptor details.
*/
type RuntimeSystem interface {
	Name() string
	Context() context.Context
	Status() Stage
	Transition(Stage)
	Error(...error) error
	AddCloser(io.Closer)
	Close() error
}

/*
System owns cancel, stage, and closer ordering so one FATAL tears down
websockets, disruptor, and store drain in a predictable stack.
*/
type System struct {
	ctx     context.Context
	cancel  context.CancelFunc
	name    string
	err     error
	status  atomic.Pointer[Status]
	closers []io.Closer
}

/*
NewSystem roots context and seeds INIT status before callers Transition to READY.
*/
func NewSystem(
	ctx context.Context,
	name string,
	closers ...io.Closer,
) *System {

	ctx, cancel := context.WithCancel(ctx)

	system := &System{
		ctx:     ctx,
		cancel:  cancel,
		name:    name,
		status:  atomic.Pointer[Status]{},
		closers: closers,
	}

	system.status.Store(NewStatus())

	return system
}

/*
Name labels log lines and UI pages for this runtime object.
*/
func (system *System) Name() string { return system.name }

/*
Context is the shutdown root shared with store client and network readers.
*/
func (system *System) Context() context.Context { return system.ctx }

/*
Transition records stage changes for diagnostics and drops duplicate reports.
*/
func (system *System) Transition(stage Stage) {
	old := system.status.Load().Current()
	system.status.Load().Transition(stage)

	if system.status.Load().Current() == stage && old != stage {
		errnie.Info(fmt.Sprintf("%s: %s -> %s", system.name, old, stage))
	}
}

/*
Status is the current stage; Workspace.Step consults it before Reserve/Commit.
*/
func (system *System) Status() Stage { return system.status.Load().Current() }

/*
ShuttingDown reports whether err is only the cancellation of ctx during an
orderly shutdown: ctx is done and err is context.Canceled. Such an error is
the expected end of in-flight work, not a failure; anything else is.
*/
func ShuttingDown(ctx context.Context, err error) bool {
	return ctx.Err() != nil && errors.Is(err, context.Canceled)
}

/*
Error accumulates fatal faults, promotes ERROR/FATAL, and closes on internal errnie codes.
*/
func (system *System) Error(errs ...error) error {
	var added bool

	for _, err := range errs {
		if err == nil || ShuttingDown(system.ctx, err) {
			continue
		}

		added = true

		if system.err == nil {
			system.err = err
			continue
		}

		system.err = errors.Join(system.err, err)
	}

	if added && system.err != nil {
		if system.status.Load().Current() != FATAL {
			system.Transition(ERROR)
		}

		errnie.Error(system.err)

		errnieErr, ok := errnie.AsErrnie(system.err)

		if ok && errnie.IsInternal(errnieErr) {
			system.Close()
		}
	}

	return system.err
}

/*
AddCloser registers teardown after cancel so nested Systems are not double-closed.
*/
func (system *System) AddCloser(closer io.Closer) {
	system.closers = append(system.closers, closer)
}

/*
System returns the embedded *System for types that satisfy RuntimeSystem.
*/
func (system *System) System() *System { return system }

/*
Close cancels context and runs closers so SIGINT still drains uploads and disruptor.
*/
func (system *System) Close() error {
	if system.cancel == nil {
		return system.err
	}

	system.cancel()

	closers := system.closers
	system.closers = nil

	for _, closer := range closers {
		if closer == nil {
			continue
		}

		if sysGetter, ok := closer.(interface{ System() *System }); ok && sysGetter.System() == system {
			continue
		}

		if err := closer.Close(); err != nil {
			errnie.Error(errnie.Err(errnie.IO, fmt.Sprintf("%s: closing %T failed", system.name, closer), err))
			system.err = errors.Join(system.err, err)
		}
	}

	return system.err
}
