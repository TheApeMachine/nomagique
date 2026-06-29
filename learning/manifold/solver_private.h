#pragma once

#import <Foundation/Foundation.h>
#import <Metal/Metal.h>
#include "bridge.h"
#include <math.h>
#include <string.h>

// Reduction threadgroup sizes are derived per-dispatch from the actual reduction
// length and the pipeline's queried maxTotalThreadsPerThreadgroup (see
// -reduceThreadgroupSizeFor:pipeline:), not hardcoded.


// Lightweight scalar constant representation for hot kernel-dispatch paths.
// This avoids allocating Objective-C NSArray/NSNumber objects for every encoded
// Metal kernel in settle/learn loops.
typedef enum ResonanceConstKind {
    ResonanceConstKindUInt = 0,
    ResonanceConstKindFloat = 1,
} ResonanceConstKind;

typedef struct ResonanceConst {
    ResonanceConstKind kind;
    uint32_t u;
    float f;
} ResonanceConst;

static inline ResonanceConst ResU(uint32_t value) {
    ResonanceConst c;
    c.kind = ResonanceConstKindUInt;
    c.u = value;
    c.f = 0.0f;
    return c;
}

static inline ResonanceConst ResF(float value) {
    ResonanceConst c;
    c.kind = ResonanceConstKindFloat;
    c.u = 0u;
    c.f = value;
    return c;
}

/*
BatchDimsHost mirrors the BatchDims struct in resonance.metal — the per-symbol
dimensions/config the batched kernels read to self-navigate the flat buffers.
*/
typedef struct BatchDimsHost {
    uint32_t n;
    uint32_t arch_len;
    uint32_t num_links;
    uint32_t top_dim;
    uint32_t target_dim;
    uint32_t z_total;     // per-symbol latent scalars
    uint32_t pred_total;  // per-symbol prediction scalars
    uint32_t w_total;     // per-symbol generative weight scalars
    uint32_t r_total;     // per-symbol recognition weight scalars
    uint32_t use_precision;
    float temporal_weight;
    float latent_decay;
    float sparsity;
    float state_clip;
    float grad_clip;
    float early_stop_tol;

    // Fused settle parameters
    uint32_t max_inference_steps;
    uint32_t min_inference_steps;
    uint32_t line_search_halvings;
    uint32_t monotone_state_steps;
    float lr_state;
} BatchDimsHost;

/*
BatchResonanceSolver settles N independent predictive-coding manifolds in
lockstep on Metal. State layout: per layer l, scalar (row r, slot s) at
z_off[l]*N + r*N + s. Weights are per-symbol, slot-major blocks. Line search and
early-stop run per column so symbols converge independently.
*/
@interface BatchResonanceSolver : NSObject

@property (nonatomic, strong) id<MTLDevice> device;
@property (nonatomic, strong) id<MTLCommandQueue> queue;
@property (nonatomic, strong) id<MTLLibrary> library;

@property (nonatomic, assign) ResonanceConfig config;
@property (nonatomic, assign) uint32_t archLen;
@property (nonatomic, assign) uint32_t numLinks;
@property (nonatomic, assign) uint32_t targetDim;
@property (nonatomic, assign) uint32_t batch;
@property (nonatomic, assign) BOOL hasPrevTop;

// Host-side layout tables.
@property (nonatomic, assign) uint32_t *arch;       // [archLen]
@property (nonatomic, assign) uint32_t *zOffset;    // [archLen] per-symbol scalar offsets
@property (nonatomic, assign) uint32_t *wOffset;    // [numLinks]
@property (nonatomic, assign) uint32_t *rOffset;    // [numLinks]
@property (nonatomic, assign) uint32_t *predOffset; // [numLinks]
@property (nonatomic, assign) uint32_t zTotal;
@property (nonatomic, assign) uint32_t predTotal;
@property (nonatomic, assign) uint32_t wTotal;
@property (nonatomic, assign) uint32_t rTotal;
@property (nonatomic, assign) uint32_t aTotal;      // top*top
@property (nonatomic, assign) uint32_t vTotal;      // target*top (0 if none)
@property (nonatomic, assign) uint32_t maxDim;

// Pipelines.
@property (nonatomic, strong) id<MTLComputePipelineState> pGemv;
@property (nonatomic, strong) id<MTLComputePipelineState> pGemvT;
@property (nonatomic, strong) id<MTLComputePipelineState> pSub;
@property (nonatomic, strong) id<MTLComputePipelineState> pMul;
@property (nonatomic, strong) id<MTLComputePipelineState> pCopy;
@property (nonatomic, strong) id<MTLComputePipelineState> pTanhDeriv;
@property (nonatomic, strong) id<MTLComputePipelineState> pAxpy;
@property (nonatomic, strong) id<MTLComputePipelineState> pScale;
@property (nonatomic, strong) id<MTLComputePipelineState> pSparsity;
@property (nonatomic, strong) id<MTLComputePipelineState> pEnergy;
@property (nonatomic, strong) id<MTLComputePipelineState> pGradClipLayer;
@property (nonatomic, strong) id<MTLComputePipelineState> pApplyState;
@property (nonatomic, strong) id<MTLComputePipelineState> pDecide;
@property (nonatomic, strong) id<MTLComputePipelineState> pRevert;
@property (nonatomic, strong) id<MTLComputePipelineState> pEarlyStop;
@property (nonatomic, strong) id<MTLComputePipelineState> pPrecision;
@property (nonatomic, strong) id<MTLComputePipelineState> pOuterFactor;
@property (nonatomic, strong) id<MTLComputePipelineState> pOuterApply;
@property (nonatomic, strong) id<MTLComputePipelineState> pMergeClamp;
@property (nonatomic, strong) id<MTLComputePipelineState> pSettleFused;

// Weights (per-symbol slot-major).
@property (nonatomic, strong) id<MTLBuffer> bufW;   // [wTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufR;   // [rTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufA;   // [aTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufV;   // [vTotal * N] or nil

// State (per layer, [dim x N]).
@property (nonatomic, strong) id<MTLBuffer> bufZ;        // [zTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufZSaved;   // [zTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufPrevTop;  // [top * N]
@property (nonatomic, strong) id<MTLBuffer> bufPred;     // [predTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufErr;      // [predTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufPrecision;// [predTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufVariance; // [predTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufTemporalPrec; // [top * N]
@property (nonatomic, strong) id<MTLBuffer> bufTemporalVar;  // [top * N]
@property (nonatomic, strong) id<MTLBuffer> bufTemporalErr;  // [top * N]
@property (nonatomic, strong) id<MTLBuffer> bufTaskPrec; // [target * N] or nil
@property (nonatomic, strong) id<MTLBuffer> bufTaskVar;  // [target * N] or nil
@property (nonatomic, strong) id<MTLBuffer> bufGradCache;// [zTotal * N]

// Init/scratch.
@property (nonatomic, strong) id<MTLBuffer> bufBottomUp; // [zTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufTopDown;  // [zTotal * N]
@property (nonatomic, strong) id<MTLBuffer> bufTmpA;     // [maxDim * N]
@property (nonatomic, strong) id<MTLBuffer> bufTmpB;     // [maxDim * N]
@property (nonatomic, strong) id<MTLBuffer> bufTmpC;     // [maxDim * N]
@property (nonatomic, strong) id<MTLBuffer> bufTarget;   // [target * N] or nil
@property (nonatomic, strong) id<MTLBuffer> bufFactor;   // [N]

// Per-column line-search scalars.
@property (nonatomic, strong) id<MTLBuffer> bufStep;       // [N]
@property (nonatomic, strong) id<MTLBuffer> bufEnergyOld;  // [N]
@property (nonatomic, strong) id<MTLBuffer> bufEnergyNew;  // [N]
@property (nonatomic, strong) id<MTLBuffer> bufReconstruction; // [N]
@property (nonatomic, strong) id<MTLBuffer> bufFlags;      // [N]
@property (nonatomic, strong) id<MTLBuffer> bufActive;     // [N]
@property (nonatomic, strong) id<MTLBuffer> bufAnyActive;  // [1] atomic counter
@property (nonatomic, assign) float *startSnapshot;       // [N] host copy of per-step start energy

// Layout buffers for kernels.
@property (nonatomic, strong) id<MTLBuffer> bufDims;       // BatchDimsHost
@property (nonatomic, strong) id<MTLBuffer> bufArchDim;    // [archLen]
@property (nonatomic, strong) id<MTLBuffer> bufZOff;       // [archLen]
@property (nonatomic, strong) id<MTLBuffer> bufPredOff;    // [numLinks]
@property (nonatomic, strong) id<MTLBuffer> bufWOff;       // [numLinks]
@property (nonatomic, strong) id<MTLBuffer> bufROff;       // [numLinks]
@property (nonatomic, strong) id<MTLBuffer> bufLayerRow;   // [zTotal] layer per latent row
@property (nonatomic, strong) id<MTLBuffer> bufHasPrev;    // [1]

@end

@interface BatchResonanceSolver (Lifecycle)
- (instancetype)initWithConfig:(const ResonanceConfig *)config
                          arch:(const uint32_t *)arch
                       archLen:(uint32_t)archLen
                     targetDim:(uint32_t)targetDim
                         batch:(uint32_t)batch
                 metallibBytes:(const void *)metallibBytes
                metallibLength:(size_t)metallibLength
                         error:(NSString **)error;
- (BOOL)seedSlot:(uint32_t)slot
               w:(const float *)w wLen:(size_t)wLen
               r:(const float *)r rLen:(size_t)rLen
               a:(const float *)a aLen:(size_t)aLen
               v:(const float *)v vLen:(size_t)vLen
           error:(NSString **)errorOut;
- (BOOL)seedAllSlotsW:(const float *)w wLen:(size_t)wLen
                    r:(const float *)r rLen:(size_t)rLen
                    a:(const float *)a aLen:(size_t)aLen
                    v:(const float *)v vLen:(size_t)vLen
                error:(NSString **)errorOut;
- (void)readSlot:(uint32_t)slot w:(float *)w r:(float *)r a:(float *)a v:(float *)v;
- (void)setInputSlot:(uint32_t)slot input:(const float *)input
              target:(const float *)target hasTarget:(BOOL)hasTarget;
- (void)setInputBatch:(const float *)inputs inputStride:(uint32_t)inputStride
               target:(const float *)targets targetStride:(uint32_t)targetStride
            hasTarget:(BOOL)hasTarget;
- (void)readLatentSlot:(uint32_t)slot out:(float *)out length:(uint32_t)length;
- (void)resetState:(BOOL)resetPrecision;
@end

@interface BatchResonanceSolver (Pipelines)
- (BOOL)buildPipelines:(NSString **)error;
@end

@interface BatchResonanceSolver (Dispatch)
- (void)encRaw:(id<MTLComputeCommandEncoder>)encoder
          pipe:(id<MTLComputePipelineState>)pipeline
       buffers:(id<MTLBuffer> __unsafe_unretained *)buffers
   bufferCount:(NSUInteger)bufferCount
       offsets:(const NSUInteger *)offsets
        consts:(const ResonanceConst *)consts
    constCount:(NSUInteger)constCount
       threads:(NSUInteger)threads
      perGroup:(NSUInteger)perGroup
        groups:(NSUInteger)groups;
- (void)encRaw:(id<MTLComputeCommandEncoder>)encoder
          pipe:(id<MTLComputePipelineState>)pipeline
       buffers:(id<MTLBuffer> __unsafe_unretained *)buffers
   bufferCount:(NSUInteger)bufferCount
       offsets:(const NSUInteger *)offsets
        consts:(const ResonanceConst *)consts
    constCount:(NSUInteger)constCount
       threads:(NSUInteger)threads;
- (void)encReduceRaw:(id<MTLComputeCommandEncoder>)encoder
                pipe:(id<MTLComputePipelineState>)pipeline
             buffers:(id<MTLBuffer> __unsafe_unretained *)buffers
         bufferCount:(NSUInteger)bufferCount
             offsets:(const NSUInteger *)offsets
              consts:(const ResonanceConst *)consts
          constCount:(NSUInteger)constCount
             columns:(NSUInteger)columns
           reduceLen:(NSUInteger)reduceLen;
- (void)enc:(id<MTLComputeCommandEncoder>)encoder
       pipe:(id<MTLComputePipelineState>)pipeline
    buffers:(NSArray<id<MTLBuffer>> *)buffers
    offsets:(const NSUInteger *)offsets
     consts:(NSArray<NSArray *> *)consts
    threads:(NSUInteger)threads;
- (void)encReduce:(id<MTLComputeCommandEncoder>)encoder
             pipe:(id<MTLComputePipelineState>)pipeline
          buffers:(NSArray<id<MTLBuffer>> *)buffers
          offsets:(const NSUInteger *)offsets
           consts:(NSArray<NSArray *> *)consts
          columns:(NSUInteger)columns
        reduceLen:(NSUInteger)reduceLen;
- (void)syncDims;
@end

@interface BatchResonanceSolver (Lifecycle2)
- (void)computeLayout:(const uint32_t *)arch;
- (void)allocateBuffers;
- (void)buildLayoutBuffers;
- (id<MTLBuffer>)floats:(uint32_t)count;
- (void)fill:(id<MTLBuffer>)b count:(uint32_t)count value:(float)value;
@end

@interface BatchResonanceSolver (Compute)
- (void)settleAdvanceTemporal:(BOOL)advanceTemporal;
- (void)learn;
- (void)advanceTemporal;
- (float)energySlotCompute:(uint32_t)slot;
- (float)reconstructionSlotCompute:(uint32_t)slot;
- (void)computeOutcomesBatch;
- (BOOL)readOutcomesBatchLatent:(float *)latent
                        latentLen:(uint32_t)latentLen
                           energy:(float *)energy
                         energyLen:(uint32_t)energyLen
                   reconstruction:(float *)reconstruction
                         reconLen:(uint32_t)reconLen
                            error:(NSString **)errorOut;
- (BOOL)readWireSlot:(uint32_t)slot
               state:(float *)state
            stateLen:(uint32_t)stateLen
          prediction:(float *)prediction
       predictionLen:(uint32_t)predictionLen
           errorNorm:(float *)errorNorm
        errorNormLen:(uint32_t)errorNormLen
                error:(NSString **)errorOut;
@end

void resonance_write_error(char *err_out, int err_cap, NSString *message);


