#import "solver_private.h"

/*
This file mirrors the gonum learning.ResonanceManifold math on the GPU. Each
helper drives the small kernels in resonance.metal over the flat state buffers.
Layer vectors are tiny, so readback of scalar reductions per line-search halving
is cheap and keeps the control flow identical to the reference.
*/

@implementation ResonanceSolver (Compute)

// ---- low-level convenience wrappers --------------------------------------

- (void)copyN:(uint32_t)n
          src:(id<MTLBuffer>)src srcOff:(uint32_t)srcOff
          dst:(id<MTLBuffer>)dst dstOff:(uint32_t)dstOff {
    [self dispatch1D:self.pVecCopy
             buffers:@[src, dst, [self constantUint:n]]
             offsets:@[@((NSUInteger)srcOff * sizeof(float)), @((NSUInteger)dstOff * sizeof(float)), @0]
         threadCount:n];
}

// out[rows] = act(M * x)
- (void)gemv:(BOOL)tanhAct
      matrix:(id<MTLBuffer>)matrix matOff:(uint32_t)matOff
           x:(id<MTLBuffer>)x xOff:(uint32_t)xOff
         out:(id<MTLBuffer>)out outOff:(uint32_t)outOff
        rows:(uint32_t)rows cols:(uint32_t)cols {
    id<MTLComputePipelineState> pipeline = tanhAct ? self.pGemvTanh : self.pGemv;
    [self dispatch1D:pipeline
             buffers:@[matrix, x, out, [self constantUint:rows], [self constantUint:cols]]
             offsets:@[@((NSUInteger)matOff * sizeof(float)), @((NSUInteger)xOff * sizeof(float)),
                       @((NSUInteger)outOff * sizeof(float)), @0, @0]
         threadCount:rows];
}

// out[cols] = M^T * x[rows]
- (void)gemvTranspose:(id<MTLBuffer>)matrix matOff:(uint32_t)matOff
                    x:(id<MTLBuffer>)x xOff:(uint32_t)xOff
                  out:(id<MTLBuffer>)out outOff:(uint32_t)outOff
                 rows:(uint32_t)rows cols:(uint32_t)cols {
    [self dispatch1D:self.pGemvTranspose
             buffers:@[matrix, x, out, [self constantUint:rows], [self constantUint:cols]]
             offsets:@[@((NSUInteger)matOff * sizeof(float)), @((NSUInteger)xOff * sizeof(float)),
                       @((NSUInteger)outOff * sizeof(float)), @0, @0]
         threadCount:cols];
}

- (void)binaryOp:(id<MTLComputePipelineState>)pipeline
               a:(id<MTLBuffer>)a aOff:(uint32_t)aOff
               b:(id<MTLBuffer>)b bOff:(uint32_t)bOff
             out:(id<MTLBuffer>)out outOff:(uint32_t)outOff
               n:(uint32_t)n {
    [self dispatch1D:pipeline
             buffers:@[a, b, out, [self constantUint:n]]
             offsets:@[@((NSUInteger)aOff * sizeof(float)), @((NSUInteger)bOff * sizeof(float)),
                       @((NSUInteger)outOff * sizeof(float)), @0]
         threadCount:n];
}

- (void)scale:(id<MTLBuffer>)src srcOff:(uint32_t)srcOff
          out:(id<MTLBuffer>)out outOff:(uint32_t)outOff
       scalar:(float)scalar n:(uint32_t)n {
    [self dispatch1D:self.pVecScale
             buffers:@[src, out, [self constantFloat:scalar], [self constantUint:n]]
             offsets:@[@((NSUInteger)srcOff * sizeof(float)), @((NSUInteger)outOff * sizeof(float)), @0, @0]
         threadCount:n];
}

- (void)axpy:(id<MTLBuffer>)src srcOff:(uint32_t)srcOff
         out:(id<MTLBuffer>)out outOff:(uint32_t)outOff
      scalar:(float)scalar n:(uint32_t)n {
    [self dispatch1D:self.pVecAxpy
             buffers:@[src, out, [self constantFloat:scalar], [self constantUint:n]]
             offsets:@[@((NSUInteger)srcOff * sizeof(float)), @((NSUInteger)outOff * sizeof(float)), @0, @0]
         threadCount:n];
}

- (void)clampVec:(id<MTLBuffer>)src srcOff:(uint32_t)srcOff
             out:(id<MTLBuffer>)out outOff:(uint32_t)outOff
           limit:(float)limit n:(uint32_t)n {
    [self dispatch1D:self.pVecClamp
             buffers:@[src, out, [self constantFloat:limit], [self constantUint:n]]
             offsets:@[@((NSUInteger)srcOff * sizeof(float)), @((NSUInteger)outOff * sizeof(float)), @0, @0]
         threadCount:n];
}

- (void)tanhDeriv:(id<MTLBuffer>)p pOff:(uint32_t)pOff
              out:(id<MTLBuffer>)out outOff:(uint32_t)outOff n:(uint32_t)n {
    [self dispatch1D:self.pTanhDeriv
             buffers:@[p, out, [self constantUint:n]]
             offsets:@[@((NSUInteger)pOff * sizeof(float)), @((NSUInteger)outOff * sizeof(float)), @0]
         threadCount:n];
}

- (float)dot:(id<MTLBuffer>)a aOff:(uint32_t)aOff
           b:(id<MTLBuffer>)b bOff:(uint32_t)bOff n:(uint32_t)n {
    [self dispatchReduce:self.pReduceDot
                 buffers:@[a, b, self.bufScalar, [self constantUint:n]]
                 offsets:@[@((NSUInteger)aOff * sizeof(float)), @((NSUInteger)bOff * sizeof(float)), @0, @0]];
    return ((const float *)self.bufScalar.contents)[0];
}

- (float)absSum:(id<MTLBuffer>)a aOff:(uint32_t)aOff n:(uint32_t)n {
    [self dispatchReduce:self.pReduceAbsSum
                 buffers:@[a, self.bufScalar, [self constantUint:n]]
                 offsets:@[@((NSUInteger)aOff * sizeof(float)), @0, @0]];
    return ((const float *)self.bufScalar.contents)[0];
}

- (void)gradClip:(id<MTLBuffer>)g gOff:(uint32_t)gOff clip:(float)clip n:(uint32_t)n {
    [self dispatchReduce:self.pGradClip
                 buffers:@[g, [self constantFloat:clip], [self constantUint:n]]
                 offsets:@[@((NSUInteger)gOff * sizeof(float)), @0, @0]];
}

// ---- temporal prior helper: topPrior = tanh(A * prevTop) into bufTmpC -----

- (void)computeTopPriorInto:(id<MTLBuffer>)out outOff:(uint32_t)outOff {
    uint32_t topDim = self.arch[self.archLen - 1u];
    [self gemv:YES matrix:self.bufA matOff:0 x:self.bufPrevTop xOff:0
           out:out outOff:outOff rows:topDim cols:topDim];
}

// ---- predictAdjacentLayers ------------------------------------------------

- (void)predictAdjacentLayers {
    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l];
        uint32_t cols = self.arch[l + 1u];
        uint32_t predOff = self.predOffset[l];

        // prediction[l] = tanh(W[l] * z[l+1])
        [self gemv:YES matrix:self.bufW matOff:self.wOffset[l]
               x:self.bufZ xOff:self.zOffset[l + 1u]
             out:self.bufPred outOff:predOff rows:rows cols:cols];

        // error[l] = z[l] - prediction[l]
        [self binaryOp:self.pVecSub
                     a:self.bufZ aOff:self.zOffset[l]
                     b:self.bufPred bOff:predOff
                   out:self.bufErr outOff:predOff n:rows];
    }
}

// ---- energy ---------------------------------------------------------------

- (float)energy {
    [self predictAdjacentLayers];
    double energy = 0.0;

    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l];
        uint32_t predOff = self.predOffset[l];

        if (self.config.use_precision) {
            // weighted = precision .* error ; energy += 0.5 * weighted . error
            [self binaryOp:self.pVecMulElem
                         a:self.bufPrecision aOff:predOff
                         b:self.bufErr bOff:predOff
                       out:self.bufTmpA outOff:0 n:rows];
            energy += 0.5 * (double)[self dot:self.bufTmpA aOff:0 b:self.bufErr bOff:predOff n:rows];
        } else {
            energy += 0.5 * (double)[self dot:self.bufErr aOff:predOff b:self.bufErr bOff:predOff n:rows];
        }
    }

    uint32_t topDim = self.arch[self.archLen - 1u];
    uint32_t topOff = self.zOffset[self.archLen - 1u];

    if (self.hasPrevTop) {
        // topPrior into tmpA ; temporalError = z[top] - topPrior into tmpB
        [self computeTopPriorInto:self.bufTmpA outOff:0];
        [self binaryOp:self.pVecSub
                     a:self.bufZ aOff:topOff
                     b:self.bufTmpA bOff:0
                   out:self.bufTmpB outOff:0 n:topDim];

        if (self.config.use_precision) {
            [self binaryOp:self.pVecMulElem
                         a:self.bufTemporalPrec aOff:0
                         b:self.bufTmpB bOff:0
                       out:self.bufTmpC outOff:0 n:topDim];
            energy += 0.5 * (double)self.config.temporal_weight *
                      (double)[self dot:self.bufTmpC aOff:0 b:self.bufTmpB bOff:0 n:topDim];
        } else {
            energy += 0.5 * (double)self.config.temporal_weight *
                      (double)[self dot:self.bufTmpB aOff:0 b:self.bufTmpB bOff:0 n:topDim];
        }
    }

    if (self.config.latent_decay > 0.0f) {
        for (uint32_t l = 1u; l < self.archLen; ++l) {
            uint32_t dim = self.arch[l];
            float nrm2 = [self dot:self.bufZ aOff:self.zOffset[l] b:self.bufZ bOff:self.zOffset[l] n:dim];
            energy += 0.5 * (double)self.config.latent_decay * (double)nrm2;
        }
    }

    if (self.config.sparsity > 0.0f) {
        for (uint32_t l = 1u; l < self.archLen; ++l) {
            uint32_t dim = self.arch[l];
            float l1 = [self absSum:self.bufZ aOff:self.zOffset[l] n:dim];
            energy += (double)self.config.sparsity * (double)l1;
        }
    }

    return (float)energy;
}

// ---- reconstruction error: ||z[0] - tanh(W[0] z[1])|| ---------------------

- (float)reconstructionError {
    uint32_t rows = self.arch[0];
    uint32_t cols = self.arch[1];

    [self gemv:YES matrix:self.bufW matOff:self.wOffset[0]
           x:self.bufZ xOff:self.zOffset[1]
         out:self.bufTmpA outOff:0 rows:rows cols:cols];
    [self binaryOp:self.pVecSub
                 a:self.bufZ aOff:self.zOffset[0]
                 b:self.bufTmpA bOff:0
               out:self.bufTmpB outOff:0 n:rows];
    float sq = [self dot:self.bufTmpB aOff:0 b:self.bufTmpB bOff:0 n:rows];
    return sqrtf(sq);
}

// ---- initializeLatents (bottom-up recognition + optional top-down) --------

- (void)initializeLatents {
    // bottomUp[0] = input(z[0]) ; bottomUp[l+1] = tanh(R[l] * bottomUp[l])
    [self copyN:self.arch[0] src:self.bufZ srcOff:self.zOffset[0]
            dst:self.bufBottomUp dstOff:self.zOffset[0]];

    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l];        // R[l] is cols x rows in W-space => (arch[l+1] x arch[l])
        uint32_t cols = self.arch[l + 1u];
        [self gemv:YES matrix:self.bufR matOff:self.rOffset[l]
               x:self.bufBottomUp xOff:self.zOffset[l]
             out:self.bufBottomUp outOff:self.zOffset[l + 1u] rows:cols cols:rows];
    }

    if (!self.hasPrevTop) {
        for (uint32_t l = 1u; l < self.archLen; ++l) {
            [self copyN:self.arch[l] src:self.bufBottomUp srcOff:self.zOffset[l]
                    dst:self.bufZ dstOff:self.zOffset[l]];
        }
        return;
    }

    // topDown[top] = tanh(A * prevTop)
    uint32_t topDim = self.arch[self.archLen - 1u];
    [self computeTopPriorInto:self.bufTopDown outOff:self.zOffset[self.archLen - 1u]];

    // topDown[l] = tanh(W[l] * topDown[l+1]) for l = numLinks-1 .. 1
    for (int l = (int)self.numLinks - 1; l > 0; --l) {
        uint32_t rows = self.arch[l];
        uint32_t cols = self.arch[l + 1u];
        [self gemv:YES matrix:self.bufW matOff:self.wOffset[l]
               x:self.bufTopDown xOff:self.zOffset[l + 1u]
             out:self.bufTopDown outOff:self.zOffset[l] rows:rows cols:cols];
    }
    (void)topDim;

    // z[l] = clamp(mix*topDown[l] + (1-mix)*bottomUp[l]) for l = 1..top
    float mix = self.config.top_down_init_mix;
    float limit = self.config.state_clip;
    for (uint32_t l = 1u; l < self.archLen; ++l) {
        uint32_t dim = self.arch[l];
        [self dispatch1D:self.pMergeClamp
                 buffers:@[self.bufTopDown, self.bufBottomUp, self.bufZ,
                           [self constantFloat:mix], [self constantFloat:limit], [self constantUint:dim]]
                 offsets:@[@((NSUInteger)self.zOffset[l] * sizeof(float)),
                           @((NSUInteger)self.zOffset[l] * sizeof(float)),
                           @((NSUInteger)self.zOffset[l] * sizeof(float)), @0, @0, @0]
             threadCount:dim];
    }
}

// ---- state gradients (into bufGrad, one layer at a time) ------------------
// Returns by leaving the per-layer gradient in bufGrad; caller applies it.

- (void)stateGradientForLayer:(uint32_t)layer {
    uint32_t topIndex = self.archLen - 1u;
    uint32_t dim = self.arch[layer];

    // gradient = 0
    [self scale:self.bufZ srcOff:self.zOffset[layer] out:self.bufGrad outOff:0 scalar:0.0f n:dim];

    // if layer < top: += precision .* error[layer]  (or just error)
    if (layer < topIndex) {
        uint32_t predOff = self.predOffset[layer];
        if (self.config.use_precision) {
            [self binaryOp:self.pVecMulElem
                         a:self.bufPrecision aOff:predOff
                         b:self.bufErr bOff:predOff
                       out:self.bufTmpA outOff:0 n:dim];
            [self axpy:self.bufTmpA srcOff:0 out:self.bufGrad outOff:0 scalar:1.0f n:dim];
        } else {
            [self axpy:self.bufErr srcOff:predOff out:self.bufGrad outOff:0 scalar:1.0f n:dim];
        }
    }

    // belowSignal = (1 - pred[layer-1]^2) .* error[layer-1] [.* precision[layer-1]]
    uint32_t belowRows = self.arch[layer - 1u];
    uint32_t belowPredOff = self.predOffset[layer - 1u];
    [self tanhDeriv:self.bufPred pOff:belowPredOff out:self.bufTmpA outOff:0 n:belowRows];
    [self binaryOp:self.pVecMulElem
                 a:self.bufTmpA aOff:0
                 b:self.bufErr bOff:belowPredOff
               out:self.bufTmpA outOff:0 n:belowRows];
    if (self.config.use_precision) {
        [self binaryOp:self.pVecMulElem
                     a:self.bufTmpA aOff:0
                     b:self.bufPrecision bOff:belowPredOff
                   out:self.bufTmpA outOff:0 n:belowRows];
    }

    // correction = W[layer-1]^T * belowSignal ; gradient -= correction
    [self gemvTranspose:self.bufW matOff:self.wOffset[layer - 1u]
                      x:self.bufTmpA xOff:0
                    out:self.bufTmpB outOff:0
                   rows:belowRows cols:dim];
    [self axpy:self.bufTmpB srcOff:0 out:self.bufGrad outOff:0 scalar:-1.0f n:dim];

    // temporal term at top
    if (layer == topIndex && self.hasPrevTop) {
        [self computeTopPriorInto:self.bufTmpA outOff:0];
        [self binaryOp:self.pVecSub
                     a:self.bufZ aOff:self.zOffset[topIndex]
                     b:self.bufTmpA bOff:0
                   out:self.bufTmpB outOff:0 n:dim];
        if (self.config.use_precision) {
            [self binaryOp:self.pVecMulElem
                         a:self.bufTmpB aOff:0
                         b:self.bufTemporalPrec bOff:0
                       out:self.bufTmpB outOff:0 n:dim];
        }
        [self axpy:self.bufTmpB srcOff:0 out:self.bufGrad outOff:0 scalar:self.config.temporal_weight n:dim];
    }

    // latent decay: gradient += latentDecay * z[layer]
    if (self.config.latent_decay > 0.0f) {
        [self axpy:self.bufZ srcOff:self.zOffset[layer] out:self.bufGrad outOff:0
            scalar:self.config.latent_decay n:dim];
    }

    // sparsity subgradient: gradient += sparsity * sign(z[layer])
    if (self.config.sparsity > 0.0f) {
        [self dispatch1D:self.pSparsitySubgrad
                 buffers:@[self.bufZ, self.bufGrad, [self constantFloat:self.config.sparsity], [self constantUint:dim]]
                 offsets:@[@((NSUInteger)self.zOffset[layer] * sizeof(float)), @0, @0, @0]
             threadCount:dim];
    }

    // grad clip
    [self gradClip:self.bufGrad gOff:0 clip:self.config.grad_clip n:dim];
}

// ---- settle ---------------------------------------------------------------

- (void)saveStates {
    [self copyN:self.zTotal src:self.bufZ srcOff:0 dst:self.bufZSaved dstOff:0];
}

- (void)restoreStates {
    [self copyN:self.zTotal src:self.bufZSaved srcOff:0 dst:self.bufZ dstOff:0];
}

- (void)settleInput:(const float *)input advanceTemporal:(BOOL)advanceTemporal {
    // load input into z[0]
    float *z = (float *)self.bufZ.contents;
    memcpy(z + self.zOffset[0], input, (size_t)self.arch[0] * sizeof(float));

    [self initializeLatents];
    // z[0] must stay as the input after init
    memcpy(z + self.zOffset[0], input, (size_t)self.arch[0] * sizeof(float));

    float energyPrev = [self energy];

    uint32_t topIndex = self.archLen - 1u;

    for (uint32_t step = 0u; step < self.config.max_inference_steps; ++step) {
        [self predictAdjacentLayers];

        // Compute and stash gradients per layer into saved-grad region of bufBottomUp
        // (reuse bufBottomUp as gradient cache: layout matches z offsets).
        for (uint32_t l = 1u; l <= topIndex; ++l) {
            [self stateGradientForLayer:l];
            [self copyN:self.arch[l] src:self.bufGrad srcOff:0
                    dst:self.bufBottomUp dstOff:self.zOffset[l]];
        }

        [self saveStates];
        float oldEnergy = energyPrev;
        BOOL accepted = NO;
        float stepSize = self.config.lr_state;

        uint32_t halvings = self.config.monotone_state_steps ? self.config.line_search_halvings : 0u;

        for (uint32_t h = 0u; h <= halvings; ++h) {
            // z[l] = clamp(saved[l] - stepSize * grad[l])
            for (uint32_t l = 1u; l <= topIndex; ++l) {
                uint32_t dim = self.arch[l];
                // tmpA = saved[l] - stepSize*grad[l]
                [self copyN:dim src:self.bufZSaved srcOff:self.zOffset[l] dst:self.bufTmpA dstOff:0];
                [self axpy:self.bufBottomUp srcOff:self.zOffset[l] out:self.bufTmpA outOff:0
                    scalar:-stepSize n:dim];
                [self clampVec:self.bufTmpA srcOff:0 out:self.bufZ outOff:self.zOffset[l]
                         limit:self.config.state_clip n:dim];
            }
            // z[0] stays input
            memcpy(z + self.zOffset[0], input, (size_t)self.arch[0] * sizeof(float));

            float newEnergy = [self energy];

            if (!self.config.monotone_state_steps || newEnergy <= oldEnergy + 1e-12f) {
                accepted = YES;
                energyPrev = newEnergy;
                break;
            }

            [self restoreStates];
            stepSize *= 0.5f;
        }

        if (!accepted) {
            [self restoreStates];
            memcpy(z + self.zOffset[0], input, (size_t)self.arch[0] * sizeof(float));
            energyPrev = oldEnergy;
        }

        float deltaEnergy = fabsf(oldEnergy - energyPrev);
        float relativeDelta = deltaEnergy / (fabsf(oldEnergy) + 1e-12f);

        if (step + 1u >= self.config.min_inference_steps && relativeDelta < self.config.early_stop_tol) {
            break;
        }
    }

    if (advanceTemporal) {
        [self advanceTemporalState];
    }
}

// ---- advance temporal state ----------------------------------------------

- (void)advanceTemporalState {
    uint32_t topIndex = self.archLen - 1u;
    uint32_t topDim = self.arch[topIndex];
    [self copyN:topDim src:self.bufZ srcOff:self.zOffset[topIndex] dst:self.bufPrevTop dstOff:0];
    self.hasPrevTop = YES;
}

// ---- learn ----------------------------------------------------------------

- (void)learnTarget:(const float *)target hasTarget:(BOOL)hasTarget {
    [self predictAdjacentLayers];
    uint32_t topIndex = self.archLen - 1u;
    uint32_t topDim = self.arch[topIndex];

    BOOL useTarget = hasTarget && self.targetDim > 0u && self.bufV != nil;
    if (useTarget) {
        memcpy(self.bufTarget.contents, target, (size_t)self.targetDim * sizeof(float));
    }

    // Generative weight updates W[l].
    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l];
        uint32_t cols = self.arch[l + 1u];
        uint32_t predOff = self.predOffset[l];

        // localSignal = (1 - pred^2) .* error .* precision
        [self tanhDeriv:self.bufPred pOff:predOff out:self.bufTmpA outOff:0 n:rows];
        [self binaryOp:self.pVecMulElem a:self.bufTmpA aOff:0 b:self.bufErr bOff:predOff
                   out:self.bufTmpA outOff:0 n:rows];
        if (self.config.use_precision) {
            [self binaryOp:self.pVecMulElem a:self.bufTmpA aOff:0 b:self.bufPrecision bOff:predOff
                       out:self.bufTmpA outOff:0 n:rows];
        }

        // update = localSignal (rows) x z[l+1] (cols), clipped, scaled by lrGenerative
        [self outerUpdate:self.bufW matOff:self.wOffset[l]
                        a:self.bufTmpA aOff:0
                        b:self.bufZ bOff:self.zOffset[l + 1u]
                     rows:rows cols:cols
                       lr:self.config.lr_generative];
    }

    // Recognition weight updates R[l].
    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l];
        uint32_t cols = self.arch[l + 1u];

        // proposal = tanh(R[l] * z[l]) (cols)
        [self gemv:YES matrix:self.bufR matOff:self.rOffset[l]
               x:self.bufZ xOff:self.zOffset[l]
             out:self.bufTmpA outOff:0 rows:cols cols:rows];
        // recError = z[l+1] - proposal (cols)
        [self binaryOp:self.pVecSub a:self.bufZ aOff:self.zOffset[l + 1u] b:self.bufTmpA bOff:0
                   out:self.bufTmpB outOff:0 n:cols];
        // recSignal = (1 - proposal^2) .* recError (cols)
        [self tanhDeriv:self.bufTmpA pOff:0 out:self.bufTmpC outOff:0 n:cols];
        [self binaryOp:self.pVecMulElem a:self.bufTmpC aOff:0 b:self.bufTmpB bOff:0
                   out:self.bufTmpC outOff:0 n:cols];

        // update = recSignal (cols) x z[l] (rows) -> R[l] is cols x rows
        [self outerUpdate:self.bufR matOff:self.rOffset[l]
                        a:self.bufTmpC aOff:0
                        b:self.bufZ bOff:self.zOffset[l]
                     rows:cols cols:rows
                       lr:self.config.lr_recognition];
    }

    // Task head V.
    if (useTarget) {
        // taskPred = tanh(V * z[top]) (targetDim)
        [self gemv:YES matrix:self.bufV matOff:0 x:self.bufZ xOff:self.zOffset[topIndex]
             out:self.bufTmpA outOff:0 rows:self.targetDim cols:topDim];
        // taskError = target - taskPred
        [self binaryOp:self.pVecSub a:self.bufTarget aOff:0 b:self.bufTmpA bOff:0
                   out:self.bufTmpB outOff:0 n:self.targetDim];
        // taskSignal = (1 - taskPred^2) .* taskError .* taskPrecision
        [self tanhDeriv:self.bufTmpA pOff:0 out:self.bufTmpC outOff:0 n:self.targetDim];
        [self binaryOp:self.pVecMulElem a:self.bufTmpC aOff:0 b:self.bufTmpB bOff:0
                   out:self.bufTmpC outOff:0 n:self.targetDim];
        if (self.config.use_precision) {
            [self binaryOp:self.pVecMulElem a:self.bufTmpC aOff:0 b:self.bufTaskPrec bOff:0
                       out:self.bufTmpC outOff:0 n:self.targetDim];
        }
        [self outerUpdate:self.bufV matOff:0
                        a:self.bufTmpC aOff:0
                        b:self.bufZ bOff:self.zOffset[topIndex]
                     rows:self.targetDim cols:topDim
                       lr:self.config.lr_generative];
    }

    // Temporal weights A.
    if (self.hasPrevTop) {
        // topPrior = tanh(A * prevTop) (top)
        [self computeTopPriorInto:self.bufTmpA outOff:0];
        // temporalError = z[top] - topPrior
        [self binaryOp:self.pVecSub a:self.bufZ aOff:self.zOffset[topIndex] b:self.bufTmpA bOff:0
                   out:self.bufTmpB outOff:0 n:topDim];
        // temporalSignal = (1 - topPrior^2) .* temporalError .* temporalPrecision * temporalWeight
        [self tanhDeriv:self.bufTmpA pOff:0 out:self.bufTmpC outOff:0 n:topDim];
        [self binaryOp:self.pVecMulElem a:self.bufTmpC aOff:0 b:self.bufTmpB bOff:0
                   out:self.bufTmpC outOff:0 n:topDim];
        if (self.config.use_precision) {
            [self binaryOp:self.pVecMulElem a:self.bufTmpC aOff:0 b:self.bufTemporalPrec bOff:0
                       out:self.bufTmpC outOff:0 n:topDim];
        }
        [self scale:self.bufTmpC srcOff:0 out:self.bufTmpC outOff:0 scalar:self.config.temporal_weight n:topDim];
        [self outerUpdate:self.bufA matOff:0
                        a:self.bufTmpC aOff:0
                        b:self.bufPrevTop bOff:0
                     rows:topDim cols:topDim
                       lr:self.config.lr_temporal];
    }

    [self updatePrecision:useTarget];
    [self advanceTemporalState];
}

// outerUpdate with grad-clip on the outer-product magnitude then lr scale + decay.
- (void)outerUpdate:(id<MTLBuffer>)matrix matOff:(uint32_t)matOff
                  a:(id<MTLBuffer>)a aOff:(uint32_t)aOff
                  b:(id<MTLBuffer>)b bOff:(uint32_t)bOff
               rows:(uint32_t)rows cols:(uint32_t)cols
                 lr:(float)lr {
    // The gonum reference clips the outer product (Frobenius norm) before
    // scaling by lr. ||a (x) b||_F = ||a||_2 * ||b||_2, so we derive the clip
    // factor from the two vector norms without materializing the matrix.
    float aSq = [self dot:a aOff:aOff b:a bOff:aOff n:rows];
    float bSq = [self dot:b aOff:bOff b:b bOff:bOff n:cols];
    float norm = sqrtf(aSq) * sqrtf(bSq);
    float scale = lr;
    if (norm > self.config.grad_clip) {
        scale = lr * (self.config.grad_clip / (norm + 1e-12f));
    }

    float decay = (self.config.weight_decay > 0.0f) ? (lr * self.config.weight_decay) : 0.0f;

    [self dispatch1D:self.pOuterUpdate
             buffers:@[matrix, a, b, [self constantFloat:scale], [self constantFloat:decay],
                       [self constantUint:rows], [self constantUint:cols]]
             offsets:@[@((NSUInteger)matOff * sizeof(float)), @((NSUInteger)aOff * sizeof(float)),
                       @((NSUInteger)bOff * sizeof(float)), @0, @0, @0, @0]
         threadCount:rows * cols];
}

// ---- precision update -----------------------------------------------------

- (void)precisionUpdateErr:(id<MTLBuffer>)err errOff:(uint32_t)errOff
                  variance:(id<MTLBuffer>)variance varOff:(uint32_t)varOff
                 precision:(id<MTLBuffer>)precision precOff:(uint32_t)precOff
                         n:(uint32_t)n {
    [self dispatch1D:self.pPrecisionUpdate
             buffers:@[err, variance, precision,
                       [self constantFloat:self.config.precision_beta],
                       [self constantFloat:self.config.precision_eps],
                       [self constantFloat:self.config.precision_min],
                       [self constantFloat:self.config.precision_max],
                       [self constantUint:n]]
             offsets:@[@((NSUInteger)errOff * sizeof(float)), @((NSUInteger)varOff * sizeof(float)),
                       @((NSUInteger)precOff * sizeof(float)), @0, @0, @0, @0, @0]
         threadCount:n];
}

- (void)updatePrecision:(BOOL)useTarget {
    if (!self.config.use_precision) {
        return;
    }

    uint32_t topIndex = self.archLen - 1u;
    uint32_t topDim = self.arch[topIndex];

    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l];
        uint32_t predOff = self.predOffset[l];
        [self precisionUpdateErr:self.bufErr errOff:predOff
                        variance:self.bufVariance varOff:predOff
                       precision:self.bufPrecision precOff:predOff n:rows];
    }

    if (self.hasPrevTop) {
        [self computeTopPriorInto:self.bufTmpA outOff:0];
        [self binaryOp:self.pVecSub a:self.bufZ aOff:self.zOffset[topIndex] b:self.bufTmpA bOff:0
                   out:self.bufTmpB outOff:0 n:topDim];
        [self precisionUpdateErr:self.bufTmpB errOff:0
                        variance:self.bufTemporalVar varOff:0
                       precision:self.bufTemporalPrec precOff:0 n:topDim];
    }

    if (useTarget) {
        [self gemv:YES matrix:self.bufV matOff:0 x:self.bufZ xOff:self.zOffset[topIndex]
             out:self.bufTmpA outOff:0 rows:self.targetDim cols:topDim];
        [self binaryOp:self.pVecSub a:self.bufTarget aOff:0 b:self.bufTmpA bOff:0
                   out:self.bufTmpB outOff:0 n:self.targetDim];
        [self precisionUpdateErr:self.bufTmpB errOff:0
                        variance:self.bufTaskVar varOff:0
                       precision:self.bufTaskPrec precOff:0 n:self.targetDim];
    }
}

// ---- reset ----------------------------------------------------------------

- (void)resetState:(BOOL)resetPrecision {
    memset(self.bufZ.contents, 0, (size_t)self.zTotal * sizeof(float));
    self.hasPrevTop = NO;

    if (resetPrecision) {
        [self fillFloats:self.bufVariance count:self.predTotal value:1.0f];
        [self fillFloats:self.bufPrecision count:self.predTotal value:1.0f];
        uint32_t topDim = self.arch[self.archLen - 1u];
        [self fillFloats:self.bufTemporalVar count:topDim value:1.0f];
        [self fillFloats:self.bufTemporalPrec count:topDim value:1.0f];
        if (self.targetDim > 0u) {
            [self fillFloats:self.bufTaskVar count:self.targetDim value:1.0f];
            [self fillFloats:self.bufTaskPrec count:self.targetDim value:1.0f];
        }
    }
}

@end
