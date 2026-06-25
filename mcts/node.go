package causal

import (
	"errors"
	"math"

	"github.com/theapemachine/nomagique/causal"
)

// CausalState represents a flat slice of float64 features representing
// the state variables and actions, matching the row-major format used in causal.Table.
type CausalState []float64

type CausalMCTSNode struct {
	State       CausalState
	Action      float64
	Parent      *CausalMCTSNode
	Children    []*CausalMCTSNode
	Visits      int
	TotalReward float64

	// Causal metadata mapped to your library
	TreatmentNode int   // Index of the Action variable in the state vector
	TargetNode    int   // Index of the Reward/Target variable in the state vector
	ControlNodes  []int // Indices of the confounding/control variables
	MinHistory    int   // Minimum rows required to run structural fits
}

// SelectBestChild incorporates interventional expectations into UCT selection
func (n *CausalMCTSNode) SelectBestChild(c float64, table causal.NodeTable) (*CausalMCTSNode, error) {
	if len(n.Children) == 0 {
		return nil, errors.New("no children to select from")
	}

	var bestChild *CausalMCTSNode
	bestScore := math.Inf(-1)

	for _, child := range n.Children {
		if child.Visits == 0 {
			// Eagerly select unexplored nodes to maintain expansion parity
			return child, nil
		}

		// 1. Classical Exploitation/Exploration
		exploitation := child.TotalReward / float64(child.Visits)
		exploration := c * math.Sqrt(math.Log(float64(n.Visits))/float64(child.Visits))
		uct := exploitation + exploration

		// 2. Interventional Bias using do-calculus
		// We calculate E[Target | do(Action)] over the historically observed table
		interventionalExpectation, err := table.DoExpectation(
			child.TreatmentNode,
			child.Action,
			child.ControlNodes...,
		)

		// If the causal engine cannot find a stable fit yet, fall back gracefully
		if err != nil {
			interventionalExpectation = 0.0
		}

		// Blend UCT with the causal expectation.
		// This biases selection towards actions with proven causal influence.
		causalScore := uct + (0.5 * interventionalExpectation)

		if causalScore > bestScore {
			bestScore = causalScore
			bestChild = child
		}
	}

	return bestChild, nil
}

// CounterfactualUpdate backpropagates virtual rewards to unexplored sibling nodes.
// When a real rollout completes, we use its final trajectory row to calculate
// what the reward *would have been* for alternative sibling actions.
func (n *CausalMCTSNode) CounterfactualUpdate(
	actualRow []float64,
	table causal.NodeTable,
	features []int,
	linear bool,
) {
	// Increment visits and update with actual reward
	n.Visits++
	actualReward := actualRow[n.TargetNode]
	n.TotalReward += actualReward

	if n.Parent == nil {
		return
	}

	// Iterate through sibling nodes to perform counterfactual retroaction
	for _, sibling := range n.Parent.Children {
		if sibling == n {
			continue // Already updated with real reward
		}

		// Abduct, Act, and Predict alternative sibling rewards
		_, counterfactualReward, _, err := table.AbductiveCounterfactual(
			features,
			linear,
			actualRow,
			n.TargetNode,
			n.TreatmentNode,
			sibling.Action, // Intervene with sibling's action
		)

		if err == nil && !math.IsNaN(counterfactualReward) && !math.IsInf(counterfactualReward, 0) {
			// Update sibling with virtual experience (weighted fraction of the visit value)
			sibling.Visits++
			sibling.TotalReward += counterfactualReward
		}
	}
}
