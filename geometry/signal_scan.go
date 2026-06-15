package geometry

/*
ScanZeroRun finds the longest contiguous run of zero bits across a slice of
uint64 words and returns the run's starting bit index and its length in bits.

All-zero words extend a run in O(1); mixed words fall back to a tight
per-bit walk so cross-word boundaries stay exact.
*/
func ScanZeroRun(words []uint64) (startBit, length int) {
	bestStart, bestLen := 0, 0
	curStart, curLen := 0, 0
	bitBase := 0

	for _, word := range words {
		if word == 0 {
			if curLen == 0 {
				curStart = bitBase
			}

			curLen += 64
			bitBase += 64

			continue
		}

		if word == ^uint64(0) {
			if curLen > bestLen {
				bestLen = curLen
				bestStart = curStart
			}

			curLen = 0
			bitBase += 64

			continue
		}

		for bit := 0; bit < 64; bit++ {
			if (word>>bit)&1 == 0 {
				if curLen == 0 {
					curStart = bitBase + bit
				}

				curLen++
			} else {
				if curLen > bestLen {
					bestLen = curLen
					bestStart = curStart
				}

				curLen = 0
			}
		}

		bitBase += 64
	}

	if curLen > bestLen {
		bestLen = curLen
		bestStart = curStart
	}

	return bestStart, bestLen
}

/*
ScanOneRun finds the longest contiguous run of one bits across uint64 words.
Where AND of two token regions produces a long one-run, both Values agree
densely at that position.
*/
func ScanOneRun(words []uint64) (startBit, length int) {
	bestStart, bestLen := 0, 0
	curStart, curLen := 0, 0
	bitBase := 0

	for _, word := range words {
		if word == ^uint64(0) {
			if curLen == 0 {
				curStart = bitBase
			}

			curLen += 64
			bitBase += 64

			continue
		}

		if word == 0 {
			if curLen > bestLen {
				bestLen = curLen
				bestStart = curStart
			}

			curLen = 0
			bitBase += 64

			continue
		}

		for bit := 0; bit < 64; bit++ {
			if (word>>bit)&1 == 1 {
				if curLen == 0 {
					curStart = bitBase + bit
				}

				curLen++
			} else {
				if curLen > bestLen {
					bestLen = curLen
					bestStart = curStart
				}

				curLen = 0
			}
		}

		bitBase += 64
	}

	if curLen > bestLen {
		bestLen = curLen
		bestStart = curStart
	}

	return bestStart, bestLen
}

/*
RunLabel maps a zero-run's starting bit position to a deterministic 16-bit
label hash. The start position encodes the structural fingerprint: two Values
that share structure at the same bit position produce the same label. Length
influences the hash to distinguish short incidental matches from long ones.
*/
func RunLabel(startBit, length int) uint16 {
	combined := uint32(startBit)<<9 | uint32(length&0x1FF)

	// FNV-1a fold to 16 bits.
	h := uint32(2166136261)
	h ^= combined & 0xFF
	h *= 16777619
	h ^= (combined >> 8) & 0xFF
	h *= 16777619
	h ^= (combined >> 16) & 0xFF
	h *= 16777619

	return uint16(h ^ (h >> 16))
}
