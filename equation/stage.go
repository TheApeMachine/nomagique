package equation

import (
	"io"

	"github.com/theapemachine/datura"
)

/*
ArtifactStage is an io stage that retains state on a datura artifact.
*/
type ArtifactStage interface {
	io.ReadWriter
	StageArtifact() *datura.Artifact
}
