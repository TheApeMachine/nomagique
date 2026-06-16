package learning

import "gonum.org/v1/gonum/mat"

type resonanceWorkspace struct {
	xCol                *mat.Dense
	yCol                *mat.Dense
	predictions         []*mat.Dense
	errors              []*mat.Dense
	grads               []*mat.Dense
	topPrior            *mat.Dense
	temporalError       *mat.Dense
	temporalSignal      *mat.Dense
	temporalWeightedErr *mat.Dense
	taskPred            *mat.Dense
	taskError           *mat.Dense
	taskSignal          *mat.Dense
	taskCorrection      *mat.Dense
	weightedErr         []*mat.Dense
	belowSignal         []*mat.Dense
	correction          []*mat.Dense
	localSignal         []*mat.Dense
	weightUpdate        []*mat.Dense
	recProposal         []*mat.Dense
	recError            []*mat.Dense
	recSignal           []*mat.Dense
	recUpdate           []*mat.Dense
	taskUpdate          *mat.Dense
	temporalUpdate      *mat.Dense
	bottomUp            []*mat.Dense
	topDown             []*mat.Dense
	mergeTD             []*mat.Dense
	mergeBU             []*mat.Dense
	savedStates         []*mat.Dense
	stepBuf             []*mat.Dense
	reconPred           *mat.Dense
	reconDiff           *mat.Dense
}

func newResonanceWorkspace(arch []int, targetDim int) *resonanceWorkspace {
	numLinks := len(arch) - 1
	topDim := arch[len(arch)-1]

	workspace := &resonanceWorkspace{
		xCol:                mat.NewDense(arch[0], 1, nil),
		predictions:         make([]*mat.Dense, numLinks),
		errors:              make([]*mat.Dense, numLinks),
		grads:               make([]*mat.Dense, len(arch)),
		topPrior:            mat.NewDense(topDim, 1, nil),
		temporalError:       mat.NewDense(topDim, 1, nil),
		temporalSignal:      mat.NewDense(topDim, 1, nil),
		temporalWeightedErr: mat.NewDense(topDim, 1, nil),
		bottomUp:            make([]*mat.Dense, len(arch)),
		topDown:             make([]*mat.Dense, len(arch)),
		mergeTD:             make([]*mat.Dense, len(arch)),
		mergeBU:             make([]*mat.Dense, len(arch)),
		savedStates:         make([]*mat.Dense, len(arch)),
		stepBuf:             make([]*mat.Dense, len(arch)),
		weightedErr:         make([]*mat.Dense, numLinks),
		belowSignal:         make([]*mat.Dense, numLinks),
		correction:          make([]*mat.Dense, len(arch)),
		localSignal:         make([]*mat.Dense, numLinks),
		weightUpdate:        make([]*mat.Dense, numLinks),
		recProposal:         make([]*mat.Dense, numLinks),
		recError:            make([]*mat.Dense, numLinks),
		recSignal:           make([]*mat.Dense, numLinks),
		recUpdate:           make([]*mat.Dense, numLinks),
		reconPred:           mat.NewDense(arch[0], 1, nil),
		reconDiff:           mat.NewDense(arch[0], 1, nil),
		temporalUpdate:      mat.NewDense(topDim, topDim, nil),
	}

	for layerIndex, layerDim := range arch {
		workspace.bottomUp[layerIndex] = mat.NewDense(layerDim, 1, nil)
		workspace.topDown[layerIndex] = mat.NewDense(layerDim, 1, nil)
		workspace.mergeTD[layerIndex] = mat.NewDense(layerDim, 1, nil)
		workspace.mergeBU[layerIndex] = mat.NewDense(layerDim, 1, nil)
		workspace.savedStates[layerIndex] = mat.NewDense(layerDim, 1, nil)
		workspace.stepBuf[layerIndex] = mat.NewDense(layerDim, 1, nil)
		workspace.correction[layerIndex] = mat.NewDense(layerDim, 1, nil)

		if layerIndex > 0 {
			workspace.grads[layerIndex] = mat.NewDense(layerDim, 1, nil)
		}
	}

	for linkIndex := 0; linkIndex < numLinks; linkIndex++ {
		rowDim := arch[linkIndex]
		colDim := arch[linkIndex+1]

		workspace.predictions[linkIndex] = mat.NewDense(rowDim, 1, nil)
		workspace.errors[linkIndex] = mat.NewDense(rowDim, 1, nil)
		workspace.weightedErr[linkIndex] = mat.NewDense(rowDim, 1, nil)
		workspace.belowSignal[linkIndex] = mat.NewDense(rowDim, 1, nil)
		workspace.localSignal[linkIndex] = mat.NewDense(rowDim, 1, nil)
		workspace.weightUpdate[linkIndex] = mat.NewDense(rowDim, colDim, nil)
		workspace.recProposal[linkIndex] = mat.NewDense(colDim, 1, nil)
		workspace.recError[linkIndex] = mat.NewDense(colDim, 1, nil)
		workspace.recSignal[linkIndex] = mat.NewDense(colDim, 1, nil)
		workspace.recUpdate[linkIndex] = mat.NewDense(colDim, rowDim, nil)
	}

	if targetDim > 0 {
		workspace.yCol = mat.NewDense(targetDim, 1, nil)
		workspace.taskPred = mat.NewDense(targetDim, 1, nil)
		workspace.taskError = mat.NewDense(targetDim, 1, nil)
		workspace.taskSignal = mat.NewDense(targetDim, 1, nil)
		workspace.taskCorrection = mat.NewDense(topDim, 1, nil)
		workspace.taskUpdate = mat.NewDense(targetDim, topDim, nil)
	}

	return workspace
}
