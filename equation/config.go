package equation

import "github.com/theapemachine/datura"

/*
DecayConfig returns a decay stage config artifact with schema inputs.
*/
func DecayConfig() *datura.Artifact {
	return datura.Acquire("decay", datura.APPJSON).Poke(DecayInputKeys, "inputs")
}

/*
FlowConfig returns a flow stage config artifact with schema inputs.
*/
func FlowConfig() *datura.Artifact {
	return datura.Acquire("flow", datura.APPJSON).Poke(FlowInputKeys, "inputs")
}

/*
DepthConfig returns a depth stage config artifact with schema inputs.
*/
func DepthConfig() *datura.Artifact {
	return datura.Acquire("depth", datura.APPJSON).Poke(DepthInputKeys, "inputs")
}

/*
CohortConfig returns a cohort stage config artifact with schema inputs.
*/
func CohortConfig() *datura.Artifact {
	return datura.Acquire("cohort", datura.APPJSON).Poke(CohortInputKeys, "inputs")
}

/*
BookflowConfig returns a bookflow stage config artifact with schema inputs.
*/
func BookflowConfig() *datura.Artifact {
	return datura.Acquire("bookflow", datura.APPJSON).Poke(BookflowInputKeys, "inputs")
}

/*
BookQualityConfig returns a book-quality stage config artifact with schema inputs.
*/
func BookQualityConfig() *datura.Artifact {
	return datura.Acquire("book-quality", datura.APPJSON).Poke(BookQualityInputKeys, "inputs")
}

/*
ConvictionConfig returns a conviction stage config artifact with schema inputs.
*/
func ConvictionConfig() *datura.Artifact {
	return datura.Acquire("conviction", datura.APPJSON).Poke(ConvictionInputKeys, "inputs")
}

/*
FluidflowConfig returns a fluidflow stage config artifact with schema inputs.
*/
func FluidflowConfig() *datura.Artifact {
	return datura.Acquire("fluidflow", datura.APPJSON).Poke(FluidflowInputKeys, "inputs")
}

/*
ManifoldConfig returns a manifold stage config artifact with schema inputs.
*/
func ManifoldConfig() *datura.Artifact {
	return datura.Acquire("manifold", datura.APPJSON).Poke(ManifoldInputKeys, "inputs")
}

/*
CausalStoryConfig returns a causal-story stage config artifact.
*/
func CausalStoryConfig() *datura.Artifact {
	return datura.Acquire("causal-story", datura.APPJSON)
}
