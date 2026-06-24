package geometry

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Translator builds a translation motor from displacement scalars.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Translator struct {
	artifact    *datura.Artifact
	multivector Multivector
	output      float64
}

/*
NewTranslator returns a translation stage wired from config attributes on the artifact.
*/
func NewTranslator(artifact *datura.Artifact) *Translator {
	return &Translator{
		artifact: artifact,
	}
}

func (translator *Translator) Read(payload []byte) (int, error) {
	state := datura.Acquire("translator-state", datura.APPJSON)

	if _, err := state.Write(translator.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometry-translator: state write failed",
			err,
		))
	}

	scalars := datura.Peek[[]float64](state, "batch")

	if len(scalars) < 3 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometry-translator: batch requires displacement",
			nil,
		))
	}

	translator.multivector.FromTranslation(scalars[0], scalars[1], scalars[2])
	translator.output = translator.multivector[MvScalar]
	translator.artifact.Poke(translator.output, "output", "value")
	translator.artifact.Poke(multivectorSlice(translator.multivector), "output", "motor")
	state.MergeOutput("value", translator.output)
	state.MergeOutput("motor", multivectorSlice(translator.multivector))
	state.Poke("output", "root")
	state.Poke([]string{"value", "motor"}, "inputs")

	return state.Read(payload)
}

func (translator *Translator) Write(payload []byte) (int, error) {
	translator.artifact.WithPayload(payload)
	return len(payload), nil
}

func (translator *Translator) Close() error {
	return nil
}
