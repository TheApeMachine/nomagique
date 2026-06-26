package nomagique

import (
	"bytes"
	"errors"
	"io"

	"github.com/theapemachine/datura"
)

/*
WriteArtifact writes one packed artifact frame to destination.
*/
func WriteArtifact(destination io.Writer, artifact *datura.Artifact) (int64, error) {
	if artifact == nil || !artifact.IsValid() {
		return 0, errors.New("nomagique: invalid artifact")
	}

	wire := artifact.Pack()

	if len(wire) == 0 {
		return 0, errors.New("nomagique: empty artifact frame")
	}

	written, err := destination.Write(wire)

	return int64(written), err
}

/*
ReadArtifact reads one packed artifact frame from source into artifact.
*/
func ReadArtifact(source io.Reader, artifact *datura.Artifact) error {
	if artifact == nil {
		return errors.New("nomagique: nil artifact")
	}

	var frame bytes.Buffer
	chunk := make([]byte, 262144)

	for {
		readCount, err := source.Read(chunk)

		if readCount > 0 {
			frame.Write(chunk[:readCount])
		}

		if err == io.EOF {
			break
		}

		if err != nil && err != io.ErrShortBuffer {
			return err
		}

		if readCount == 0 {
			break
		}
	}

	if frame.Len() == 0 {
		return io.EOF
	}

	_, err := artifact.Unpack(frame.Bytes())

	return err
}

/*
RoundTripArtifact sends artifact through a stage and replaces it with the result.
*/
func RoundTripArtifact(artifact *datura.Artifact, stage io.ReadWriter) error {
	if _, err := WriteArtifact(stage, artifact); err != nil {
		return err
	}

	if err := ReadArtifact(stage, artifact); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}
