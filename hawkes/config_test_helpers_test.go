package hawkes

import "github.com/theapemachine/datura"

func momentConfigArtifact(params BivariateParams, momentR, momentS float64) *datura.Artifact {
	return datura.Acquire("hawkes-moment-config", datura.APPJSON).
		Poke(params.MuX, "config", "muX").
		Poke(params.MuY, "config", "muY").
		Poke(params.AlphaXX, "config", "alphaXX").
		Poke(params.AlphaXY, "config", "alphaXY").
		Poke(params.AlphaYX, "config", "alphaYX").
		Poke(params.AlphaYY, "config", "alphaYY").
		Poke(params.Beta, "config", "beta").
		Poke(momentR, "config", "momentR").
		Poke(momentS, "config", "momentS")
}

func fitConfigArtifact(horizonUnixNano float64, prior BivariateFit) *datura.Artifact {
	return datura.Acquire("hawkes-fit-config", datura.APPJSON).
		Poke(horizonUnixNano, "config", "horizonUnixNano").
		Poke(prior.MuX, "config", "muX").
		Poke(prior.MuY, "config", "muY").
		Poke(prior.AlphaXX, "config", "alphaXX").
		Poke(prior.AlphaXY, "config", "alphaXY").
		Poke(prior.AlphaYX, "config", "alphaYX").
		Poke(prior.AlphaYY, "config", "alphaYY").
		Poke(prior.Beta, "config", "beta").
		Poke(prior.IntensityX, "config", "intensityX").
		Poke(prior.IntensityY, "config", "intensityY").
		Poke(prior.SpectralRadius, "config", "spectralRadius")
}
