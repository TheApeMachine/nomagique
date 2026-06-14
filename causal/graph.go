package causal

import "fmt"

/*
Graph is a directed acyclic structural model over node indices.
*/
type Graph struct {
	nodeCount int
	parents   [][]int
	children  [][]int
}

type graphVisit struct {
	node       int
	fromParent bool
}

/*
NewGraph builds a parent-list DAG and derives child adjacency.
*/
func NewGraph(parents [][]int) (Graph, error) {
	if len(parents) == 0 {
		return Graph{}, fmt.Errorf("causal: graph requires at least one node")
	}

	nodeCount := len(parents)
	children := make([][]int, nodeCount)

	for node := range parents {
		for _, parent := range parents[node] {
			if parent < 0 || parent >= nodeCount {
				return Graph{}, fmt.Errorf(
					"causal: parent %d of node %d outside graph width %d",
					parent, node, nodeCount,
				)
			}

			if parent == node {
				return Graph{}, fmt.Errorf("causal: node %d cannot be its own parent", node)
			}

			children[parent] = append(children[parent], node)
		}
	}

	return Graph{
		nodeCount: nodeCount,
		parents:   parents,
		children:  children,
	}, nil
}

/*
BackdoorAdmissible reports whether controls satisfy Pearl's backdoor criterion.
*/
func (graph Graph) BackdoorAdmissible(treatment, target int, controls []int) (bool, error) {
	if err := graph.validateNode(treatment); err != nil {
		return false, err
	}

	if err := graph.validateNode(target); err != nil {
		return false, err
	}

	for _, control := range controls {
		if err := graph.validateNode(control); err != nil {
			return false, err
		}
	}

	controlSet := nodeSet(controls)
	descendants := graph.Descendants(treatment)

	for control := range controlSet {
		if _, blocked := descendants[control]; blocked {
			return false, nil
		}
	}

	cutGraph := graph.withoutOutgoing(treatment)

	return cutGraph.mSeparated(treatment, target, controlSet), nil
}

/*
Descendants returns all nodes reachable downstream from node.
*/
func (graph Graph) Descendants(node int) map[int]struct{} {
	reachable := make(map[int]struct{})
	queue := append([]int(nil), graph.children[node]...)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if _, seen := reachable[current]; seen {
			continue
		}

		reachable[current] = struct{}{}
		queue = append(queue, graph.children[current]...)
	}

	return reachable
}

func (graph Graph) mSeparated(start, target int, conditioning map[int]struct{}) bool {
	reachable := graph.bayesBallReachable(start, conditioning)
	_, connected := reachable[target]

	return !connected
}

func (graph Graph) bayesBallReachable(start int, conditioning map[int]struct{}) map[int]struct{} {
	reached := map[int]struct{}{start: {}}
	seen := make(map[graphVisit]struct{})
	queue := []graphVisit{
		{node: start, fromParent: true},
		{node: start, fromParent: false},
	}

	for len(queue) > 0 {
		visit := queue[0]
		queue = queue[1:]

		if _, recorded := seen[visit]; recorded {
			continue
		}

		seen[visit] = struct{}{}
		reached[visit.node] = struct{}{}

		_, conditioned := conditioning[visit.node]

		if conditioned {
			if visit.fromParent {
				for _, parent := range graph.parents[visit.node] {
					queue = append(queue, graphVisit{node: parent, fromParent: true})
				}
			} else {
				for _, child := range graph.children[visit.node] {
					queue = append(queue, graphVisit{node: child, fromParent: false})
				}
			}

			continue
		}

		if visit.fromParent {
			for _, child := range graph.children[visit.node] {
				queue = append(queue, graphVisit{node: child, fromParent: false})
			}

			continue
		}

		for _, parent := range graph.parents[visit.node] {
			queue = append(queue, graphVisit{node: parent, fromParent: true})
		}

		for _, child := range graph.children[visit.node] {
			queue = append(queue, graphVisit{node: child, fromParent: false})
		}
	}

	return reached
}

func (graph Graph) withoutOutgoing(treatment int) Graph {
	parents := make([][]int, graph.nodeCount)
	children := make([][]int, graph.nodeCount)

	for node := range graph.parents {
		parents[node] = append([]int(nil), graph.parents[node]...)

		if node == treatment {
			continue
		}

		children[node] = append([]int(nil), graph.children[node]...)
	}

	for _, child := range graph.children[treatment] {
		filtered := parents[child][:0]

		for _, parent := range parents[child] {
			if parent == treatment {
				continue
			}

			filtered = append(filtered, parent)
		}

		parents[child] = filtered
	}

	return Graph{
		nodeCount: graph.nodeCount,
		parents:   parents,
		children:  children,
	}
}

func (graph Graph) validateNode(node int) error {
	if node < 0 || node >= graph.nodeCount {
		return fmt.Errorf("causal: node %d outside graph width %d", node, graph.nodeCount)
	}

	return nil
}

func nodeSet(nodes []int) map[int]struct{} {
	set := make(map[int]struct{}, len(nodes))

	for _, node := range nodes {
		set[node] = struct{}{}
	}

	return set
}
