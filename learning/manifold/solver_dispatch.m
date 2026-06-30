//go:build darwin && cgo
// +build darwin,cgo

#import "solver_private.h"

@implementation BatchResonanceSolver (Pipelines)

static const uint32_t ResonanceFusedScalarCapacity = 512u;

static NSUInteger resonance_fused_scratch_bytes(uint32_t capacity) {
    return (NSUInteger)capacity * 11u * sizeof(float) +
        4u * sizeof(float) +
        2u * sizeof(uint32_t);
}

- (id<MTLComputePipelineState>)pipe:(NSString *)name error:(NSError **)error {
    id<MTLFunction> fn = [self.library newFunctionWithName:name];
    if (fn == nil) {
        if (error) *error = [NSError errorWithDomain:@"resonance" code:1
            userInfo:@{NSLocalizedDescriptionKey: [NSString stringWithFormat:@"kernel %@ not found", name]}];
        return nil;
    }
    return [self.device newComputePipelineStateWithFunction:fn error:error];
}

- (uint32_t)settleFusedCapacity {
    uint32_t capacity = self.zTotal;
    if (self.predTotal > capacity) capacity = self.predTotal;
    if (self.maxDim > capacity) capacity = self.maxDim;

    uint32_t top = self.arch[self.archLen - 1u];
    if (top > capacity) capacity = top;
    if (capacity == 0u) capacity = 1u;

    return capacity;
}

- (BOOL)validateSettleFusedCapacity:(uint32_t)capacity error:(NSError **)error {
    if (capacity > ResonanceFusedScalarCapacity) {
        if (error) {
            NSString *message = [NSString stringWithFormat:
                @"fused settle capacity %u exceeds compiled capacity %u",
                capacity,
                ResonanceFusedScalarCapacity];
            *error = [NSError errorWithDomain:@"resonance" code:1
                userInfo:@{NSLocalizedDescriptionKey: message}];
        }

        return NO;
    }

    NSUInteger required = resonance_fused_scratch_bytes(ResonanceFusedScalarCapacity);
    NSUInteger limit = self.device.maxThreadgroupMemoryLength;

    if (required <= limit) {
        return YES;
    }

    if (error) {
        NSString *message = [NSString stringWithFormat:
            @"fused settle scratch %lu bytes for capacity %u exceeds Metal threadgroup limit %lu",
            (unsigned long)required,
            capacity,
            (unsigned long)limit];
        *error = [NSError errorWithDomain:@"resonance" code:1
            userInfo:@{NSLocalizedDescriptionKey: message}];
    }

    return NO;
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
    if (e == nil && [self validateSettleFusedCapacity:[self settleFusedCapacity] error:&e]) {
        self.pSettleFused = [self pipe:@"bsettle_fused" error:&e];
    }

    if (e != nil) { if (error) *error = e.localizedDescription ?: @"pipeline build failed"; return NO; }
    return YES;
}

@end

@implementation BatchResonanceSolver (Dispatch)

/*
encRaw is the hot dispatch path. It uses stack C arrays for buffers/constants, so
settle/learn kernels no longer allocate NSArray/NSNumber objects just to encode
scalar arguments. A compatibility NSArray wrapper remains below for non-hot paths.
*/
- (void)encRaw:(id<MTLComputeCommandEncoder>)encoder
          pipe:(id<MTLComputePipelineState>)pipeline
       buffers:(id<MTLBuffer> __unsafe_unretained *)buffers
   bufferCount:(NSUInteger)bufferCount
       offsets:(const NSUInteger *)offsets
        consts:(const ResonanceConst *)consts
    constCount:(NSUInteger)constCount
       threads:(NSUInteger)threads
      perGroup:(NSUInteger)perGroup
        groups:(NSUInteger)groups {
    [encoder setComputePipelineState:pipeline];

    NSUInteger idx = 0;
    for (; idx < bufferCount; idx++) {
        NSUInteger off = (offsets != NULL) ? offsets[idx] : 0;
        [encoder setBuffer:buffers[idx] offset:off atIndex:idx];
    }

    for (NSUInteger c = 0; c < constCount; c++) {
        ResonanceConst constant = consts[c];
        if (constant.kind == ResonanceConstKindFloat) {
            float v = constant.f;
            [encoder setBytes:&v length:sizeof(float) atIndex:idx++];
        } else {
            uint32_t v = constant.u;
            [encoder setBytes:&v length:sizeof(uint32_t) atIndex:idx++];
        }
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
- (void)encRaw:(id<MTLComputeCommandEncoder>)encoder
          pipe:(id<MTLComputePipelineState>)pipeline
       buffers:(id<MTLBuffer> __unsafe_unretained *)buffers
   bufferCount:(NSUInteger)bufferCount
       offsets:(const NSUInteger *)offsets
        consts:(const ResonanceConst *)consts
    constCount:(NSUInteger)constCount
       threads:(NSUInteger)threads {
    [self encRaw:encoder pipe:pipeline buffers:buffers bufferCount:bufferCount
          offsets:offsets consts:consts constCount:constCount
          threads:threads perGroup:0 groups:0];
}

// Compatibility wrapper for less performance-sensitive call sites.
- (void)enc:(id<MTLComputeCommandEncoder>)encoder
       pipe:(id<MTLComputePipelineState>)pipeline
    buffers:(NSArray<id<MTLBuffer>> *)buffers
    offsets:(const NSUInteger *)offsets
     consts:(NSArray<NSArray *> *)consts
    threads:(NSUInteger)threads
   perGroup:(NSUInteger)perGroup
     groups:(NSUInteger)groups {
    id<MTLBuffer> __unsafe_unretained rawBuffers[16];
    ResonanceConst rawConsts[16];

    NSUInteger bufferCount = buffers.count;
    if (bufferCount > 16) bufferCount = 16;
    for (NSUInteger i = 0; i < bufferCount; i++) rawBuffers[i] = buffers[i];

    NSUInteger constCount = consts.count;
    if (constCount > 16) constCount = 16;
    for (NSUInteger c = 0; c < constCount; c++) {
        NSString *type = consts[c][0];
        NSNumber *num = consts[c][1];
        rawConsts[c] = [type isEqualToString:@"f"] ? ResF(num.floatValue) : ResU(num.unsignedIntValue);
    }

    [self encRaw:encoder pipe:pipeline buffers:rawBuffers bufferCount:bufferCount
          offsets:offsets consts:rawConsts constCount:constCount
          threads:threads perGroup:perGroup groups:groups];
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
- (void)encReduceRaw:(id<MTLComputeCommandEncoder>)encoder
                pipe:(id<MTLComputePipelineState>)pipeline
             buffers:(id<MTLBuffer> __unsafe_unretained *)buffers
         bufferCount:(NSUInteger)bufferCount
             offsets:(const NSUInteger *)offsets
              consts:(const ResonanceConst *)consts
          constCount:(NSUInteger)constCount
             columns:(NSUInteger)columns
           reduceLen:(NSUInteger)reduceLen {
    NSUInteger perGroup = [self reduceThreadgroupSizeFor:reduceLen pipeline:pipeline];
    [self encRaw:encoder pipe:pipeline buffers:buffers bufferCount:bufferCount
          offsets:offsets consts:consts constCount:constCount
          threads:0 perGroup:perGroup groups:columns];
}

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

    // Fused settle parameters
    d->max_inference_steps = self.config.max_inference_steps;
    d->min_inference_steps = self.config.min_inference_steps;
    d->line_search_halvings = self.config.monotone_state_steps ? self.config.line_search_halvings : 0u;
    d->monotone_state_steps = self.config.monotone_state_steps ? 1u : 0u;
    d->lr_state = self.config.lr_state;

    uint32_t *hp = (uint32_t *)self.bufHasPrev.contents;
    hp[0] = self.hasPrevTop ? 1u : 0u;
}

@end
