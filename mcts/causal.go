package mcts

// State represents the environment interface that MCTS interacts with.
type State interface {
	IsTerminal() bool
	GetReward() float64
	GetPossibleActions() []float64
	ApplyAction(action float64) State
	ToVector() []float64 // Converts the state to the row format expected by causal.Table
}

// InterventionMapper maps a discrete action onto the level the SCM's treatment
// variable is held at to represent it.
//
// Actions are an enum and a treatment is a measured quantity, so the two share
// a scale only by accident. A State that implements this names the level each
// action corresponds to; one that does not is intervened on with the action
// value itself, which is the historical behaviour.
type InterventionMapper interface {
	GetInterventionLevel(action float64) float64
}

// interventionLevel is the treatment level representing the action that
// produced a node.
func interventionLevel(node *Node) float64 {
	if mapper, ok := node.State.(InterventionMapper); ok {
		return mapper.GetInterventionLevel(node.Action)
	}

	return node.Action
}

// CausalEngine abstracts your custom causal package's analytical methods.
// This interface allows us to pass wrapped versions of your private package methods.
type CausalEngine interface {
	DoExpectation(rows [][]float64, target, minRows, treatment int, level float64, controls []int) (float64, error)
	AbductiveCounterfactual(rows [][]float64, target, minRows int, features []int, linear bool, row []float64, treatment int, intervention float64) (float64, float64, error)
}

// Node represents a state-action configuration in the MCTS tree.
type Node struct {
	State          State
	Action         float64 // The action that transitioned the environment to this State
	Parent         *Node
	Children       []*Node
	Visits         int
	TotalReward    float64
	UntakenActions []float64
}
