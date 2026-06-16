#import "solver_private.h"

/*
Batched settle/learn. The math is the verified single-sample predictive-coding
update, lifted to N columns: every kernel does N symbols' work per dispatch, the
line search and early-stop run per column, and converged columns are frozen via
the active mask. A whole settle step is one command buffer (barriers between
dependent kernels); only the per-column energy scalars are read back to drive
the loop's accept/early-stop decisions.
*/

@implementation BatchResonanceSolver (Compute)

static inline NSUInteger bo(uint32_t scalarOffset) { return (NSUInteger)scalarOffset * sizeof(float); }

// layer block byte offset within a [scalar x N] buffer.
- (NSUInteger)layerByteOff:(uint32_t)layerScalarOff {
    return (NSUInteger)layerScalarOff * self.batch * sizeof(float);
}

// ---- batched primitives (encoded) ----------------------------------------

// out = act(W_l * z[l+1]) into pred/err layout. mat from bufW/bufR/bufA/bufV.
- (void)encGemv:(id<MTLComputeCommandEncoder>)enc act:(BOOL)act
         matrix:(id<MTLBuffer>)matrix matBase:(uint32_t)matBase matOff:(uint32_t)matOff
            xOff:(uint32_t)xOff outBuf:(id<MTLBuffer>)outBuf outOff:(uint32_t)outOff
           xBuf:(id<MTLBuffer>)xBuf rows:(uint32_t)rows cols:(uint32_t)cols {
    NSUInteger N = self.batch;
    [self enc:enc pipe:self.pGemv
      buffers:@[matrix, xBuf, outBuf] offsets:NULL
       consts:@[@[@"u", @(matBase)], @[@"u", @(matOff)], @[@"u", @(xOff)], @[@"u", @(outOff)],
                @[@"u", @(rows)], @[@"u", @(cols)], @[@"u", @(self.batch)], @[@"u", @(act ? 1u : 0u)]]
      threads:rows * N];
}

- (void)encGemvT:(id<MTLComputeCommandEncoder>)enc
          matrix:(id<MTLBuffer>)matrix matBase:(uint32_t)matBase matOff:(uint32_t)matOff
            xBuf:(id<MTLBuffer>)xBuf xOff:(uint32_t)xOff
          outBuf:(id<MTLBuffer>)outBuf outOff:(uint32_t)outOff rows:(uint32_t)rows cols:(uint32_t)cols {
    NSUInteger N = self.batch;
    [self enc:enc pipe:self.pGemvT
      buffers:@[matrix, xBuf, outBuf] offsets:NULL
       consts:@[@[@"u", @(matBase)], @[@"u", @(matOff)], @[@"u", @(xOff)], @[@"u", @(outOff)],
                @[@"u", @(rows)], @[@"u", @(cols)], @[@"u", @(self.batch)]]
      threads:cols * N];
}

// elementwise over a layer block (dim*N): pipe in {pSub,pMul}. a,b,out are
// buffers; offsets are layer scalar offsets (converted to byte * N).
- (void)encBin:(id<MTLComputeCommandEncoder>)enc pipe:(id<MTLComputePipelineState>)pipe
             a:(id<MTLBuffer>)a aOff:(uint32_t)aOff b:(id<MTLBuffer>)b bOff:(uint32_t)bOff
           out:(id<MTLBuffer>)out outOff:(uint32_t)outOff dim:(uint32_t)dim {
    NSUInteger N = self.batch;
    NSUInteger offs[3] = { [self layerByteOff:aOff], [self layerByteOff:bOff], [self layerByteOff:outOff] };
    [self enc:enc pipe:pipe buffers:@[a, b, out] offsets:offs
       consts:@[@[@"u", @(dim * (uint32_t)N)]] threads:dim * N];
}

- (void)encCopy:(id<MTLComputeCommandEncoder>)enc src:(id<MTLBuffer>)src srcOff:(uint32_t)srcOff
            dst:(id<MTLBuffer>)dst dstOff:(uint32_t)dstOff count:(uint32_t)count {
    NSUInteger offs[2] = { bo(srcOff), bo(dstOff) };
    [self enc:enc pipe:self.pCopy buffers:@[src, dst] offsets:offs
       consts:@[@[@"u", @(count)]] threads:count];
}

- (void)encTanhDeriv:(id<MTLComputeCommandEncoder>)enc p:(id<MTLBuffer>)p pOff:(uint32_t)pOff
                 out:(id<MTLBuffer>)out outOff:(uint32_t)outOff dim:(uint32_t)dim {
    NSUInteger N = self.batch;
    NSUInteger offs[2] = { [self layerByteOff:pOff], [self layerByteOff:outOff] };
    [self enc:enc pipe:self.pTanhDeriv buffers:@[p, out] offsets:offs
       consts:@[@[@"u", @(dim * (uint32_t)N)]] threads:dim * N];
}

- (void)encAxpy:(id<MTLComputeCommandEncoder>)enc src:(id<MTLBuffer>)src srcOff:(uint32_t)srcOff
            out:(id<MTLBuffer>)out outOff:(uint32_t)outOff scalar:(float)scalar dim:(uint32_t)dim {
    NSUInteger N = self.batch;
    NSUInteger offs[2] = { [self layerByteOff:srcOff], [self layerByteOff:outOff] };
    [self enc:enc pipe:self.pAxpy buffers:@[src, out] offsets:offs
       consts:@[@[@"f", @(scalar)], @[@"u", @(dim * (uint32_t)N)]] threads:dim * N];
}

- (void)encScale:(id<MTLComputeCommandEncoder>)enc src:(id<MTLBuffer>)src srcOff:(uint32_t)srcOff
             out:(id<MTLBuffer>)out outOff:(uint32_t)outOff scalar:(float)scalar dim:(uint32_t)dim {
    NSUInteger N = self.batch;
    NSUInteger offs[2] = { [self layerByteOff:srcOff], [self layerByteOff:outOff] };
    [self enc:enc pipe:self.pScale buffers:@[src, out] offsets:offs
       consts:@[@[@"f", @(scalar)], @[@"u", @(dim * (uint32_t)N)]] threads:dim * N];
}

// topPrior = tanh(A * prevTop) into dst[dstOff layer block]. A is per-symbol.
- (void)encTopPrior:(id<MTLComputeCommandEncoder>)enc dst:(id<MTLBuffer>)dst dstOff:(uint32_t)dstOff {
    uint32_t top = self.arch[self.archLen - 1u];
    // prevTop occupies its own [top x N] buffer at offset 0; treat as layer off 0.
    [self encGemv:enc act:YES matrix:self.bufA matBase:self.aTotal matOff:0
             xOff:0 outBuf:dst outOff:dstOff xBuf:self.bufPrevTop rows:top cols:top];
}

// ---- predict ---------------------------------------------------------------

- (void)encPredict:(id<MTLComputeCommandEncoder>)enc {
    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l], cols = self.arch[l + 1u];
        uint32_t predOff = self.predOffset[l];
        [self encGemv:enc act:YES matrix:self.bufW matBase:self.wTotal matOff:self.wOffset[l]
                 xOff:self.zOffset[l + 1u] outBuf:self.bufPred outOff:predOff xBuf:self.bufZ rows:rows cols:cols];
        [self encBin:enc pipe:self.pSub a:self.bufZ aOff:self.zOffset[l] b:self.bufPred bOff:predOff
                 out:self.bufErr outOff:predOff dim:rows];
    }
}

// ---- init latents (bottom-up recognition + optional top-down) -------------

- (void)encInitLatents:(id<MTLComputeCommandEncoder>)enc {
    NSUInteger N = self.batch;
    // bottomUp[0] = z[0] (input)
    [self encCopy:enc src:self.bufZ srcOff:self.zOffset[0] * (uint32_t)N
              dst:self.bufBottomUp dstOff:self.zOffset[0] * (uint32_t)N count:self.arch[0] * (uint32_t)N];
    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l], cols = self.arch[l + 1u];
        // R[l] is cols x rows ; proposal[l+1] = tanh(R[l] * bottomUp[l])
        [self encGemv:enc act:YES matrix:self.bufR matBase:self.rTotal matOff:self.rOffset[l]
                 xOff:self.zOffset[l] outBuf:self.bufBottomUp outOff:self.zOffset[l + 1u]
                 xBuf:self.bufBottomUp rows:cols cols:rows];
    }

    if (!self.hasPrevTop) {
        for (uint32_t l = 1u; l < self.archLen; ++l) {
            [self encCopy:enc src:self.bufBottomUp srcOff:self.zOffset[l] * (uint32_t)N
                      dst:self.bufZ dstOff:self.zOffset[l] * (uint32_t)N count:self.arch[l] * (uint32_t)N];
        }
        return;
    }

    uint32_t topIndex = self.archLen - 1u;
    [self encTopPrior:enc dst:self.bufTopDown dstOff:self.zOffset[topIndex]];
    for (int l = (int)self.numLinks - 1; l > 0; --l) {
        uint32_t rows = self.arch[l], cols = self.arch[l + 1u];
        [self encGemv:enc act:YES matrix:self.bufW matBase:self.wTotal matOff:self.wOffset[l]
                 xOff:self.zOffset[l + 1u] outBuf:self.bufTopDown outOff:self.zOffset[l]
                 xBuf:self.bufTopDown rows:rows cols:cols];
    }

    float mix = self.config.top_down_init_mix, clip = self.config.state_clip;
    for (uint32_t l = 1u; l < self.archLen; ++l) {
        uint32_t dim = self.arch[l];
        NSUInteger off = [self layerByteOff:self.zOffset[l]];
        NSUInteger offs[3] = { off, off, off };
        [self enc:enc pipe:self.pMergeClamp
          buffers:@[self.bufTopDown, self.bufBottomUp, self.bufZ] offsets:offs
           consts:@[@[@"f", @(mix)], @[@"f", @(clip)], @[@"u", @(dim * (uint32_t)N)]]
          threads:dim * N];
    }
}

// ---- per-layer gradient into gradCache ------------------------------------

- (void)encGradLayer:(id<MTLComputeCommandEncoder>)enc layer:(uint32_t)layer {
    uint32_t topIndex = self.archLen - 1u;
    uint32_t dim = self.arch[layer];
    uint32_t gOff = self.zOffset[layer];
    BOOL useP = self.config.use_precision;
    NSUInteger N = self.batch;

    // gradient = 0  (scale z by 0 into gradCache slot)
    [self encScale:enc src:self.bufZ srcOff:gOff out:self.bufGradCache outOff:gOff scalar:0.0f dim:dim];

    if (layer < topIndex) {
        uint32_t predOff = self.predOffset[layer];
        if (useP) {
            [self encBin:enc pipe:self.pMul a:self.bufPrecision aOff:predOff b:self.bufErr bOff:predOff
                     out:self.bufTmpA outOff:0 dim:dim];
            [self encAxpy:enc src:self.bufTmpA srcOff:0 out:self.bufGradCache outOff:gOff scalar:1.0f dim:dim];
        } else {
            [self encAxpy:enc src:self.bufErr srcOff:predOff out:self.bufGradCache outOff:gOff scalar:1.0f dim:dim];
        }
    }

    uint32_t belowRows = self.arch[layer - 1u];
    uint32_t belowPredOff = self.predOffset[layer - 1u];
    [self encTanhDeriv:enc p:self.bufPred pOff:belowPredOff out:self.bufTmpA outOff:0 dim:belowRows];
    [self encBin:enc pipe:self.pMul a:self.bufTmpA aOff:0 b:self.bufErr bOff:belowPredOff out:self.bufTmpA outOff:0 dim:belowRows];
    if (useP) {
        [self encBin:enc pipe:self.pMul a:self.bufTmpA aOff:0 b:self.bufPrecision bOff:belowPredOff out:self.bufTmpA outOff:0 dim:belowRows];
    }
    // correction = W[layer-1]^T * belowSignal ; grad -= correction
    [self encGemvT:enc matrix:self.bufW matBase:self.wTotal matOff:self.wOffset[layer - 1u]
              xBuf:self.bufTmpA xOff:0 outBuf:self.bufTmpB outOff:0 rows:belowRows cols:dim];
    [self encAxpy:enc src:self.bufTmpB srcOff:0 out:self.bufGradCache outOff:gOff scalar:-1.0f dim:dim];

    if (layer == topIndex && self.hasPrevTop) {
        [self encTopPrior:enc dst:self.bufTmpA dstOff:0];
        [self encBin:enc pipe:self.pSub a:self.bufZ aOff:self.zOffset[topIndex] b:self.bufTmpA bOff:0 out:self.bufTmpB outOff:0 dim:dim];
        if (useP) {
            [self encBin:enc pipe:self.pMul a:self.bufTmpB aOff:0 b:self.bufTemporalPrec bOff:0 out:self.bufTmpB outOff:0 dim:dim];
        }
        [self encAxpy:enc src:self.bufTmpB srcOff:0 out:self.bufGradCache outOff:gOff scalar:self.config.temporal_weight dim:dim];
    }

    if (self.config.latent_decay > 0.0f) {
        [self encAxpy:enc src:self.bufZ srcOff:self.zOffset[layer] out:self.bufGradCache outOff:gOff scalar:self.config.latent_decay dim:dim];
    }
    if (self.config.sparsity > 0.0f) {
        NSUInteger off = [self layerByteOff:gOff];
        NSUInteger zoff = [self layerByteOff:self.zOffset[layer]];
        NSUInteger offs[2] = { zoff, off };
        [self enc:enc pipe:self.pSparsity buffers:@[self.bufZ, self.bufGradCache] offsets:offs
           consts:@[@[@"f", @(self.config.sparsity)], @[@"u", @(dim * (uint32_t)N)]] threads:dim * N];
    }

    // per-column grad clip on this layer block
    [self encReduce:enc pipe:self.pGradClipLayer
            buffers:@[self.bufGradCache] offsets:NULL
             consts:@[@[@"u", @(gOff)], @[@"u", @(dim)], @[@"u", @(self.batch)], @[@"f", @(self.config.grad_clip)]]
            columns:self.batch];
}

// ---- per-column energy into energyBuf -------------------------------------

- (void)encEnergy:(id<MTLComputeCommandEncoder>)enc into:(id<MTLBuffer>)energyBuf {
    uint32_t topIndex = self.archLen - 1u, top = self.arch[topIndex];
    if (self.hasPrevTop) {
        [self encTopPrior:enc dst:self.bufTmpA dstOff:0];
        [self encBin:enc pipe:self.pSub a:self.bufZ aOff:self.zOffset[topIndex] b:self.bufTmpA bOff:0
                 out:self.bufTemporalErr outOff:0 dim:top];
    }
    [self encReduce:enc pipe:self.pEnergy
            buffers:@[self.bufZ, self.bufErr, self.bufPrecision, self.bufTemporalErr, self.bufTemporalPrec,
                      self.bufArchDim, self.bufZOff, self.bufPredOff, self.bufDims, self.bufHasPrev, energyBuf]
            offsets:NULL
             consts:@[@[@"u", @0]]
            columns:self.batch];
}

// ---- settle ---------------------------------------------------------------

- (void)settleAdvanceTemporal:(BOOL)advanceTemporal {
    [self syncDims];
    NSUInteger N = self.batch;
    uint32_t topIndex = self.archLen - 1u;
    uint32_t halvings = self.config.monotone_state_steps ? self.config.line_search_halvings : 0u;
    uint32_t monotone = self.config.monotone_state_steps ? 1u : 0u;

    uint32_t *active = (uint32_t *)self.bufActive.contents;
    for (uint32_t s = 0u; s < N; ++s) active[s] = 1u;

    float *stepBuf = (float *)self.bufStep.contents;
    uint32_t *flags = (uint32_t *)self.bufFlags.contents;

    // init: latents, predict, energyOld.
    {
        id<MTLCommandBuffer> cb = [self.queue commandBuffer];
        id<MTLComputeCommandEncoder> enc = [cb computeCommandEncoder];
        [self encInitLatents:enc];
        [self encPredict:enc];
        [self encEnergy:enc into:self.bufEnergyOld];
        [enc endEncoding]; [cb commit]; [cb waitUntilCompleted];
    }

    for (uint32_t step = 0u; step < self.config.max_inference_steps; ++step) {
        // CB-A: predict (refresh err for grads), gradients, save z.
        {
            id<MTLCommandBuffer> cb = [self.queue commandBuffer];
            id<MTLComputeCommandEncoder> enc = [cb computeCommandEncoder];
            [self encPredict:enc];
            for (uint32_t l = 1u; l <= topIndex; ++l) [self encGradLayer:enc layer:l];
            [self encCopy:enc src:self.bufZ srcOff:0 dst:self.bufZSaved dstOff:0 count:self.zTotal * (uint32_t)N];
            [enc endEncoding]; [cb commit]; [cb waitUntilCompleted];
        }

        // Snapshot each column's start energy (energyOld currently holds it:
        // the initial energy, or the previous step's carried-accept energy).
        {
            const float *eOld = (const float *)self.bufEnergyOld.contents;
            for (uint32_t s = 0u; s < N; ++s) self.startSnapshot[s] = eOld[s];
        }

        for (uint32_t s = 0u; s < N; ++s) stepBuf[s] = self.config.lr_state;

        for (uint32_t h = 0u; h <= halvings; ++h) {
            uint32_t isLast = (h == halvings) ? 1u : 0u;
            id<MTLCommandBuffer> cb = [self.queue commandBuffer];
            id<MTLComputeCommandEncoder> enc = [cb computeCommandEncoder];

            // z = clamp(saved - step[s]*grad) for active columns, layers>=1.
            [self enc:enc pipe:self.pApplyState
              buffers:@[self.bufZ, self.bufZSaved, self.bufGradCache, self.bufLayerRow, self.bufStep, self.bufActive]
              offsets:NULL
               consts:@[@[@"u", @(self.zTotal)], @[@"u", @(self.batch)], @[@"f", @(self.config.state_clip)]]
              threads:self.zTotal * N];

            [self encPredict:enc];
            [self encEnergy:enc into:self.bufEnergyNew];

            // decide per column
            [self enc:enc pipe:self.pDecide
              buffers:@[self.bufEnergyOld, self.bufEnergyNew, self.bufFlags, self.bufEnergyOld, self.bufStep, self.bufActive]
              offsets:NULL
               consts:@[@[@"u", @(monotone)], @[@"u", @(isLast)], @[@"u", @(self.batch)]]
              threads:N];

            // revert columns whose flags==0
            [self enc:enc pipe:self.pRevert
              buffers:@[self.bufZ, self.bufZSaved, self.bufLayerRow, self.bufFlags]
              offsets:NULL
               consts:@[@[@"u", @(self.zTotal)], @[@"u", @(self.batch)]]
              threads:self.zTotal * N];

            [enc endEncoding]; [cb commit]; [cb waitUntilCompleted];

            // host check: any column still wants another halving (flag==2)?
            BOOL anyRetry = NO;
            for (uint32_t s = 0u; s < N; ++s) {
                if (active[s] && flags[s] == 2u) { anyRetry = YES; break; }
            }
            if (!anyRetry || isLast) break;
        }

        // Early stop per column. After the halving loop, energyOld holds each
        // column's carried (accepted/exhausted) end energy. Compare to the start
        // snapshot; freeze columns whose relative change fell below tol once past
        // the minimum step count. Mirrors the reference's per-settle early stop.
        {
            const float *eEnd = (const float *)self.bufEnergyOld.contents;
            uint32_t pastMin = (step + 1u >= self.config.min_inference_steps) ? 1u : 0u;
            uint32_t remaining = 0u;
            for (uint32_t s = 0u; s < N; ++s) {
                if (!active[s]) continue;
                float start = self.startSnapshot[s];
                float rel = fabsf(start - eEnd[s]) / (fabsf(start) + 1e-12f);
                if (pastMin && rel < self.config.early_stop_tol) active[s] = 0u;
                else remaining++;
            }
            if (remaining == 0u) break;
        }
    }

    if (advanceTemporal) {
        [self advanceTemporal];
    }
}

- (void)advanceTemporal {
    NSUInteger N = self.batch;
    uint32_t topIndex = self.archLen - 1u, top = self.arch[topIndex];
    id<MTLCommandBuffer> cb = [self.queue commandBuffer];
    id<MTLComputeCommandEncoder> enc = [cb computeCommandEncoder];
    [self encCopy:enc src:self.bufZ srcOff:self.zOffset[topIndex] * (uint32_t)N
              dst:self.bufPrevTop dstOff:0 count:top * (uint32_t)N];
    [enc endEncoding]; [cb commit]; [cb waitUntilCompleted];
    self.hasPrevTop = YES;
    [self syncDims];
}

// Full per-column energy reduction; caller reads the slot it needs.
- (float)energySlotCompute:(uint32_t)slot {
    [self syncDims];
    id<MTLCommandBuffer> cb = [self.queue commandBuffer];
    id<MTLComputeCommandEncoder> enc = [cb computeCommandEncoder];
    [self encPredict:enc];
    [self encEnergy:enc into:self.bufEnergyNew];
    [enc endEncoding]; [cb commit]; [cb waitUntilCompleted];
    return ((const float *)self.bufEnergyNew.contents)[slot];
}

// reconstruction error per column: ||z[0] - tanh(W[0] z[1])||, read slot.
- (float)reconstructionSlotCompute:(uint32_t)slot {
    [self syncDims];
    NSUInteger N = self.batch;
    uint32_t rows = self.arch[0], cols = self.arch[1];
    id<MTLCommandBuffer> cb = [self.queue commandBuffer];
    id<MTLComputeCommandEncoder> enc = [cb computeCommandEncoder];
    // tmpA = tanh(W[0] z[1]) ; tmpB = z[0]-tmpA
    [self encGemv:enc act:YES matrix:self.bufW matBase:self.wTotal matOff:self.wOffset[0]
             xOff:self.zOffset[1] outBuf:self.bufTmpA outOff:0 xBuf:self.bufZ rows:rows cols:cols];
    [self encBin:enc pipe:self.pSub a:self.bufZ aOff:self.zOffset[0] b:self.bufTmpA bOff:0 out:self.bufTmpB outOff:0 dim:rows];
    [enc endEncoding]; [cb commit]; [cb waitUntilCompleted];
    const float *d = (const float *)self.bufTmpB.contents;
    float sq = 0.0f;
    for (uint32_t r = 0u; r < rows; ++r) { float v = d[r * (uint32_t)N + slot]; sq += v * v; }
    return sqrtf(sq);
}

// ---- batched learn (one command buffer) -----------------------------------

- (void)encOuter:(id<MTLComputeCommandEncoder>)enc matrix:(id<MTLBuffer>)matrix
         matBase:(uint32_t)matBase matOff:(uint32_t)matOff
            aBuf:(id<MTLBuffer>)aBuf bBuf:(id<MTLBuffer>)bBuf bOff:(uint32_t)bOff
            rows:(uint32_t)rows cols:(uint32_t)cols lr:(float)lr useActive:(BOOL)useActive {
    NSUInteger N = self.batch;
    float decay = (self.config.weight_decay > 0.0f) ? (lr * self.config.weight_decay) : 0.0f;

    // factor[s] from per-column norms of a (rows x N at off 0) and b (cols x N at bOff block).
    NSUInteger fOffs[3] = { 0, [self layerByteOff:bOff], 0 };
    [self enc:enc pipe:self.pOuterFactor
      buffers:@[aBuf, bBuf, self.bufFactor] offsets:fOffs
       consts:@[@[@"u", @(rows)], @[@"u", @(cols)], @[@"u", @(self.batch)], @[@"f", @(lr)], @[@"f", @(self.config.grad_clip)]]
      threads:N];

    NSUInteger aOffs[5] = { 0, 0, [self layerByteOff:bOff], 0, 0 };
    [self enc:enc pipe:self.pOuterApply
      buffers:@[matrix, aBuf, bBuf, self.bufFactor, self.bufActive] offsets:aOffs
       consts:@[@[@"u", @(matBase)], @[@"u", @(matOff)], @[@"u", @(rows)], @[@"u", @(cols)],
                @[@"u", @(self.batch)], @[@"f", @(decay)], @[@"u", @(useActive ? 1u : 0u)]]
      threads:rows * cols * N];
}

- (void)learn {
    [self syncDims];
    uint32_t topIndex = self.archLen - 1u, top = self.arch[topIndex];
    BOOL useP = self.config.use_precision;
    BOOL useTarget = (self.targetDim > 0u && self.bufV != nil);

    id<MTLCommandBuffer> cb = [self.queue commandBuffer];
    id<MTLComputeCommandEncoder> enc = [cb computeCommandEncoder];

    [self encPredict:enc];

    // Generative W[l]. signal lands in tmpA (rows x N at off 0); b = z[l+1].
    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l], cols = self.arch[l + 1u];
        uint32_t predOff = self.predOffset[l];
        [self encTanhDeriv:enc p:self.bufPred pOff:predOff out:self.bufTmpA outOff:0 dim:rows];
        [self encBin:enc pipe:self.pMul a:self.bufTmpA aOff:0 b:self.bufErr bOff:predOff out:self.bufTmpA outOff:0 dim:rows];
        if (useP) [self encBin:enc pipe:self.pMul a:self.bufTmpA aOff:0 b:self.bufPrecision bOff:predOff out:self.bufTmpA outOff:0 dim:rows];
        [self encOuter:enc matrix:self.bufW matBase:self.wTotal matOff:self.wOffset[l]
                  aBuf:self.bufTmpA bBuf:self.bufZ bOff:self.zOffset[l + 1u]
                  rows:rows cols:cols lr:self.config.lr_generative useActive:NO];
    }

    // Recognition R[l]. proposal=tanh(R z[l]) (cols); recError=z[l+1]-proposal;
    // recSignal=(1-proposal^2).*recError ; update = recSignal (cols) x z[l] (rows).
    for (uint32_t l = 0u; l < self.numLinks; ++l) {
        uint32_t rows = self.arch[l], cols = self.arch[l + 1u];
        [self encGemv:enc act:YES matrix:self.bufR matBase:self.rTotal matOff:self.rOffset[l]
                 xOff:self.zOffset[l] outBuf:self.bufTmpA outOff:0 xBuf:self.bufZ rows:cols cols:rows];
        [self encBin:enc pipe:self.pSub a:self.bufZ aOff:self.zOffset[l + 1u] b:self.bufTmpA bOff:0 out:self.bufTmpB outOff:0 dim:cols];
        [self encTanhDeriv:enc p:self.bufTmpA pOff:0 out:self.bufTmpC outOff:0 dim:cols];
        [self encBin:enc pipe:self.pMul a:self.bufTmpC aOff:0 b:self.bufTmpB bOff:0 out:self.bufTmpC outOff:0 dim:cols];
        [self encOuter:enc matrix:self.bufR matBase:self.rTotal matOff:self.rOffset[l]
                  aBuf:self.bufTmpC bBuf:self.bufZ bOff:self.zOffset[l]
                  rows:cols cols:rows lr:self.config.lr_recognition useActive:NO];
    }

    // Task V.
    if (useTarget) {
        [self encGemv:enc act:YES matrix:self.bufV matBase:self.vTotal matOff:0
                 xOff:self.zOffset[topIndex] outBuf:self.bufTmpA outOff:0 xBuf:self.bufZ rows:self.targetDim cols:top];
        [self encBin:enc pipe:self.pSub a:self.bufTarget aOff:0 b:self.bufTmpA bOff:0 out:self.bufTmpB outOff:0 dim:self.targetDim];
        [self encTanhDeriv:enc p:self.bufTmpA pOff:0 out:self.bufTmpC outOff:0 dim:self.targetDim];
        [self encBin:enc pipe:self.pMul a:self.bufTmpC aOff:0 b:self.bufTmpB bOff:0 out:self.bufTmpC outOff:0 dim:self.targetDim];
        if (useP) [self encBin:enc pipe:self.pMul a:self.bufTmpC aOff:0 b:self.bufTaskPrec bOff:0 out:self.bufTmpC outOff:0 dim:self.targetDim];
        [self encOuter:enc matrix:self.bufV matBase:self.vTotal matOff:0
                  aBuf:self.bufTmpC bBuf:self.bufZ bOff:self.zOffset[topIndex]
                  rows:self.targetDim cols:top lr:self.config.lr_generative useActive:NO];
    }

    // Temporal A.
    if (self.hasPrevTop) {
        [self encTopPrior:enc dst:self.bufTmpA dstOff:0];
        [self encBin:enc pipe:self.pSub a:self.bufZ aOff:self.zOffset[topIndex] b:self.bufTmpA bOff:0 out:self.bufTmpB outOff:0 dim:top];
        [self encTanhDeriv:enc p:self.bufTmpA pOff:0 out:self.bufTmpC outOff:0 dim:top];
        [self encBin:enc pipe:self.pMul a:self.bufTmpC aOff:0 b:self.bufTmpB bOff:0 out:self.bufTmpC outOff:0 dim:top];
        if (useP) [self encBin:enc pipe:self.pMul a:self.bufTmpC aOff:0 b:self.bufTemporalPrec bOff:0 out:self.bufTmpC outOff:0 dim:top];
        [self encScale:enc src:self.bufTmpC srcOff:0 out:self.bufTmpC outOff:0 scalar:self.config.temporal_weight dim:top];
        // b = prevTop (top x N at off 0)
        [self encOuter:enc matrix:self.bufA matBase:self.aTotal matOff:0
                  aBuf:self.bufTmpC bBuf:self.bufPrevTop bOff:0
                  rows:top cols:top lr:self.config.lr_temporal useActive:NO];
    }

    // Precision EMA. Generative uses the PRE-update errors (bufErr from the
    // initial predict — never recomputed here). Temporal/task recompute with the
    // now-updated A/V, matching the reference.
    if (useP) {
        for (uint32_t l = 0u; l < self.numLinks; ++l) {
            uint32_t rows = self.arch[l], predOff = self.predOffset[l];
            [self encPrecision:enc err:self.bufErr errOff:predOff var:self.bufVariance prec:self.bufPrecision dim:rows];
        }
        if (self.hasPrevTop) {
            [self encTopPrior:enc dst:self.bufTmpA dstOff:0];
            [self encBin:enc pipe:self.pSub a:self.bufZ aOff:self.zOffset[topIndex] b:self.bufTmpA bOff:0 out:self.bufTmpB outOff:0 dim:top];
            [self encPrecisionBuf:enc errBuf:self.bufTmpB var:self.bufTemporalVar prec:self.bufTemporalPrec dim:top];
        }
        if (useTarget) {
            [self encGemv:enc act:YES matrix:self.bufV matBase:self.vTotal matOff:0
                     xOff:self.zOffset[topIndex] outBuf:self.bufTmpA outOff:0 xBuf:self.bufZ rows:self.targetDim cols:top];
            [self encBin:enc pipe:self.pSub a:self.bufTarget aOff:0 b:self.bufTmpA bOff:0 out:self.bufTmpB outOff:0 dim:self.targetDim];
            [self encPrecisionBuf:enc errBuf:self.bufTmpB var:self.bufTaskVar prec:self.bufTaskPrec dim:self.targetDim];
        }
    }

    [enc endEncoding]; [cb commit]; [cb waitUntilCompleted];

    [self advanceTemporal];
}

// precision update on a pred-layout block at predOff.
- (void)encPrecision:(id<MTLComputeCommandEncoder>)enc err:(id<MTLBuffer>)err errOff:(uint32_t)errOff
                 var:(id<MTLBuffer>)var prec:(id<MTLBuffer>)prec dim:(uint32_t)dim {
    NSUInteger N = self.batch;
    NSUInteger off = [self layerByteOff:errOff];
    NSUInteger offs[3] = { off, off, off };
    [self enc:enc pipe:self.pPrecision buffers:@[err, var, prec] offsets:offs
       consts:@[@[@"f", @(self.config.precision_beta)], @[@"f", @(self.config.precision_eps)],
                @[@"f", @(self.config.precision_min)], @[@"f", @(self.config.precision_max)],
                @[@"u", @(dim * (uint32_t)N)]] threads:dim * N];
}

// precision update where err is a standalone [dim x N] buffer at offset 0.
- (void)encPrecisionBuf:(id<MTLComputeCommandEncoder>)enc errBuf:(id<MTLBuffer>)errBuf
                    var:(id<MTLBuffer>)var prec:(id<MTLBuffer>)prec dim:(uint32_t)dim {
    NSUInteger N = self.batch;
    [self enc:enc pipe:self.pPrecision buffers:@[errBuf, var, prec] offsets:NULL
       consts:@[@[@"f", @(self.config.precision_beta)], @[@"f", @(self.config.precision_eps)],
                @[@"f", @(self.config.precision_min)], @[@"f", @(self.config.precision_max)],
                @[@"u", @(dim * (uint32_t)N)]] threads:dim * N];
}

@end
