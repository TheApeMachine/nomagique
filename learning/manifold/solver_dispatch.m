#import "solver_private.h"

@implementation BatchResonanceSolver (Pipelines)

- (id<MTLComputePipelineState>)pipe:(NSString *)name error:(NSError **)error {
    id<MTLFunction> fn = [self.library newFunctionWithName:name];
    if (fn == nil) {
        if (error) *error = [NSError errorWithDomain:@"resonance" code:1
            userInfo:@{NSLocalizedDescriptionKey: [NSString stringWithFormat:@"kernel %@ not found", name]}];
        return nil;
    }
    return [self.device newComputePipelineStateWithFunction:fn error:error];
}

- (BOOL)buildPipelines:(NSString **)error {
    NSError *e = nil;
    self.pGemv = [self pipe:@"bgemv" error:&e];
    self.pGemvT = [self pipe:@"bgemv_t" error:&e];
    self.pSub = [self pipe:@"bvec_sub" error:&e];
    self.pMul = [self pipe:@"bvec_mulelem" error:&e];
    self.pCopy = [self pipe:@"bvec_copy" error:&e];
    self.pTanhDeriv = [self pipe:@"btanh_deriv" error:&e];
    self.pAxpy = [self pipe:@"bvec_axpy" error:&e];
    self.pScale = [self pipe:@"bvec_scale" error:&e];
    self.pSparsity = [self pipe:@"bsparsity_subgrad" error:&e];
    self.pEnergy = [self pipe:@"benergy" error:&e];
    self.pGradClipLayer = [self pipe:@"bgrad_clip_layer" error:&e];
    self.pApplyState = [self pipe:@"bapply_state" error:&e];
    self.pDecide = [self pipe:@"bdecide" error:&e];
    self.pRevert = [self pipe:@"brevert" error:&e];
    self.pEarlyStop = [self pipe:@"bearly_stop" error:&e];
    self.pPrecision = [self pipe:@"bprecision_update" error:&e];
    self.pOuterFactor = [self pipe:@"bouter_factor" error:&e];
    self.pOuterApply = [self pipe:@"bouter_apply" error:&e];
    self.pMergeClamp = [self pipe:@"bmerge_clamp" error:&e];

    if (e != nil) { if (error) *error = e.localizedDescription ?: @"pipeline build failed"; return NO; }
    return YES;
}

@end

@implementation BatchResonanceSolver (Dispatch)

/*
enc encodes one kernel into `encoder` using setBytes for scalar constants (no
per-call buffer allocation), with a buffer barrier after the dispatch so the
next dependent kernel in the same command buffer observes the writes. consts is
an ordered list of @[@"u"|@"f", @(value)] appended after the buffers.
*/
- (void)enc:(id<MTLComputeCommandEncoder>)encoder
       pipe:(id<MTLComputePipelineState>)pipeline
    buffers:(NSArray<id<MTLBuffer>> *)buffers
    offsets:(const NSUInteger *)offsets
     consts:(NSArray<NSArray *> *)consts
    threads:(NSUInteger)threads
   perGroup:(NSUInteger)perGroup
   groups:(NSUInteger)groups {
    [encoder setComputePipelineState:pipeline];
    NSUInteger idx = 0;
    for (; idx < buffers.count; idx++) {
        NSUInteger off = (offsets != NULL) ? offsets[idx] : 0;
        [encoder setBuffer:buffers[idx] offset:off atIndex:idx];
    }
    for (NSUInteger c = 0; c < consts.count; c++) {
        NSString *type = consts[c][0];
        NSNumber *num = consts[c][1];
        if ([type isEqualToString:@"u"]) { uint32_t v = num.unsignedIntValue; [encoder setBytes:&v length:sizeof(uint32_t) atIndex:idx++]; }
        else { float v = num.floatValue; [encoder setBytes:&v length:sizeof(float) atIndex:idx++]; }
    }
    if (groups > 0) {
        [encoder dispatchThreadgroups:MTLSizeMake(groups, 1, 1)
                threadsPerThreadgroup:MTLSizeMake(perGroup, 1, 1)];
    } else {
        if (threads == 0) return;
        NSUInteger w = pipeline.maxTotalThreadsPerThreadgroup;
        if (w > threads) w = threads;
        [encoder dispatchThreads:MTLSizeMake(threads, 1, 1) threadsPerThreadgroup:MTLSizeMake(w, 1, 1)];
    }
    [encoder memoryBarrierWithScope:MTLBarrierScopeBuffers];
}

// 1-D grid over `threads`.
- (void)enc:(id<MTLComputeCommandEncoder>)encoder
       pipe:(id<MTLComputePipelineState>)pipeline
    buffers:(NSArray<id<MTLBuffer>> *)buffers
    offsets:(const NSUInteger *)offsets
     consts:(NSArray<NSArray *> *)consts
    threads:(NSUInteger)threads {
    [self enc:encoder pipe:pipeline buffers:buffers offsets:offsets consts:consts
      threads:threads perGroup:0 groups:0];
}

/*
reduceThreadgroupSizeFor returns the threadgroup width for a per-column tree
reduction over `reduceLen` elements.

Sizing note (measured on M-series, {8,16,8} x 800 columns): shrinking the group
to the reduction length — e.g. 32 threads for a 16-wide layer — is ~2.4x SLOWER
than a wider group, despite the "idle" threads. The reason is occupancy, not
reduction cost: with hundreds of columns each as a tiny 32-wide group, the GPU
fills its cores poorly, whereas a wider group keeps each core busy and lets the
scheduler overlap per-column work. The unused threads in the binary tree just
skip the `for` loop — nearly free — so a healthy occupancy FLOOR wins.

So: floor at kReduceOccupancyFloor (a full set of SIMD groups), grow to the next
power of two when the reduction is genuinely larger, and clamp to the pipeline's
queried maxTotalThreadsPerThreadgroup (1024 on Apple Silicon). Always a power of
two so the tree reduce stays valid; never exceeds the scratch[1024] in-kernel.
*/
static const NSUInteger kReduceOccupancyFloor = 256u;

- (NSUInteger)reduceThreadgroupSizeFor:(NSUInteger)reduceLen
                              pipeline:(id<MTLComputePipelineState>)pipeline {
    NSUInteger pow2 = 32u;                  // one SIMD group minimum
    while (pow2 < reduceLen) pow2 <<= 1;    // cover the whole reduction
    if (pow2 < kReduceOccupancyFloor) pow2 = kReduceOccupancyFloor;

    NSUInteger hwMax = pipeline.maxTotalThreadsPerThreadgroup;
    if (hwMax == 0) hwMax = 1024u;
    NSUInteger cap = 1u;                     // largest pow2 <= hwMax
    while ((cap << 1) <= hwMax) cap <<= 1;

    if (pow2 > cap) pow2 = cap;
    return pow2;
}

// One threadgroup per column; width sized to the reduction length, clamped to
// the pipeline's hardware max.
- (void)encReduce:(id<MTLComputeCommandEncoder>)encoder
             pipe:(id<MTLComputePipelineState>)pipeline
          buffers:(NSArray<id<MTLBuffer>> *)buffers
          offsets:(const NSUInteger *)offsets
           consts:(NSArray<NSArray *> *)consts
          columns:(NSUInteger)columns
        reduceLen:(NSUInteger)reduceLen {
    NSUInteger perGroup = [self reduceThreadgroupSizeFor:reduceLen pipeline:pipeline];
    [self enc:encoder pipe:pipeline buffers:buffers offsets:offsets consts:consts
      threads:0 perGroup:perGroup groups:columns];
}

- (void)syncDims {
    BatchDimsHost *d = (BatchDimsHost *)self.bufDims.contents;
    d->n = self.batch;
    d->arch_len = self.archLen;
    d->num_links = self.numLinks;
    d->top_dim = self.arch[self.archLen - 1u];
    d->target_dim = self.targetDim;
    d->z_total = self.zTotal;
    d->pred_total = self.predTotal;
    d->w_total = self.wTotal;
    d->r_total = self.rTotal;
    d->use_precision = self.config.use_precision ? 1u : 0u;
    d->temporal_weight = self.config.temporal_weight;
    d->latent_decay = self.config.latent_decay;
    d->sparsity = self.config.sparsity;
    d->state_clip = self.config.state_clip;
    d->grad_clip = self.config.grad_clip;
    d->early_stop_tol = self.config.early_stop_tol;

    uint32_t *hp = (uint32_t *)self.bufHasPrev.contents;
    hp[0] = self.hasPrevTop ? 1u : 0u;
}

@end
