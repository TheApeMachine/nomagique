#import "solver_private.h"
#include <dispatch/dispatch.h>

void resonance_write_error(char *err_out, int err_cap, NSString *message) {
    if (err_out == NULL || err_cap <= 0) return;
    const char *utf8 = message.UTF8String;
    if (utf8 == NULL) { err_out[0] = '\0'; return; }
    strncpy(err_out, utf8, (size_t)err_cap - 1);
    err_out[err_cap - 1] = '\0';
}

// Primary implementation anchors the class symbol; behavior lives in categories.
@implementation BatchResonanceSolver
@end

@implementation BatchResonanceSolver (Lifecycle)

- (instancetype)initWithConfig:(const ResonanceConfig *)config
                          arch:(const uint32_t *)arch
                       archLen:(uint32_t)archLen
                     targetDim:(uint32_t)targetDim
                         batch:(uint32_t)batch
                 metallibBytes:(const void *)metallibBytes
                metallibLength:(size_t)metallibLength
                         error:(NSString **)error {
    self = [super init];
    if (self == nil) return nil;

    if (archLen < 2) { if (error) *error = @"architecture must contain at least input and one latent layer"; return nil; }
    if (batch == 0u) { if (error) *error = @"batch size must be positive"; return nil; }

    self.device = MTLCreateSystemDefaultDevice();
    if (self.device == nil) { if (error) *error = @"Metal device unavailable"; return nil; }

    self.queue = [self.device newCommandQueue];
    self.config = *config;
    self.archLen = archLen;
    self.numLinks = archLen - 1u;
    self.targetDim = targetDim;
    self.batch = batch;
    self.hasPrevTop = NO;

    if (metallibBytes == NULL || metallibLength == 0) { if (error) *error = @"metallib payload is empty"; return nil; }

    NSError *metalError = nil;
    dispatch_data_t data = dispatch_data_create(metallibBytes, metallibLength,
        dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0), DISPATCH_DATA_DESTRUCTOR_DEFAULT);
    self.library = [self.device newLibraryWithData:data error:&metalError];
    if (self.library == nil) { if (error) *error = metalError.localizedDescription ?: @"failed to load kernels.metallib"; return nil; }

    // Layout first: the tiled GEMV pipeline specializes on maxDim (cols cap).
    [self computeLayout:arch];

    if (![self buildPipelines:error]) return nil;

    [self allocateBuffers];
    [self buildLayoutBuffers];

    return self;
}

- (void)computeLayout:(const uint32_t *)arch {
    uint32_t archLen = self.archLen, numLinks = self.numLinks;
    self.arch = (uint32_t *)calloc(archLen, sizeof(uint32_t));
    self.zOffset = (uint32_t *)calloc(archLen, sizeof(uint32_t));
    self.wOffset = (uint32_t *)calloc(numLinks, sizeof(uint32_t));
    self.rOffset = (uint32_t *)calloc(numLinks, sizeof(uint32_t));
    self.predOffset = (uint32_t *)calloc(numLinks, sizeof(uint32_t));

    uint32_t zc = 0u, maxDim = 0u;
    for (uint32_t l = 0u; l < archLen; ++l) {
        self.arch[l] = arch[l];
        self.zOffset[l] = zc;
        zc += arch[l];
        if (arch[l] > maxDim) maxDim = arch[l];
    }
    self.zTotal = zc;
    self.maxDim = maxDim;

    uint32_t wc = 0u, rc = 0u, pc = 0u;
    for (uint32_t l = 0u; l < numLinks; ++l) {
        uint32_t rows = arch[l], cols = arch[l + 1u];
        self.wOffset[l] = wc; wc += rows * cols;
        self.rOffset[l] = rc; rc += cols * rows;
        self.predOffset[l] = pc; pc += rows;
    }
    self.wTotal = wc;
    self.rTotal = rc;
    self.predTotal = pc;

    uint32_t top = arch[archLen - 1u];
    self.aTotal = top * top;
    self.vTotal = (self.targetDim > 0u) ? self.targetDim * top : 0u;
}

- (id<MTLBuffer>)floats:(uint32_t)count {
    size_t len = (size_t)(count == 0u ? 1u : count) * sizeof(float);
    id<MTLBuffer> b = [self.device newBufferWithLength:len options:MTLResourceStorageModeShared];
    memset(b.contents, 0, len);
    return b;
}

- (void)fill:(id<MTLBuffer>)b count:(uint32_t)count value:(float)value {
    float *d = (float *)b.contents;
    for (uint32_t i = 0u; i < count; ++i) d[i] = value;
}

- (void)allocateBuffers {
    uint32_t N = self.batch;
    uint32_t top = self.arch[self.archLen - 1u];

    self.bufW = [self floats:self.wTotal * N];
    self.bufR = [self floats:self.rTotal * N];
    self.bufA = [self floats:self.aTotal * N];
    self.bufV = (self.vTotal > 0u) ? [self floats:self.vTotal * N] : nil;

    self.bufZ = [self floats:self.zTotal * N];
    self.bufZSaved = [self floats:self.zTotal * N];
    self.bufPrevTop = [self floats:top * N];
    self.bufPred = [self floats:self.predTotal * N];
    self.bufErr = [self floats:self.predTotal * N];
    self.bufGradCache = [self floats:self.zTotal * N];

    self.bufPrecision = [self floats:self.predTotal * N];
    self.bufVariance = [self floats:self.predTotal * N];
    [self fill:self.bufPrecision count:self.predTotal * N value:1.0f];
    [self fill:self.bufVariance count:self.predTotal * N value:1.0f];

    self.bufTemporalPrec = [self floats:top * N];
    self.bufTemporalVar = [self floats:top * N];
    self.bufTemporalErr = [self floats:top * N];
    [self fill:self.bufTemporalPrec count:top * N value:1.0f];
    [self fill:self.bufTemporalVar count:top * N value:1.0f];

    if (self.targetDim > 0u) {
        self.bufTaskPrec = [self floats:self.targetDim * N];
        self.bufTaskVar = [self floats:self.targetDim * N];
        [self fill:self.bufTaskPrec count:self.targetDim * N value:1.0f];
        [self fill:self.bufTaskVar count:self.targetDim * N value:1.0f];
        self.bufTarget = [self floats:self.targetDim * N];
    }

    self.bufBottomUp = [self floats:self.zTotal * N];
    self.bufTopDown = [self floats:self.zTotal * N];
    self.bufTmpA = [self floats:self.maxDim * N];
    self.bufTmpB = [self floats:self.maxDim * N];
    self.bufTmpC = [self floats:self.maxDim * N];
    self.bufFactor = [self floats:N];

    self.bufStep = [self floats:N];
    self.bufEnergyOld = [self floats:N];
    self.bufEnergyNew = [self floats:N];
    self.bufFlags = [self.device newBufferWithLength:N * sizeof(uint32_t) options:MTLResourceStorageModeShared];
    self.bufActive = [self.device newBufferWithLength:N * sizeof(uint32_t) options:MTLResourceStorageModeShared];
    self.bufAnyActive = [self.device newBufferWithLength:sizeof(uint32_t) options:MTLResourceStorageModeShared];
    self.startSnapshot = (float *)calloc(N, sizeof(float));
}

- (void)buildLayoutBuffers {
    uint32_t archLen = self.archLen, numLinks = self.numLinks;
    self.bufArchDim = [self.device newBufferWithBytes:self.arch length:archLen * sizeof(uint32_t) options:MTLResourceStorageModeShared];
    self.bufZOff = [self.device newBufferWithBytes:self.zOffset length:archLen * sizeof(uint32_t) options:MTLResourceStorageModeShared];
    self.bufPredOff = [self.device newBufferWithBytes:self.predOffset length:(numLinks ? numLinks : 1u) * sizeof(uint32_t) options:MTLResourceStorageModeShared];

    self.bufLayerRow = [self.device newBufferWithLength:self.zTotal * sizeof(uint32_t) options:MTLResourceStorageModeShared];
    uint32_t *lr = (uint32_t *)self.bufLayerRow.contents;
    for (uint32_t l = 0u; l < archLen; ++l) {
        for (uint32_t i = 0u; i < self.arch[l]; ++i) lr[self.zOffset[l] + i] = l;
    }

    self.bufHasPrev = [self.device newBufferWithLength:sizeof(uint32_t) options:MTLResourceStorageModeShared];
    self.bufDims = [self.device newBufferWithLength:sizeof(BatchDimsHost) options:MTLResourceStorageModeShared];
    [self syncDims];
}

- (BOOL)seedSlot:(uint32_t)slot
               w:(const float *)w wLen:(size_t)wLen
               r:(const float *)r rLen:(size_t)rLen
               a:(const float *)a aLen:(size_t)aLen
               v:(const float *)v vLen:(size_t)vLen
           error:(NSString **)errorOut {
    if (slot >= self.batch) { if (errorOut) *errorOut = @"slot out of range"; return NO; }

    if (w) {
        if (wLen != self.wTotal) { if (errorOut) *errorOut = @"W seed length mismatch"; return NO; }
        memcpy((float *)self.bufW.contents + (size_t)self.wTotal * slot, w, wLen * sizeof(float));
    }
    if (r) {
        if (rLen != self.rTotal) { if (errorOut) *errorOut = @"R seed length mismatch"; return NO; }
        memcpy((float *)self.bufR.contents + (size_t)self.rTotal * slot, r, rLen * sizeof(float));
    }
    if (a) {
        if (aLen != self.aTotal) { if (errorOut) *errorOut = @"A seed length mismatch"; return NO; }
        memcpy((float *)self.bufA.contents + (size_t)self.aTotal * slot, a, aLen * sizeof(float));
    }
    if (v && self.bufV) {
        if (vLen != self.vTotal) { if (errorOut) *errorOut = @"V seed length mismatch"; return NO; }
        memcpy((float *)self.bufV.contents + (size_t)self.vTotal * slot, v, vLen * sizeof(float));
    }
    return YES;
}

- (void)readSlot:(uint32_t)slot w:(float *)w r:(float *)r a:(float *)a v:(float *)v {
    if (slot >= self.batch) return;
    if (w) memcpy(w, (float *)self.bufW.contents + (size_t)self.wTotal * slot, (size_t)self.wTotal * sizeof(float));
    if (r) memcpy(r, (float *)self.bufR.contents + (size_t)self.rTotal * slot, (size_t)self.rTotal * sizeof(float));
    if (a) memcpy(a, (float *)self.bufA.contents + (size_t)self.aTotal * slot, (size_t)self.aTotal * sizeof(float));
    if (v && self.bufV) memcpy(v, (float *)self.bufV.contents + (size_t)self.vTotal * slot, (size_t)self.vTotal * sizeof(float));
}

// Write column `slot` of layer-0 latent (the input) and the target column.
- (void)setInputSlot:(uint32_t)slot input:(const float *)input
              target:(const float *)target hasTarget:(BOOL)hasTarget {
    if (slot >= self.batch) return;
    uint32_t N = self.batch;
    uint32_t dim0 = self.arch[0];
    float *z = (float *)self.bufZ.contents;
    uint32_t base = self.zOffset[0] * N;
    for (uint32_t r = 0u; r < dim0; ++r) z[base + r * N + slot] = input[r];

    if (hasTarget && self.bufTarget && target) {
        float *t = (float *)self.bufTarget.contents;
        for (uint32_t r = 0u; r < self.targetDim; ++r) t[r * N + slot] = target[r];
    }
}

- (void)readLatentSlot:(uint32_t)slot out:(float *)out length:(uint32_t)length {
    if (slot >= self.batch) return;
    uint32_t N = self.batch;
    uint32_t topIndex = self.archLen - 1u;
    uint32_t top = self.arch[topIndex];
    uint32_t count = (length < top) ? length : top;
    const float *z = (const float *)self.bufZ.contents;
    uint32_t base = self.zOffset[topIndex] * N;
    for (uint32_t r = 0u; r < count; ++r) out[r] = z[base + r * N + slot];
}

- (void)resetState:(BOOL)resetPrecision {
    memset(self.bufZ.contents, 0, (size_t)self.zTotal * self.batch * sizeof(float));
    self.hasPrevTop = NO;
    if (resetPrecision) {
        uint32_t N = self.batch, top = self.arch[self.archLen - 1u];
        [self fill:self.bufVariance count:self.predTotal * N value:1.0f];
        [self fill:self.bufPrecision count:self.predTotal * N value:1.0f];
        [self fill:self.bufTemporalVar count:top * N value:1.0f];
        [self fill:self.bufTemporalPrec count:top * N value:1.0f];
        if (self.targetDim > 0u) {
            [self fill:self.bufTaskVar count:self.targetDim * N value:1.0f];
            [self fill:self.bufTaskPrec count:self.targetDim * N value:1.0f];
        }
    }
}

- (void)dealloc {
    free(self.arch); free(self.zOffset); free(self.wOffset);
    free(self.rOffset); free(self.predOffset);
    free(self.startSnapshot);
}

@end
