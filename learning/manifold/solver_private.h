#pragma once

#import <Foundation/Foundation.h>
#import <Metal/Metal.h>
#include "bridge.h"
#include <math.h>
#include <string.h>

static const uint32_t kReduceThreads = 256u;

/*
ResonanceSolver runs the predictive-coding resonance manifold on Metal.

State is stored as flat float buffers with per-layer offset tables so a single
class handles arbitrary architectures. The host orchestrates the settle
line-search and the learning weight updates; kernels do the per-element work.
*/
@interface ResonanceSolver : NSObject

@property (nonatomic, strong) id<MTLDevice> device;
@property (nonatomic, strong) id<MTLCommandQueue> queue;
@property (nonatomic, strong) id<MTLLibrary> library;

@property (nonatomic, assign) ResonanceConfig config;
@property (nonatomic, assign) uint32_t archLen;     // number of layers
@property (nonatomic, assign) uint32_t numLinks;    // archLen - 1
@property (nonatomic, assign) uint32_t targetDim;
@property (nonatomic, assign) BOOL hasPrevTop;      // temporal prior active

// Architecture and offset tables (host memory).
@property (nonatomic, assign) uint32_t *arch;       // [archLen]
@property (nonatomic, assign) uint32_t *zOffset;    // [archLen] latent vec offsets
@property (nonatomic, assign) uint32_t *wOffset;    // [numLinks] weight matrix offsets
@property (nonatomic, assign) uint32_t *rOffset;    // [numLinks] recognition matrix offsets
@property (nonatomic, assign) uint32_t zTotal;      // total latent scalars
@property (nonatomic, assign) uint32_t wTotal;      // total generative weight scalars
@property (nonatomic, assign) uint32_t rTotal;      // total recognition weight scalars

// Pipelines.
@property (nonatomic, strong) id<MTLComputePipelineState> pGemv;
@property (nonatomic, strong) id<MTLComputePipelineState> pGemvTanh;
@property (nonatomic, strong) id<MTLComputePipelineState> pGemvTranspose;
@property (nonatomic, strong) id<MTLComputePipelineState> pVecCopy;
@property (nonatomic, strong) id<MTLComputePipelineState> pVecSub;
@property (nonatomic, strong) id<MTLComputePipelineState> pVecAdd;
@property (nonatomic, strong) id<MTLComputePipelineState> pVecMulElem;
@property (nonatomic, strong) id<MTLComputePipelineState> pVecScale;
@property (nonatomic, strong) id<MTLComputePipelineState> pVecAxpy;
@property (nonatomic, strong) id<MTLComputePipelineState> pVecClamp;
@property (nonatomic, strong) id<MTLComputePipelineState> pTanhDeriv;
@property (nonatomic, strong) id<MTLComputePipelineState> pMergeClamp;
@property (nonatomic, strong) id<MTLComputePipelineState> pSparsitySubgrad;
@property (nonatomic, strong) id<MTLComputePipelineState> pPrecisionUpdate;
@property (nonatomic, strong) id<MTLComputePipelineState> pOuterUpdate;
@property (nonatomic, strong) id<MTLComputePipelineState> pReduceDot;
@property (nonatomic, strong) id<MTLComputePipelineState> pReduceAbsSum;
@property (nonatomic, strong) id<MTLComputePipelineState> pGradClip;

// Weight buffers (flat, shared storage so host can seed/read).
@property (nonatomic, strong) id<MTLBuffer> bufW;   // generative  [wTotal]
@property (nonatomic, strong) id<MTLBuffer> bufR;   // recognition [rTotal]
@property (nonatomic, strong) id<MTLBuffer> bufA;   // temporal    [top x top]
@property (nonatomic, strong) id<MTLBuffer> bufV;   // task        [targetDim x top] (or nil)

// Latent state and saved copy for the line search.
@property (nonatomic, strong) id<MTLBuffer> bufZ;        // [zTotal]
@property (nonatomic, strong) id<MTLBuffer> bufZSaved;   // [zTotal]
@property (nonatomic, strong) id<MTLBuffer> bufPrevTop;  // [top]

// Per-link prediction/error scratch (flat over links, sized like z[0..numLinks-1]).
@property (nonatomic, strong) id<MTLBuffer> bufPred;     // predictions  [sum arch[l], l<numLinks]
@property (nonatomic, strong) id<MTLBuffer> bufErr;      // layer errors [same layout]
@property (nonatomic, assign) uint32_t *predOffset;      // [numLinks]
@property (nonatomic, assign) uint32_t predTotal;

// Precision / variance per link (same layout as pred), plus temporal & task.
@property (nonatomic, strong) id<MTLBuffer> bufPrecision;
@property (nonatomic, strong) id<MTLBuffer> bufVariance;
@property (nonatomic, strong) id<MTLBuffer> bufTemporalPrec;
@property (nonatomic, strong) id<MTLBuffer> bufTemporalVar;
@property (nonatomic, strong) id<MTLBuffer> bufTaskPrec;
@property (nonatomic, strong) id<MTLBuffer> bufTaskVar;

// Generic per-layer scratch buffers (sized to max layer dim).
@property (nonatomic, strong) id<MTLBuffer> bufTmpA;     // [maxDim]
@property (nonatomic, strong) id<MTLBuffer> bufTmpB;     // [maxDim]
@property (nonatomic, strong) id<MTLBuffer> bufTmpC;     // [maxDim]
@property (nonatomic, strong) id<MTLBuffer> bufGrad;     // [maxDim]
@property (nonatomic, strong) id<MTLBuffer> bufBottomUp; // [zTotal] init pass
@property (nonatomic, strong) id<MTLBuffer> bufTopDown;  // [zTotal] init pass
@property (nonatomic, strong) id<MTLBuffer> bufInput;    // [arch[0]]
@property (nonatomic, strong) id<MTLBuffer> bufTarget;   // [targetDim]
@property (nonatomic, strong) id<MTLBuffer> bufScalar;   // [1] reduction output
@property (nonatomic, assign) uint32_t maxDim;

@end

@interface ResonanceSolver (Pipelines)
- (id<MTLComputePipelineState>)pipelineNamed:(NSString *)name error:(NSError **)error;
- (BOOL)buildPipelines:(NSString **)error;
@end

@interface ResonanceSolver (Dispatch)
// One-shot dispatch helpers (each owns a command buffer, commits, waits).
- (void)dispatch1D:(id<MTLComputePipelineState>)pipeline
           buffers:(NSArray<id<MTLBuffer>> *)buffers
           offsets:(NSArray<NSNumber *> *)offsets
       threadCount:(NSUInteger)threadCount;
- (void)dispatchReduce:(id<MTLComputePipelineState>)pipeline
               buffers:(NSArray<id<MTLBuffer>> *)buffers
               offsets:(NSArray<NSNumber *> *)offsets;
- (id<MTLBuffer>)constantUint:(uint32_t)value;
- (id<MTLBuffer>)constantFloat:(float)value;
@end

// Helpers implemented in solver_darwin.m, shared with the Compute category.
@interface ResonanceSolver (Layout)
- (instancetype)initWithConfig:(const ResonanceConfig *)config
                          arch:(const uint32_t *)arch
                       archLen:(uint32_t)archLen
                     targetDim:(uint32_t)targetDim
                 metallibBytes:(const void *)metallibBytes
                metallibLength:(size_t)metallibLength
                         error:(NSString **)error;
- (void)fillFloats:(id<MTLBuffer>)buffer count:(uint32_t)count value:(float)value;
- (BOOL)seedW:(const float *)w len:(size_t)wLen
            r:(const float *)r len:(size_t)rLen
            a:(const float *)a len:(size_t)aLen
            v:(const float *)v len:(size_t)vLen
        error:(NSString **)errorOut;
- (void)readWeightsW:(float *)w r:(float *)r a:(float *)a v:(float *)v;
- (void)readLatent:(float *)out length:(uint32_t)length;
@end

@interface ResonanceSolver (Compute)
- (void)predictAdjacentLayers;     // fills bufPred/bufErr from current z and W
- (void)initializeLatents;         // bottom-up + top-down init into z
- (float)energy;
- (float)reconstructionError;
- (void)settleInput:(const float *)input advanceTemporal:(BOOL)advanceTemporal;
- (void)learnTarget:(const float *)target hasTarget:(BOOL)hasTarget;
- (void)advanceTemporalState;
- (void)resetState:(BOOL)resetPrecision;
@end

void resonance_write_error(char *err_out, int err_cap, NSString *message);
