package core

/*
BoundaryRegistry maps boundary tokens to the pipelines that produced them.
*/
type BoundaryRegistry struct {
	stacks      map[Float64][]*Pipeline
	stageStacks map[Float64][][]Number
}

/*
NewBoundaryRegistry instantiates an empty registry.
*/
func NewBoundaryRegistry() *BoundaryRegistry {
	return &BoundaryRegistry{
		stacks:      make(map[Float64][]*Pipeline),
		stageStacks: make(map[Float64][][]Number),
	}
}

/*
Register associates a boundary value with a pipeline for nested composition.
*/
func (registry *BoundaryRegistry) Register(
	boundary Float64, pipeline *Pipeline,
) {
	registry.stacks[boundary] = append(registry.stacks[boundary], pipeline)
	registry.stageStacks[boundary] = append(
		registry.stageStacks[boundary], pipeline.stages,
	)
}

/*
RegisterStages associates a boundary token with stage references only.
*/
func (registry *BoundaryRegistry) RegisterStages(
	boundary Float64, stages []Number,
) {
	copied := make([]Number, len(stages))
	copy(copied, stages)

	registry.stageStacks[boundary] = append(registry.stageStacks[boundary], copied)
}

/*
StagesFor returns the stages registered for a boundary token.
*/
func (registry *BoundaryRegistry) StagesFor(
	boundary Float64,
) ([]Number, bool) {
	stages, registered := registry.stageStacks[boundary]

	if !registered || len(stages) == 0 {
		return nil, false
	}

	return stages[len(stages)-1], true
}

/*
ExpandNumbers flattens nested boundary tokens into their staged dynamics.
*/
func (registry *BoundaryRegistry) ExpandNumbers(numbers []Number) []Number {
	expanded := make([]Number, 0, len(numbers))

	for _, number := range numbers {
		boundary, isBoundary := number.(Float64)

		if !isBoundary {
			expanded = append(expanded, number)
			continue
		}

		stages, registered := registry.StagesFor(boundary)

		if !registered {
			expanded = append(expanded, number)
			continue
		}

		expanded = append(expanded, stages...)
	}

	return expanded
}

/*
DefaultRegistry is the shared boundary-to-pipeline map.
*/
var DefaultRegistry = NewBoundaryRegistry()
