package mcts

import "fmt"

const (
	GraphTreatmentColumn = 2
	GraphTargetColumn    = 4
)

var (
	GraphControlColumns = []int{0, 1, 3}
	GraphFeatureColumns = []int{0, 1, 2, 3}
)

/*
Graph is the directed, weighted artifact traversed by GraphState.
*/
type Graph interface {
	Roots() []string
	Targets(string) []string
	NodeValue(string) (float64, float64)
	EdgeValue(string, string) (float64, float64)
}

/*
GraphState traverses one weighted path through a Graph.
*/
type GraphState struct {
	graph        Graph
	current      string
	visited      map[string]bool
	row          []float64
	reward       float64
	intervention float64
}

func NewGraphState(graph Graph) (*GraphState, error) {
	if graph == nil || len(graph.Roots()) == 0 {
		return nil, fmt.Errorf("mcts: graph roots required")
	}

	return &GraphState{graph: graph, visited: make(map[string]bool)}, nil
}

func (graphState *GraphState) IsTerminal() bool {
	return len(graphState.targets()) == 0
}

func (graphState *GraphState) GetReward() float64 {
	return graphState.reward
}

func (graphState *GraphState) GetPossibleActions() []float64 {
	targets := graphState.targets()
	actions := make([]float64, len(targets))

	for index := range targets {
		actions[index] = float64(index)
	}

	return actions
}

func (graphState *GraphState) ApplyAction(action float64) State {
	targets := graphState.targets()
	index := int(action)

	if index < 0 || index >= len(targets) || action != float64(index) {
		panic(fmt.Sprintf("mcts: graph action %f unavailable", action))
	}

	target := targets[index]
	next := &GraphState{
		graph:        graphState.graph,
		current:      target,
		visited:      make(map[string]bool, len(graphState.visited)+1),
		intervention: 0,
	}

	for node := range graphState.visited {
		next.visited[node] = true
	}

	next.visited[target] = true
	targetValue, targetConfidence := graphState.graph.NodeValue(target)

	if graphState.current == "" {
		next.reward = targetValue * targetConfidence
		next.row = []float64{0, 0, 0, 0, next.reward}
		return next
	}

	sourceValue, sourceConfidence := graphState.graph.NodeValue(graphState.current)
	edgeValue, edgeConfidence := graphState.graph.EdgeValue(graphState.current, target)
	next.intervention = edgeValue
	next.reward = graphState.reward + edgeValue*edgeConfidence*targetValue*targetConfidence
	next.row = []float64{
		sourceValue,
		sourceConfidence,
		edgeValue,
		edgeConfidence,
		next.reward,
	}

	return next
}

func (graphState *GraphState) ToVector() []float64 {
	return append([]float64(nil), graphState.row...)
}

func (graphState *GraphState) GetInterventionLevel(_ float64) float64 {
	return graphState.intervention
}

func (graphState *GraphState) History() [][]float64 {
	rows := make([][]float64, 0)
	visited := make(map[string]bool)
	queue := append([]string(nil), graphState.graph.Roots()...)

	for len(queue) > 0 {
		source := queue[0]
		queue = queue[1:]

		if visited[source] {
			continue
		}

		visited[source] = true
		sourceValue, sourceConfidence := graphState.graph.NodeValue(source)

		for _, target := range graphState.graph.Targets(source) {
			edgeValue, edgeConfidence := graphState.graph.EdgeValue(source, target)
			targetValue, targetConfidence := graphState.graph.NodeValue(target)
			rows = append(rows, []float64{
				sourceValue,
				sourceConfidence,
				edgeValue,
				edgeConfidence,
				edgeValue * edgeConfidence * targetValue * targetConfidence,
			})
			queue = append(queue, target)
		}
	}

	return rows
}

func (graphState *GraphState) targets() []string {
	if graphState.current == "" {
		return graphState.graph.Roots()
	}

	targets := make([]string, 0)

	for _, target := range graphState.graph.Targets(graphState.current) {
		if !graphState.visited[target] {
			targets = append(targets, target)
		}
	}

	return targets
}
