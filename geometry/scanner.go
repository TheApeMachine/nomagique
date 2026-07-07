package geometry

import (
	"sort"
	"sync"
	"time"
)

/*
CorpusEntry is a market state snapshot stored in the corpus, tagged with
what happened next (the outcome). The PhaseDial is pre-computed at insert
time so retrieval is pure similarity arithmetic with no re-encoding.
*/
type CorpusEntry struct {
	Dial    PhaseDial
	Outcome CorpusOutcome
	At      time.Time
}

/*
CorpusOutcome records what happened after the state was observed.
ReturnBps is the realized return in basis points over the horizon.
Category is the eventual market regime label (for display only).
*/
type CorpusOutcome struct {
	ReturnBps float64
	Category  string
	Horizon   time.Duration
}

/*
CorpusMatch is a single result from a corpus similarity scan.
*/
type CorpusMatch struct {
	Entry      CorpusEntry
	Similarity float64
}

/*
Corpus is an in-memory outcome-tagged collection of PhaseDial encodings.
It provides top-K cosine similarity retrieval against stored historical
market states so the decision engine can ask "how closely does this
exact current state resemble historical states?" and read their outcomes.

The corpus is safe for concurrent reads and writes.
*/
type Corpus struct {
	mu      sync.RWMutex
	entries []CorpusEntry
	maxSize int
}

/*
NewCorpus creates a corpus with a maximum capacity. When full, the
oldest entries are evicted to make room for new observations.
*/
func NewCorpus(maxSize int) *Corpus {
	if maxSize <= 0 {
		maxSize = 10000
	}

	return &Corpus{
		entries: make([]CorpusEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

/*
Insert adds a new state observation to the corpus. If the corpus is
at capacity, the oldest entry is evicted.
*/
func (corpus *Corpus) Insert(entry CorpusEntry) {
	corpus.mu.Lock()
	defer corpus.mu.Unlock()

	if len(corpus.entries) >= corpus.maxSize {
		copy(corpus.entries, corpus.entries[1:])
		corpus.entries = corpus.entries[:len(corpus.entries)-1]
	}

	corpus.entries = append(corpus.entries, entry)
}

/*
Scan returns the top-K corpus entries ranked by cosine similarity to
the query PhaseDial, sorted by descending similarity.
*/
func (corpus *Corpus) Scan(queryDial PhaseDial, topK int) []CorpusMatch {
	corpus.mu.RLock()

	matches := make([]CorpusMatch, 0, len(corpus.entries))

	for _, entry := range corpus.entries {
		similarity := queryDial.Similarity(entry.Dial)

		matches = append(matches, CorpusMatch{
			Entry:      entry,
			Similarity: similarity,
		})
	}

	corpus.mu.RUnlock()

	sort.Slice(matches, func(leftIndex, rightIndex int) bool {
		return matches[leftIndex].Similarity > matches[rightIndex].Similarity
	})

	if topK > 0 && topK < len(matches) {
		matches = matches[:topK]
	}

	return matches
}

/*
ScanExcluding returns the top-K entries excluding any that match the
given timestamps. Used to prevent self-matching when the query state
is also in the corpus.
*/
func (corpus *Corpus) ScanExcluding(
	queryDial PhaseDial, topK int, excludeTimes ...time.Time,
) []CorpusMatch {
	excluded := make(map[int64]bool, len(excludeTimes))

	for _, excludeTime := range excludeTimes {
		excluded[excludeTime.UnixNano()] = true
	}

	corpus.mu.RLock()

	matches := make([]CorpusMatch, 0, len(corpus.entries))

	for _, entry := range corpus.entries {
		if excluded[entry.At.UnixNano()] {
			continue
		}

		similarity := queryDial.Similarity(entry.Dial)

		matches = append(matches, CorpusMatch{
			Entry:      entry,
			Similarity: similarity,
		})
	}

	corpus.mu.RUnlock()

	sort.Slice(matches, func(leftIndex, rightIndex int) bool {
		return matches[leftIndex].Similarity > matches[rightIndex].Similarity
	})

	if topK > 0 && topK < len(matches) {
		matches = matches[:topK]
	}

	return matches
}

/*
Size returns the current number of entries in the corpus.
*/
func (corpus *Corpus) Size() int {
	corpus.mu.RLock()
	defer corpus.mu.RUnlock()

	return len(corpus.entries)
}

/*
WeightedOutcome computes a similarity-weighted average return from
the top-K matches. The weighting ensures closer historical analogues
dominate the predicted outcome.
*/
func WeightedOutcome(matches []CorpusMatch) float64 {
	if len(matches) == 0 {
		return 0
	}

	weightedSum := 0.0
	totalWeight := 0.0

	for _, match := range matches {
		weight := match.Similarity

		if weight <= 0 {
			continue
		}

		weightedSum += weight * match.Entry.Outcome.ReturnBps
		totalWeight += weight
	}

	if totalWeight <= 0 {
		return 0
	}

	return weightedSum / totalWeight
}
