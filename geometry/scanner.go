package geometry

import (
	"fmt"
	"math"
	"math/cmplx"
	"sort"
	"sync"
	"time"
)

/*
CorpusEntry is a market state snapshot stored in the corpus, tagged with
what happened next (the outcome). The PhaseDial is pre-computed at insert
time so retrieval is pure similarity arithmetic with no re-encoding.
*/
type CorpusEntry[Outcome any] struct {
	Dial    PhaseDial
	Outcome Outcome
	At      time.Time
}

/*
CorpusMatch is a single result from a corpus similarity scan.
*/
type CorpusMatch[Outcome any] struct {
	Outcome    Outcome
	At         time.Time
	Similarity float64
}

/*
Corpus is an in-memory outcome-tagged collection of PhaseDial encodings. It
provides top-K signed interference retrieval and controlled global phase scans
against stored historical market states.

The corpus is safe for concurrent reads and writes.
*/
type Corpus[Outcome any] struct {
	mu         sync.RWMutex
	entries    []CorpusEntry[Outcome]
	maxSize    int
	dimensions int
	next       int
}

/*
NewCorpus creates a corpus with a maximum capacity. When full, the
oldest entries are evicted to make room for new observations.
*/
func NewCorpus[Outcome any](maxSize int) (*Corpus[Outcome], error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("geometry: corpus capacity must be positive")
	}

	return &Corpus[Outcome]{
		entries: make([]CorpusEntry[Outcome], 0, maxSize),
		maxSize: maxSize,
	}, nil
}

/*
Insert adds a new state observation to the corpus. If the corpus is
at capacity, the oldest entry is evicted. The dial is copied so callers cannot
mutate a corpus entry after insertion.
*/
func (corpus *Corpus[Outcome]) Insert(entry CorpusEntry[Outcome]) error {
	if err := corpus.validate(entry.Dial); err != nil {
		return fmt.Errorf("geometry: insert corpus entry: %w", err)
	}

	entry.Dial = entry.Dial.CopyAndNormalize()
	corpus.mu.Lock()
	defer corpus.mu.Unlock()

	if corpus.dimensions == 0 {
		corpus.dimensions = len(entry.Dial)
	}

	if len(entry.Dial) != corpus.dimensions {
		return fmt.Errorf(
			"geometry: corpus dial has %d dimensions, expected %d",
			len(entry.Dial), corpus.dimensions,
		)
	}

	if len(corpus.entries) < corpus.maxSize {
		corpus.entries = append(corpus.entries, entry)

		return nil
	}

	corpus.entries[corpus.next] = entry
	corpus.next = (corpus.next + 1) % corpus.maxSize

	return nil
}

/*
Scan returns the top-K corpus entries ranked by cosine similarity to
the query PhaseDial, sorted by descending similarity.
*/
func (corpus *Corpus[Outcome]) Scan(
	queryDial PhaseDial, topK int,
) ([]CorpusMatch[Outcome], error) {
	responses, err := corpus.ScanPhases(queryDial, []float64{0}, topK)

	if err != nil {
		return nil, err
	}

	return responses[0], nil
}

/*
ScanExcluding returns the top-K entries excluding any that match the
given timestamps. Used to prevent self-matching when the query state
is also in the corpus.
*/
func (corpus *Corpus[Outcome]) ScanExcluding(
	queryDial PhaseDial, topK int, excludeTimes ...time.Time,
) ([]CorpusMatch[Outcome], error) {
	excluded := make(map[int64]bool, len(excludeTimes))

	for _, excludeTime := range excludeTimes {
		excluded[excludeTime.UnixNano()] = true
	}

	responses, err := corpus.scanPhases(queryDial, []float64{0}, topK, excluded)

	if err != nil {
		return nil, err
	}

	return responses[0], nil
}

/*
ScanPhases evaluates the corpus at each requested global phase rotation. The
complex overlaps are calculated once, then analytically rotated, preserving
both constructive and destructive interference without repeatedly allocating
rotated fingerprints.
*/
func (corpus *Corpus[Outcome]) ScanPhases(
	queryDial PhaseDial, angles []float64, topK int,
) ([][]CorpusMatch[Outcome], error) {
	return corpus.scanPhases(queryDial, angles, topK, nil)
}

/*
ScanPhasesExcluding evaluates a controlled phase path while excluding entries
at the supplied timestamps. This keeps a resident query from selecting itself
at every constructive phase when a caller scans an online corpus.
*/
func (corpus *Corpus[Outcome]) ScanPhasesExcluding(
	queryDial PhaseDial,
	angles []float64,
	topK int,
	excludeTimes ...time.Time,
) ([][]CorpusMatch[Outcome], error) {
	excluded := make(map[int64]bool, len(excludeTimes))

	for _, excludeTime := range excludeTimes {
		excluded[excludeTime.UnixNano()] = true
	}

	return corpus.scanPhases(queryDial, angles, topK, excluded)
}

/*
Size returns the current number of entries in the corpus.
*/
func (corpus *Corpus[Outcome]) Size() int {
	corpus.mu.RLock()
	defer corpus.mu.RUnlock()

	return len(corpus.entries)
}

/*
scanPhases validates a phase path, snapshots the resident complex overlaps,
and ranks their signed response at each requested angle.
*/
func (corpus *Corpus[Outcome]) scanPhases(
	queryDial PhaseDial, angles []float64, topK int, excluded map[int64]bool,
) ([][]CorpusMatch[Outcome], error) {
	if err := corpus.validate(queryDial); err != nil {
		return nil, fmt.Errorf("geometry: scan corpus: %w", err)
	}

	if topK <= 0 {
		return nil, fmt.Errorf("geometry: scan count must be positive")
	}

	if len(angles) == 0 {
		return nil, fmt.Errorf("geometry: phase scan requires at least one angle")
	}

	for _, angle := range angles {
		if math.IsNaN(angle) || math.IsInf(angle, 0) {
			return nil, fmt.Errorf("geometry: phase scan angle must be finite")
		}
	}

	corpus.mu.RLock()

	if corpus.dimensions != 0 && len(queryDial) != corpus.dimensions {
		corpus.mu.RUnlock()

		return nil, fmt.Errorf(
			"geometry: query dial has %d dimensions, expected %d",
			len(queryDial), corpus.dimensions,
		)
	}

	entries := make([]CorpusEntry[Outcome], 0, len(corpus.entries))
	overlaps := make([]complex128, 0, len(corpus.entries))

	for _, entry := range corpus.entries {
		if excluded[entry.At.UnixNano()] {
			continue
		}

		entries = append(entries, entry)
		overlaps = append(overlaps, queryDial.Overlap(entry.Dial))
	}

	corpus.mu.RUnlock()

	responses := make([][]CorpusMatch[Outcome], len(angles))
	matches := make([]CorpusMatch[Outcome], len(entries))

	for angleIndex, angle := range angles {
		rotation := cmplx.Rect(1, -angle)

		for entryIndex, entry := range entries {
			matches[entryIndex] = CorpusMatch[Outcome]{
				Outcome:    entry.Outcome,
				At:         entry.At,
				Similarity: real(overlaps[entryIndex] * rotation),
			}
		}

		corpus.rank(matches)
		responses[angleIndex] = append(
			[]CorpusMatch[Outcome](nil),
			matches[:min(topK, len(matches))]...,
		)
	}

	return responses, nil
}

/*
validate rejects fingerprints whose complex response would be undefined.
*/
func (corpus *Corpus[Outcome]) validate(dial PhaseDial) error {
	if len(dial) == 0 || dial.norm() == 0 {
		return fmt.Errorf("phase dial must contain nonzero amplitude")
	}

	for _, component := range dial {
		if math.IsNaN(real(component)) || math.IsNaN(imag(component)) ||
			math.IsInf(real(component), 0) || math.IsInf(imag(component), 0) {
			return fmt.Errorf("phase dial must contain finite components")
		}
	}

	return nil
}

/*
rank orders signed phase responses and uses observation time to make equal
responses independent of insertion order.
*/
func (corpus *Corpus[Outcome]) rank(matches []CorpusMatch[Outcome]) {
	sort.Slice(matches, func(leftIndex, rightIndex int) bool {
		left := matches[leftIndex]
		right := matches[rightIndex]

		if left.Similarity != right.Similarity {
			return left.Similarity > right.Similarity
		}

		return left.At.Before(right.At)
	})
}
