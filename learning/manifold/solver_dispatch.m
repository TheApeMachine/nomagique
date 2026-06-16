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

// one threadgroup per column (N groups, kReduceThreads each).
- (void)encReduce:(id<MTLComputeCommandEncoder>)encoder
             pipe:(id<MTLComputePipelineState>)pipeline
          buffers:(NSArray<id<MTLBuffer>> *)buffers
          offsets:(const NSUInteger *)offsets
           consts:(NSArray<NSArray *> *)consts
          columns:(NSUInteger)columns {
    [self enc:encoder pipe:pipeline buffers:buffers offsets:offsets consts:consts
      threads:0 perGroup:kReduceThreads groups:columns];
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
