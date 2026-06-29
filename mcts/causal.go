package mcts

// State represents the environment interface that MCTS interacts with.
type State interface {
	IsTerminal() bool
	GetReward() float64
	GetPossibleActions() []float64
	ApplyAction(action float64) State
	ToVector() []float64 // Converts the state to the row format expected by causal.Table
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
