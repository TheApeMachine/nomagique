#import "solver_private.h"
#include <dispatch/dispatch.h>

@implementation ResonanceSolver

- (instancetype)initWithConfig:(const ResonanceConfig *)config
                          arch:(const uint32_t *)arch
                       archLen:(uint32_t)archLen
                     targetDim:(uint32_t)targetDim
                 metallibBytes:(const void *)metallibBytes
                metallibLength:(size_t)metallibLength
                         error:(NSString **)error {
    self = [super init];

    if (self == nil) {
        return nil;
    }

    if (archLen < 2) {
        if (error != nil) {
            *error = @"architecture must contain at least input and one latent layer";
        }
        return nil;
    }

    self.device = MTLCreateSystemDefaultDevice();

    if (self.device == nil) {
        if (error != nil) {
            *error = @"Metal device unavailable";
        }
        return nil;
    }

    self.queue = [self.device newCommandQueue];
    self.config = *config;
    self.archLen = archLen;
    self.numLinks = archLen - 1u;
    self.targetDim = targetDim;
    self.hasPrevTop = NO;

    if (metallibBytes == NULL || metallibLength == 0) {
        if (error != nil) {
            *error = @"metallib payload is empty";
        }
        return nil;
    }

    NSError *metalError = nil;
    dispatch_data_t metallibData = dispatch_data_create(
        metallibBytes,
        metallibLength,
        dispatch_get_global_queue(QOS_CLASS_DEFAULT, 0),
        DISPATCH_DATA_DESTRUCTOR_DEFAULT
    );
    self.library = [self.device newLibraryWithData:metallibData error:&metalError];

    if (self.library == nil) {
        if (error != nil) {
            *error = metalError.localizedDescription ?: @"failed to load kernels.metallib";
        }
        return nil;
    }

    if (![self buildPipelines:error]) {
        return nil;
    }

    [self computeLayout:arch];

    if (![self allocateBuffers:error]) {
        return nil;
    }

    return self;
}

- (void)computeLayout:(const uint32_t *)arch {
    uint32_t archLen = self.archLen;
    uint32_t numLinks = self.numLinks;

    self.arch = (uint32_t *)calloc(archLen, sizeof(uint32_t));
    self.zOffset = (uint32_t *)calloc(archLen, sizeof(uint32_t));
    self.wOffset = (uint32_t *)calloc(numLinks, sizeof(uint32_t));
    self.rOffset = (uint32_t *)calloc(numLinks, sizeof(uint32_t));
    self.predOffset = (uint32_t *)calloc(numLinks, sizeof(uint32_t));

    uint32_t zCursor = 0u;
    uint32_t maxDim = 0u;

    for (uint32_t l = 0u; l < archLen; ++l) {
        self.arch[l] = arch[l];
        self.zOffset[l] = zCursor;
        zCursor += arch[l];
        if (arch[l] > maxDim) {
            maxDim = arch[l];
        }
    }

    self.zTotal = zCursor;
    self.maxDim = maxDim;

    uint32_t wCursor = 0u;
    uint32_t rCursor = 0u;
    uint32_t predCursor = 0u;

    for (uint32_t l = 0u; l < numLinks; ++l) {
        uint32_t rows = arch[l];
        uint32_t cols = arch[l + 1u];

        self.wOffset[l] = wCursor;
        wCursor += rows * cols;     // W[l] is rows x cols
        self.rOffset[l] = rCursor;
        rCursor += cols * rows;     // R[l] is cols x rows
        self.predOffset[l] = predCursor;
        predCursor += rows;         // prediction/error live in z[l] (rows) space
    }

    self.wTotal = wCursor;
    self.rTotal = rCursor;
    self.predTotal = predCursor;
}

- (id<MTLBuffer>)sharedFloats:(uint32_t)count {
    size_t length = (size_t)(count == 0u ? 1u : count) * sizeof(float);
    id<MTLBuffer> buffer = [self.device newBufferWithLength:length options:MTLResourceStorageModeShared];
    memset(buffer.contents, 0, length);
    return buffer;
}

- (void)fillFloats:(id<MTLBuffer>)buffer count:(uint32_t)count value:(float)value {
    float *data = (float *)buffer.contents;
    for (uint32_t i = 0u; i < count; ++i) {
        data[i] = value;
    }
}

- (BOOL)allocateBuffers:(NSString **)error {
    (void)error;
    uint32_t topDim = self.arch[self.archLen - 1u];

    self.bufW = [self sharedFloats:self.wTotal];
    self.bufR = [self sharedFloats:self.rTotal];
    self.bufA = [self sharedFloats:topDim * topDim];
    self.bufV = (self.targetDim > 0u) ? [self sharedFloats:self.targetDim * topDim] : nil;

    self.bufZ = [self sharedFloats:self.zTotal];
    self.bufZSaved = [self sharedFloats:self.zTotal];
    self.bufPrevTop = [self sharedFloats:topDim];

    self.bufPred = [self sharedFloats:self.predTotal];
    self.bufErr = [self sharedFloats:self.predTotal];

    self.bufPrecision = [self sharedFloats:self.predTotal];
    self.bufVariance = [self sharedFloats:self.predTotal];
    [self fillFloats:self.bufPrecision count:self.predTotal value:1.0f];
    [self fillFloats:self.bufVariance count:self.predTotal value:1.0f];

    self.bufTemporalPrec = [self sharedFloats:topDim];
    self.bufTemporalVar = [self sharedFloats:topDim];
    [self fillFloats:self.bufTemporalPrec count:topDim value:1.0f];
    [self fillFloats:self.bufTemporalVar count:topDim value:1.0f];

    if (self.targetDim > 0u) {
        self.bufTaskPrec = [self sharedFloats:self.targetDim];
        self.bufTaskVar = [self sharedFloats:self.targetDim];
        [self fillFloats:self.bufTaskPrec count:self.targetDim value:1.0f];
        [self fillFloats:self.bufTaskVar count:self.targetDim value:1.0f];
    }

    self.bufTmpA = [self sharedFloats:self.maxDim];
    self.bufTmpB = [self sharedFloats:self.maxDim];
    self.bufTmpC = [self sharedFloats:self.maxDim];
    self.bufGrad = [self sharedFloats:self.maxDim];
    self.bufBottomUp = [self sharedFloats:self.zTotal];
    self.bufTopDown = [self sharedFloats:self.zTotal];
    self.bufInput = [self sharedFloats:self.arch[0]];
    self.bufTarget = [self sharedFloats:(self.targetDim > 0u ? self.targetDim : 1u)];
    self.bufScalar = [self sharedFloats:1u];

    return YES;
}

- (BOOL)seedW:(const float *)w len:(size_t)wLen
            r:(const float *)r len:(size_t)rLen
            a:(const float *)a len:(size_t)aLen
            v:(const float *)v len:(size_t)vLen
        error:(NSString **)errorOut {
    uint32_t topDim = self.arch[self.archLen - 1u];

    if (w != NULL) {
        if (wLen != self.wTotal) {
            if (errorOut != nil) {
                *errorOut = [NSString stringWithFormat:@"W seed length %zu != expected %u", wLen, self.wTotal];
            }
            return NO;
        }
        memcpy(self.bufW.contents, w, wLen * sizeof(float));
    }

    if (r != NULL) {
        if (rLen != self.rTotal) {
            if (errorOut != nil) {
                *errorOut = [NSString stringWithFormat:@"R seed length %zu != expected %u", rLen, self.rTotal];
            }
            return NO;
        }
        memcpy(self.bufR.contents, r, rLen * sizeof(float));
    }

    if (a != NULL) {
        if (aLen != (size_t)topDim * topDim) {
            if (errorOut != nil) {
                *errorOut = [NSString stringWithFormat:@"A seed length %zu != expected %u", aLen, topDim * topDim];
            }
            return NO;
        }
        memcpy(self.bufA.contents, a, aLen * sizeof(float));
    }

    if (v != NULL && self.bufV != nil) {
        if (vLen != (size_t)self.targetDim * topDim) {
            if (errorOut != nil) {
                *errorOut = [NSString stringWithFormat:@"V seed length %zu != expected %u", vLen, self.targetDim * topDim];
            }
            return NO;
        }
        memcpy(self.bufV.contents, v, vLen * sizeof(float));
    }

    return YES;
}

- (void)readWeightsW:(float *)w r:(float *)r a:(float *)a v:(float *)v {
    uint32_t topDim = self.arch[self.archLen - 1u];

    if (w != NULL) {
        memcpy(w, self.bufW.contents, (size_t)self.wTotal * sizeof(float));
    }
    if (r != NULL) {
        memcpy(r, self.bufR.contents, (size_t)self.rTotal * sizeof(float));
    }
    if (a != NULL) {
        memcpy(a, self.bufA.contents, (size_t)topDim * topDim * sizeof(float));
    }
    if (v != NULL && self.bufV != nil) {
        memcpy(v, self.bufV.contents, (size_t)self.targetDim * topDim * sizeof(float));
    }
}

- (void)readLatent:(float *)out length:(uint32_t)length {
    uint32_t topDim = self.arch[self.archLen - 1u];
    uint32_t topOffset = self.zOffset[self.archLen - 1u];
    uint32_t count = (length < topDim) ? length : topDim;
    const float *z = (const float *)self.bufZ.contents;
    memcpy(out, z + topOffset, (size_t)count * sizeof(float));
}

- (void)dealloc {
    free(self.arch);
    free(self.zOffset);
    free(self.wOffset);
    free(self.rOffset);
    free(self.predOffset);
}

@end
