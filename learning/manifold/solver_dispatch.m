#import "solver_private.h"

void resonance_write_error(char *err_out, int err_cap, NSString *message) {
    if (err_out == NULL || err_cap <= 0) {
        return;
    }

    const char *utf8 = message.UTF8String;

    if (utf8 == NULL) {
        err_out[0] = '\0';
        return;
    }

    strncpy(err_out, utf8, (size_t)err_cap - 1);
    err_out[err_cap - 1] = '\0';
}

@implementation ResonanceSolver (Pipelines)

- (id<MTLComputePipelineState>)pipelineNamed:(NSString *)name error:(NSError **)error {
    id<MTLFunction> function = [self.library newFunctionWithName:name];

    if (function == nil) {
        if (error != nil) {
            *error = [NSError errorWithDomain:@"resonance"
                                         code:1
                                     userInfo:@{NSLocalizedDescriptionKey:
                                         [NSString stringWithFormat:@"kernel %@ not found", name]}];
        }

        return nil;
    }

    return [self.device newComputePipelineStateWithFunction:function error:error];
}

- (BOOL)buildPipelines:(NSString **)error {
    NSError *err = nil;

    self.pGemv = [self pipelineNamed:@"gemv" error:&err];
    self.pGemvTanh = [self pipelineNamed:@"gemv_tanh" error:&err];
    self.pGemvTranspose = [self pipelineNamed:@"gemv_transpose" error:&err];
    self.pVecCopy = [self pipelineNamed:@"vec_copy" error:&err];
    self.pVecSub = [self pipelineNamed:@"vec_sub" error:&err];
    self.pVecAdd = [self pipelineNamed:@"vec_add" error:&err];
    self.pVecMulElem = [self pipelineNamed:@"vec_mulelem" error:&err];
    self.pVecScale = [self pipelineNamed:@"vec_scale" error:&err];
    self.pVecAxpy = [self pipelineNamed:@"vec_axpy" error:&err];
    self.pVecClamp = [self pipelineNamed:@"vec_clamp" error:&err];
    self.pTanhDeriv = [self pipelineNamed:@"tanh_deriv" error:&err];
    self.pMergeClamp = [self pipelineNamed:@"merge_clamp" error:&err];
    self.pSparsitySubgrad = [self pipelineNamed:@"sparsity_subgrad" error:&err];
    self.pPrecisionUpdate = [self pipelineNamed:@"precision_update" error:&err];
    self.pOuterUpdate = [self pipelineNamed:@"outer_update" error:&err];
    self.pReduceDot = [self pipelineNamed:@"reduce_dot" error:&err];
    self.pReduceAbsSum = [self pipelineNamed:@"reduce_abs_sum" error:&err];
    self.pGradClip = [self pipelineNamed:@"grad_clip" error:&err];

    if (err != nil) {
        if (error != nil) {
            *error = err.localizedDescription ?: @"pipeline construction failed";
        }

        return NO;
    }

    return YES;
}

@end

@implementation ResonanceSolver (Dispatch)

- (id<MTLBuffer>)constantUint:(uint32_t)value {
    return [self.device newBufferWithBytes:&value length:sizeof(uint32_t) options:MTLResourceStorageModeShared];
}

- (id<MTLBuffer>)constantFloat:(float)value {
    return [self.device newBufferWithBytes:&value length:sizeof(float) options:MTLResourceStorageModeShared];
}

- (void)dispatch1D:(id<MTLComputePipelineState>)pipeline
           buffers:(NSArray<id<MTLBuffer>> *)buffers
           offsets:(NSArray<NSNumber *> *)offsets
       threadCount:(NSUInteger)threadCount {
    if (threadCount == 0) {
        return;
    }

    id<MTLCommandBuffer> commandBuffer = [self.queue commandBuffer];
    id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];
    [encoder setComputePipelineState:pipeline];

    for (NSUInteger index = 0; index < buffers.count; index++) {
        NSUInteger offset = (offsets != nil && index < offsets.count) ? offsets[index].unsignedIntegerValue : 0;
        [encoder setBuffer:buffers[index] offset:offset atIndex:index];
    }

    NSUInteger width = pipeline.maxTotalThreadsPerThreadgroup;

    if (width > threadCount) {
        width = threadCount;
    }

    MTLSize grid = MTLSizeMake(threadCount, 1, 1);
    MTLSize group = MTLSizeMake(width, 1, 1);
    [encoder dispatchThreads:grid threadsPerThreadgroup:group];
    [encoder endEncoding];
    [commandBuffer commit];
    [commandBuffer waitUntilCompleted];
}

- (void)dispatchReduce:(id<MTLComputePipelineState>)pipeline
               buffers:(NSArray<id<MTLBuffer>> *)buffers
               offsets:(NSArray<NSNumber *> *)offsets {
    id<MTLCommandBuffer> commandBuffer = [self.queue commandBuffer];
    id<MTLComputeCommandEncoder> encoder = [commandBuffer computeCommandEncoder];
    [encoder setComputePipelineState:pipeline];

    for (NSUInteger index = 0; index < buffers.count; index++) {
        NSUInteger offset = (offsets != nil && index < offsets.count) ? offsets[index].unsignedIntegerValue : 0;
        [encoder setBuffer:buffers[index] offset:offset atIndex:index];
    }

    MTLSize grid = MTLSizeMake(kReduceThreads, 1, 1);
    MTLSize group = MTLSizeMake(kReduceThreads, 1, 1);
    [encoder dispatchThreadgroups:MTLSizeMake(1, 1, 1) threadsPerThreadgroup:group];
    (void)grid;
    [encoder endEncoding];
    [commandBuffer commit];
    [commandBuffer waitUntilCompleted];
}

@end
