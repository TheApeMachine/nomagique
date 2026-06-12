package geometry

import "io"

/*
Value is a fixed-width token layout compatible with phase encoding.
Only the first eight words participate in structuralPhaseMix.
*/
type Value [128]uint64

/*
NewValueFromBytes packs payload bytes into the first words for tests and
offline encoding. It does not replicate the full six token slab allocator.
*/
func NewValueFromBytes(payload []byte) ([]Value, error) {
	if len(payload) == 0 {
		return nil, io.ErrShortBuffer
	}

	var token Value

	for index, byteValue := range payload {
		wordIndex := index / 8

		if wordIndex >= 8 {
			break
		}

		shift := (index % 8) * 8
		token[wordIndex] |= uint64(byteValue) << shift
	}

	return []Value{token}, nil
}
