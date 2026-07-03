package hawkes

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique/statistic"
)

const bivariateParamCount = 7

/*
FitContext holds data-derived bounds for one bivariate Hawkes fit.
*/
type FitContext struct {
	SpanSec               float64
	MedianGapSec          float64
	GapLowerSec           float64
	GapUpperSec           float64
	GapCV                 float64
	TotalEvents           int
	EventsX               int
	EventsY               int
	MinFitEvents          int
	MinPerSide            int
	TradeWindow           time.Duration
	ScanSteps             int
	BranchScanSteps       int
	BranchFloor           float64
	BranchCeiling         float64
	BetaCandidates        []float64
	MuXFactors            []float64
	MuYFactors            []float64
	BranchSelfCandidates  []float64
	BranchCrossCandidates []float64
	LocalScales           []float64
}

type arrivalTune struct {
	totalEvents int
	eventsX     int
	eventsY     int
}

/*
NewFitContext derives fit bounds from an arrival stream.
*/
func NewFitContext(stream ArrivalStream, horizon time.Time) (FitContext, bool) {
	marked := stream.Marked()

	if len(marked) < 2 {
		return FitContext{}, false
	}

	span := stream.Span(horizon)

	if span <= 0 {
		return FitContext{}, false
	}

	gaps := stream.Gaps()

	if len(gaps) == 0 {
		return FitContext{}, false
	}

	medianGap, ok := statistic.MedianOf(gaps)

	if !ok || medianGap <= 0 {
		return FitContext{}, false
	}

	lowerGap, upperGap, quartileErr := quartiles(gaps)

	if quartileErr != nil {
		return FitContext{}, false
	}

	if upperGap <= lowerGap {
		upperGap = medianGap * (1 + 1/math.Sqrt(float64(len(gaps))))
		lowerGap = medianGap * (1 - 1/math.Sqrt(float64(len(gaps))))

		if lowerGap <= 0 {
			lowerGap = medianGap / 2
		}
	}

	gapSpread := upperGap - lowerGap
	gapCV := gapSpread / medianGap
	tune := arrivalTune{
		totalEvents: stream.buy.Len() + stream.sell.Len(),
		eventsX:     stream.buy.Len(),
		eventsY:     stream.sell.Len(),
	}
	localMin, localMax := tune.localScaleRange(gapCV)
	scanSteps := tune.scanSteps()
	betaCandidates, betaErr := logspace(
		1/upperGap, 1/lowerGap, scanSteps,
	)

	if betaErr != nil {
		return FitContext{}, false
	}

	branchSelfCandidates, selfErr := linspace(
		tune.branchFloor(),
		tune.branchCeiling()*tune.selfBranchShare(),
		tune.branchScanSteps(),
	)

	if selfErr != nil {
		return FitContext{}, false
	}

	branchCrossCandidates, crossErr := linspace(
		0, tune.branchCeiling(), tune.branchScanSteps(),
	)

	if crossErr != nil {
		return FitContext{}, false
	}

	localScales, scalesErr := linspace(localMin, localMax, scanSteps)

	if scalesErr != nil {
		return FitContext{}, false
	}

	muXFactors, muXErr := tune.muUncertaintyFactors(tune.eventsX)

	if muXErr != nil {
		return FitContext{}, false
	}

	muYFactors, muYErr := tune.muUncertaintyFactors(tune.eventsY)

	if muYErr != nil {
		return FitContext{}, false
	}

	return FitContext{
		SpanSec:      span,
		MedianGapSec: medianGap,
		GapLowerSec:  lowerGap,
		GapUpperSec:  upperGap,
		GapCV:        gapCV,
		TotalEvents:  tune.totalEvents,
		EventsX:      tune.eventsX,
		EventsY:      tune.eventsY,
		MinFitEvents: tune.minFitEvents(),
		MinPerSide:   tune.minEventsPerSide(),
		TradeWindow: tune.tradeWindowDuration(
			medianGap, tune.minFitEvents(),
		),
		ScanSteps:             tune.scanSteps(),
		BranchScanSteps:       tune.branchScanSteps(),
		BranchFloor:           tune.branchFloor(),
		BranchCeiling:         tune.branchCeiling(),
		BetaCandidates:        betaCandidates,
		MuXFactors:            muXFactors,
		MuYFactors:            muYFactors,
		BranchSelfCandidates:  branchSelfCandidates,
		BranchCrossCandidates: branchCrossCandidates,
		LocalScales:           localScales,
	}, true
}

/*
EnoughEvents reports whether the stream satisfies context minima.
*/
func (context FitContext) EnoughEvents(stream ArrivalStream) bool {
	total := stream.buy.Len() + stream.sell.Len()

	if total < context.MinFitEvents {
		return false
	}

	if stream.buy.Len() == 0 || stream.sell.Len() == 0 {
		return stream.buy.Len() >= context.MinFitEvents ||
			stream.sell.Len() >= context.MinFitEvents
	}

	if stream.buy.Len() < context.MinPerSide {
		return false
	}

	return stream.sell.Len() >= context.MinPerSide
}

/*
MuXStart returns the event-rate seed for stream x.
*/
func (context FitContext) MuXStart() float64 {
	muX := float64(context.EventsX) / context.SpanSec

	if muX <= 0 {
		return 1 / context.SpanSec
	}

	return muX
}

/*
MuYStart returns the event-rate seed for stream y.
*/
func (context FitContext) MuYStart() float64 {
	muY := float64(context.EventsY) / context.SpanSec

	if muY <= 0 {
		return 1 / context.SpanSec
	}

	return muY
}

/*
PoissonFit returns the no-excitation bivariate baseline for this stream.
*/
func (context FitContext) PoissonFit() BivariateFit {
	fit := BivariateFit{
		MuX:            context.MuXStart(),
		MuY:            context.MuYStart(),
		Beta:           1 / context.MedianGapSec,
		SpectralRadius: 0,
	}

	fit.IntensityX = fit.MuX
	fit.IntensityY = fit.MuY

	return fit
}

/*
CrossBranchCap returns the cross-excitation ceiling given a diagonal branch.
*/
func (context FitContext) CrossBranchCap(diagonalBranch float64) float64 {
	headroom := context.BranchCeiling - diagonalBranch

	if headroom <= 0 {
		return 0
	}

	return headroom
}

func (tune arrivalTune) minFitEvents() int {
	if tune.totalEvents <= 0 {
		return bivariateParamCount * 2
	}

	identifiability := bivariateParamCount * 2
	rateScaled := int(
		math.Ceil(
			math.Sqrt(float64(tune.totalEvents)) *
				math.Log(float64(tune.totalEvents)+math.E),
		),
	)

	if rateScaled < identifiability {
		return identifiability
	}

	if rateScaled > tune.totalEvents {
		return tune.totalEvents
	}

	return rateScaled
}

func (tune arrivalTune) minEventsPerSide() int {
	if tune.totalEvents <= 0 {
		return 2
	}

	perSide := int(math.Ceil(float64(tune.totalEvents) / 4))

	if perSide < 2 {
		return 2
	}

	return perSide
}

func (tune arrivalTune) scanSteps() int {
	if tune.totalEvents <= 1 {
		return 3
	}

	steps := int(math.Ceil(math.Log2(float64(tune.totalEvents))))

	if steps < 3 {
		return 3
	}

	return steps
}

func (tune arrivalTune) branchFloor() float64 {
	if tune.totalEvents <= 0 {
		return 0
	}

	return 1 / math.Sqrt(float64(tune.totalEvents))
}

func (tune arrivalTune) branchCeiling() float64 {
	margin := 1 / math.Sqrt(float64(tune.totalEvents))

	if margin >= criticalBranch {
		return criticalBranch / 2
	}

	return criticalBranch - margin
}

func (tune arrivalTune) branchScanSteps() int {
	base := tune.scanSteps()
	ratio := float64(tune.totalEvents) / float64(bivariateParamCount)

	if ratio <= float64(base) {
		return base
	}

	steps := int(math.Ceil(math.Sqrt(float64(base))))

	if steps < 3 {
		return 3
	}

	return steps
}

func (tune arrivalTune) selfBranchShare() float64 {
	if tune.totalEvents <= 0 {
		return 0
	}

	minorSide := float64(tune.eventsX)

	if tune.eventsY < tune.eventsX {
		minorSide = float64(tune.eventsY)
	}

	balance := minorSide / float64(tune.totalEvents)

	return balance + (1-balance)/math.Sqrt(float64(tune.totalEvents))
}

func (tune arrivalTune) tradeWindowDuration(
	medianGapSec float64,
	minFitEvents int,
) time.Duration {
	if medianGapSec <= 0 || minFitEvents <= 0 {
		return 0
	}

	memoryFactor := math.Log(float64(tune.totalEvents) + math.E)

	return time.Duration(
		medianGapSec * memoryFactor * float64(minFitEvents) * float64(time.Second),
	)
}

func (tune arrivalTune) muUncertaintyFactors(count int) ([]float64, error) {
	if count <= 0 {
		return []float64{1}, nil
	}

	spread := 2 / math.Sqrt(float64(count))

	return linspace(1-spread, 1+spread, tune.scanSteps())
}

func (tune arrivalTune) localScaleRange(gapCV float64) (minScale, maxScale float64) {
	if gapCV <= 0 {
		return 1 - 1/math.Sqrt(8), 1 + 1/math.Sqrt(8)
	}

	minScale = 1 - gapCV

	if minScale <= 0 {
		minScale = 1 / (1 + gapCV)
	}

	maxScale = 1 + gapCV

	return minScale, maxScale
}
