//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <Metal/Metal.h>
#import <Accelerate/Accelerate.h>

#include "bridge.h"

#include <algorithm>
#include <cfloat>
#include <cmath>
#include <complex>
#include <cstring>
#include <vector>

static const NSUInteger FluidThreads = 256u;
static const uint32_t FluidAnchorSlots = 8u;
static const uint32_t FluidMaximumWaveModes = 128u;
static const uint32_t FluidDebugCapacity = 2048u;

typedef struct SortScatterParams {
    uint32_t num_particles;
    uint32_t num_cells;
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    float grid_spacing;
    float inv_grid_spacing;
} SortScatterParams;

typedef struct PlanckExchangeParams {
    uint32_t num_particles;
    float dt;
    float conductivity;
    float radius;
} PlanckExchangeParams;

typedef struct MergeParams {
    uint32_t num_particles;
    uint32_t padded;
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    float inv_spacing;
} MergeParams;

typedef struct PicGatherParams {
    uint32_t num_particles;
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    float grid_spacing;
    float inv_grid_spacing;
    float dt;
    float domain_x;
    float domain_y;
    float domain_z;
    float gamma;
    float gas_constant;
    float specific_heat;
    float rho_min;
    float pressure_min;
    float gravity_enabled;
} PicGatherParams;

typedef struct ModeProjectParams {
    uint32_t num_modes;
    uint32_t num_particles;
    uint32_t anchors_per_mode;
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    float grid_spacing;
    float inv_grid_spacing;
} ModeProjectParams;

typedef struct PilotWaveParams {
    uint32_t num_particles;
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    float grid_spacing;
    float inv_grid_spacing;
    float dt;
    float domain_x;
    float domain_y;
    float domain_z;
    float hbar_eff;
    float eps_denom;
    float mass_min;
} PilotWaveParams;

typedef struct GasGridParams {
    uint32_t num_cells;
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    float dx;
    float dt;
    float gamma;
    float specific_heat;
    float rho_min;
    float pressure_min;
    float viscosity;
    float thermal_conductivity;
} GasGridParams;

typedef struct SpectralModeParams {
    uint32_t num_osc;
    uint32_t max_carriers;
    uint32_t num_carriers;
    float dt;
    float coupling_scale;
    float carrier_reg;
    uint32_t rng_seed;
    float conflict_threshold;
    float offender_weight_floor;
    float gate_width_min;
    float gate_width_max;
    float ema_alpha;
    float recenter_alpha;
    uint32_t mode;
    float anchor_random_eps;
    float stable_amp_threshold;
    float crystallize_amp_threshold;
    float crystallize_conflict_threshold;
    uint32_t crystallize_age;
    float crystallized_coupling_boost;
    float volatile_decay_mul;
    float stable_decay_mul;
    float crystallized_decay_mul;
    float topdown_phase_scale;
    float topdown_energy_scale;
    float topdown_random_energy_eps;
    float repulsion_scale;
    float domain_x;
    float domain_y;
    float domain_z;
    float spatial_sigma;
    float metabolic_rate;
} SpectralModeParams;

typedef struct GPEParams {
    float dt;
    float hbar_eff;
    float mass_eff;
    float interaction;
    float energy_decay;
    float chemical_potential;
    float inv_domega2;
    uint32_t anchors;
    uint32_t rng_seed;
    float anchor_eps;
} GPEParams;

typedef struct CoherenceBinParams {
    float omega_min;
    float inv_bin_width;
} CoherenceBinParams;

static void fluid_write_error(char *output, int capacity, NSString *message) {
    if (output == nullptr || capacity <= 0) {
        return;
    }

    const char *text = message.UTF8String;

    if (text == nullptr) {
        output[0] = '\0';
        return;
    }

    std::strncpy(output, text, (size_t)capacity - 1u);
    output[capacity - 1] = '\0';
}

static uint32_t fluid_mode_count(const FluidConfig &config) {
    uint32_t largest = std::max(config.grid_x, std::max(config.grid_y, config.grid_z));
    uint32_t modes = 1u;

    while (modes < largest) {
        modes <<= 1u;
    }

    return modes;
}

static float fluid_periodic(float value, float period) {
    float wrapped = std::fmod(value, period);
    return wrapped < 0.0f ? wrapped + period : wrapped;
}

static float fluid_debug_float(uint32_t word) {
    float value;
    std::memcpy(&value, &word, sizeof(value));
    return value;
}

@interface SensoriumFluidDomain : NSObject {
@public
    FluidConfig _config;
    uint32_t _cellCount;
    uint32_t _particleCount;
    uint32_t _particleCapacity;
    uint32_t _mergePadCapacity;
    uint32_t _modeCount;
    uint32_t _randomSeed;
    BOOL _waveInitialized;
    float _omegaMinimum;
    float _omegaSpacing;
    float _gateMinimum;
    float _gateMaximum;
    float _spatialSigma;
    float _meanTemperature;
    std::vector<float> _previousAmplitude;
    std::vector<float> _previousPsiReal;
    std::vector<float> _previousPsiImaginary;

    id<MTLDevice> _device;
    id<MTLCommandQueue> _queue;
    id<MTLLibrary> _library;
    NSMutableDictionary<NSString *, id<MTLComputePipelineState>> *_pipelines;

    id<MTLBuffer> _density;
    id<MTLBuffer> _momentum;
    id<MTLBuffer> _internalEnergy;
    id<MTLBuffer> _stageDensity;
    id<MTLBuffer> _stageMomentum;
    id<MTLBuffer> _stageEnergy;
    id<MTLBuffer> _trialDensity;
    id<MTLBuffer> _trialMomentum;
    id<MTLBuffer> _trialEnergy;
    id<MTLBuffer> _k1Density;
    id<MTLBuffer> _k1Momentum;
    id<MTLBuffer> _k1Energy;
    id<MTLBuffer> _gravityPotential;
    id<MTLBuffer> _spatialPsiReal;
    id<MTLBuffer> _spatialPsiImaginary;

    // Private GPU particle SoT (Sensorium device-resident history).
    id<MTLBuffer> _position;
    id<MTLBuffer> _velocity;
    id<MTLBuffer> _mass;
    id<MTLBuffer> _heat;
    id<MTLBuffer> _energy;
    id<MTLBuffer> _phase;
    id<MTLBuffer> _omega;
    id<MTLBuffer> _amplitude;
    id<MTLBuffer> _content;
    id<MTLBuffer> _positionOutput;
    id<MTLBuffer> _velocityOutput;
    id<MTLBuffer> _heatOutput;

    // Shared staging for host pack/unpack and CPU thermo helpers. History grow
    // and append upload go Private via blit — never CPU-memcpy of resident SoT.
    id<MTLBuffer> _hostPosition;
    id<MTLBuffer> _hostVelocity;
    id<MTLBuffer> _hostMass;
    id<MTLBuffer> _hostHeat;
    id<MTLBuffer> _hostEnergy;
    id<MTLBuffer> _hostPhase;
    id<MTLBuffer> _hostOmega;
    id<MTLBuffer> _hostAmplitude;
    id<MTLBuffer> _hostContent;

    id<MTLBuffer> _mergeKeys;
    id<MTLBuffer> _mergeIndices;
    id<MTLBuffer> _mergePhase;
    id<MTLBuffer> _mergeOmega;
    id<MTLBuffer> _mergeAmplitude;
    id<MTLBuffer> _mergeContent;
    id<MTLBuffer> _mergeCount;
    id<MTLBuffer> _clamped;
    id<MTLBuffer> _hostClamped;
    id<MTLBuffer> _cellMorton;
    id<MTLBuffer> _spatialTokenIDs;
    std::vector<float> _fftInvK2;
    std::vector<std::complex<float>> _fftScratch;
    FFTSetup _fftSetup;
    uint32_t _fftLogMax;

    id<MTLBuffer> _cellIndex;
    id<MTLBuffer> _cellCounts;
    id<MTLBuffer> _cellStarts;
    id<MTLBuffer> _cellOffsets;
    id<MTLBuffer> _sortedOriginalIndex;
    id<MTLBuffer> _sortedPosition;
    id<MTLBuffer> _sortedVelocity;
    id<MTLBuffer> _sortedMass;
    id<MTLBuffer> _sortedHeat;
    id<MTLBuffer> _sortedEnergy;

    id<MTLBuffer> _psiReal;
    id<MTLBuffer> _psiImaginary;
    id<MTLBuffer> _modeOmega;
    id<MTLBuffer> _modeLinewidth;
    id<MTLBuffer> _anchorIndex;
    id<MTLBuffer> _anchorWeight;
    id<MTLBuffer> _modeAccumulators;
    id<MTLBuffer> _modeCountBuffer;
    id<MTLBuffer> _binStarts;
    id<MTLBuffer> _binnedIndex;
    id<MTLBuffer> _binParams;

    id<MTLBuffer> _debugHead;
    id<MTLBuffer> _debugWords;

    // GPU display: XZ projection scratch, extents, and Shared RGBA8 output.
    id<MTLBuffer> _displayRho;
    id<MTLBuffer> _displayPsi;
    id<MTLBuffer> _displayGuidanceX;
    id<MTLBuffer> _displayGuidanceZ;
    id<MTLBuffer> _displayExtents;
    id<MTLBuffer> _displayGlow;
    id<MTLBuffer> _displayCore;
    id<MTLBuffer> _displayRGBA;
    uint32_t _displayWidth;
    uint32_t _displayHeight;
    size_t _displayPixels;
    size_t _displayBytes;
}

- (instancetype)initWithConfig:(const FluidConfig *)config
                metallibBytes:(const void *)metallibBytes
                        length:(size_t)length
                           error:(NSString **)error;
- (BOOL)stepParticles:(FluidParticle *)particles
                 count:(uint32_t)count
           diagnostics:(FluidDiagnostics *)diagnostics
                 error:(NSString **)error;
- (BOOL)appendParticles:(const FluidParticle *)particles
             contentIDs:(const uint32_t *)contentIDs
                  count:(uint32_t)count
                  start:(uint32_t *)start
                  error:(NSString **)error;
- (BOOL)advanceResident:(FluidDiagnostics *)diagnostics error:(NSString **)error;
- (BOOL)solveGravity:(NSString **)error;
- (BOOL)applyClamp:(NSString **)error;
- (BOOL)computeSpatialIDs:(NSString **)error;
- (BOOL)readSpatialIDs:(uint32_t *)ids
                 start:(uint32_t)start
                 count:(uint32_t)count
                 error:(NSString **)error;
- (BOOL)applyCouplingWeights:(NSString **)error;
- (void)applySeparationSoliton:(float)delta;
- (BOOL)mergeInelastic:(NSString **)error;
- (BOOL)readParticles:(FluidParticle *)particles
                start:(uint32_t)start
                count:(uint32_t)count
                error:(NSString **)error;
- (BOOL)retainParticles:(const uint32_t *)indices
                  count:(uint32_t)count
                  error:(NSString **)error;
- (BOOL)readWave:(FluidWaveMode *)modes count:(uint32_t)count error:(NSString **)error;
- (BOOL)read:(FluidReading *)reading error:(NSString **)error;
- (BOOL)readProjection:(float *)density
             coherence:(float *)coherence
              guidanceX:(float *)guidanceX
              guidanceZ:(float *)guidanceZ
                  count:(uint32_t)count
                  error:(NSString **)error;
- (BOOL)readDisplay:(uint8_t *)rgba
              count:(uint32_t)byteCount
              stats:(FluidDisplayStats *)stats
              error:(NSString **)error;
@end

struct DisplayParams {
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    uint32_t display_width;
    uint32_t display_height;
    float spacing;
    float inv_spacing;
    uint32_t num_particles;
};

@implementation SensoriumFluidDomain

- (instancetype)initWithConfig:(const FluidConfig *)config
                metallibBytes:(const void *)metallibBytes
                        length:(size_t)length
                           error:(NSString **)error {
    self = [super init];

    if (self == nil) {
        return nil;
    }

    _config = *config;
    _cellCount = config->grid_x * config->grid_y * config->grid_z;
    _particleCount = 0u;
    _particleCapacity = 0u;
    _mergePadCapacity = 0u;
    uint64_t displayCells = (uint64_t)config->grid_x * (uint64_t)config->grid_y * (uint64_t)config->grid_z;
    double displayAspect = (double)config->grid_x / (double)config->grid_z;
    _displayWidth = std::max((uint32_t)std::llround(std::sqrt(displayCells * displayAspect)), 1u);
    _displayHeight = (uint32_t)((displayCells + _displayWidth - 1u) / _displayWidth);
    _displayPixels = (size_t)_displayWidth * (size_t)_displayHeight;
    _displayBytes = _displayPixels * 4u;
    _modeCount = fluid_mode_count(*config);
    _randomSeed = 1u;
    _fftSetup = nullptr;
    _fftLogMax = 0u;
    _meanTemperature = 0.0f;
    _device = MTLCreateSystemDefaultDevice();

    if (!std::isfinite(config->gravity_g) || config->gravity_g <= 0.0f) {
        if (error != nil) {
            *error = @"gravity_g must be finite and positive (ω-natural CODATA G)";
        }

        return nil;
    }

    if (_device == nil) {
        if (error != nil) {
            *error = @"Metal device is unavailable";
        }

        return nil;
    }

    _queue = [_device newCommandQueue];
    _pipelines = [NSMutableDictionary dictionary];
    dispatch_data_t metallib = dispatch_data_create(
        metallibBytes,
        length,
        nil,
        DISPATCH_DATA_DESTRUCTOR_DEFAULT
    );
    NSError *compileError = nil;
    _library = [_device newLibraryWithData:metallib error:&compileError];

    if (_library == nil) {
        if (error != nil) {
            *error = compileError.localizedDescription ?: @"Metal library loading failed";
        }

        return nil;
    }

    size_t scalarBytes = (size_t)_cellCount * sizeof(float);
    _density = [self buffer:scalarBytes];
    _momentum = [self buffer:scalarBytes * 3u];
    _internalEnergy = [self buffer:scalarBytes];
    _stageDensity = [self buffer:scalarBytes];
    _stageMomentum = [self buffer:scalarBytes * 3u];
    _stageEnergy = [self buffer:scalarBytes];
    _trialDensity = [self buffer:scalarBytes];
    _trialMomentum = [self buffer:scalarBytes * 3u];
    _trialEnergy = [self buffer:scalarBytes];
    _k1Density = [self buffer:scalarBytes];
    _k1Momentum = [self buffer:scalarBytes * 3u];
    _k1Energy = [self buffer:scalarBytes];
    _gravityPotential = [self buffer:scalarBytes];
    _spatialPsiReal = [self buffer:scalarBytes];
    _spatialPsiImaginary = [self buffer:scalarBytes];
    _cellCounts = [self buffer:(size_t)_cellCount * sizeof(uint32_t)];
    _cellStarts = [self buffer:(size_t)_cellCount * sizeof(uint32_t)];
    _cellOffsets = [self buffer:(size_t)_cellCount * sizeof(uint32_t)];

    size_t modeBytes = (size_t)_modeCount * sizeof(float);
    _psiReal = [self buffer:modeBytes];
    _psiImaginary = [self buffer:modeBytes];
    _modeOmega = [self buffer:modeBytes];
    _modeLinewidth = [self buffer:modeBytes];
    _anchorIndex = [self buffer:(size_t)_modeCount * FluidAnchorSlots * sizeof(uint32_t)];
    _anchorWeight = [self buffer:(size_t)_modeCount * FluidAnchorSlots * sizeof(float)];
    _modeAccumulators = [self buffer:(size_t)_modeCount * 8u * sizeof(uint32_t)];
    _modeCountBuffer = [self buffer:sizeof(uint32_t)];
    _binStarts = [self buffer:(size_t)(_modeCount + 1u) * sizeof(uint32_t)];
    _binnedIndex = [self buffer:(size_t)_modeCount * sizeof(uint32_t)];
    _binParams = [self buffer:sizeof(CoherenceBinParams)];
    _debugHead = [self buffer:sizeof(uint32_t)];
    _debugWords = [self buffer:(size_t)FluidDebugCapacity * 6u * sizeof(uint32_t)];
    *(uint32_t *)_modeCountBuffer.contents = _modeCount;

    size_t projectionBytes = (size_t)_config.grid_x * (size_t)_config.grid_z * sizeof(float);
    _displayRho = [self buffer:projectionBytes];
    _displayPsi = [self buffer:projectionBytes];
    _displayGuidanceX = [self buffer:projectionBytes];
    _displayGuidanceZ = [self buffer:projectionBytes];
    _displayExtents = [self buffer:12u * sizeof(uint32_t)];
    _displayGlow = [self buffer:_displayPixels * sizeof(uint32_t)];
    _displayCore = [self buffer:_displayPixels * sizeof(uint32_t)];
    _displayRGBA = [self buffer:_displayBytes];

    uint32_t *binStarts = (uint32_t *)_binStarts.contents;
    uint32_t *binned = (uint32_t *)_binnedIndex.contents;

    for (uint32_t index = 0; index < _modeCount; index++) {
        binStarts[index] = index;
        binned[index] = index;
    }

    binStarts[_modeCount] = _modeCount;

    auto isPow2 = [](uint32_t value) {
        return value > 0u && (value & (value - 1u)) == 0u;
    };

    if (!isPow2(_config.grid_x) || !isPow2(_config.grid_y) || !isPow2(_config.grid_z)) {
        if (error != nil) {
            *error = @"gravity FFT requires power-of-two grid dimensions";
        }

        return nil;
    }

    uint32_t logX = 0u;
    uint32_t logY = 0u;
    uint32_t logZ = 0u;

    for (uint32_t value = _config.grid_x; value > 1u; value >>= 1u) {
        logX++;
    }

    for (uint32_t value = _config.grid_y; value > 1u; value >>= 1u) {
        logY++;
    }

    for (uint32_t value = _config.grid_z; value > 1u; value >>= 1u) {
        logZ++;
    }

    _fftLogMax = std::max(logX, std::max(logY, logZ));
    _fftSetup = vDSP_create_fftsetup(_fftLogMax, kFFTRadix2);

    if (_fftSetup == nullptr) {
        if (error != nil) {
            *error = @"failed to create Accelerate FFT setup";
        }

        return nil;
    }

    _fftInvK2.assign((size_t)_cellCount, 0.0f);
    _fftScratch.assign((size_t)_cellCount, std::complex<float>{0.0f, 0.0f});
    const float twoPi = 2.0f * (float)M_PI;
    const float spacing = _config.spacing;

    auto fftfreq = [](uint32_t index, uint32_t count, float step) -> float {
        int32_t signedIndex = 0;

        if ((count & 1u) == 0u) {
            if (index < count / 2u) {
                signedIndex = (int32_t)index;
            } else if (index == count / 2u) {
                signedIndex = -(int32_t)(count / 2u);
            } else {
                signedIndex = (int32_t)index - (int32_t)count;
            }
        } else if (index <= count / 2u) {
            signedIndex = (int32_t)index;
        } else {
            signedIndex = (int32_t)index - (int32_t)count;
        }

        return (float)signedIndex / ((float)count * step);
    };

    for (uint32_t iz = 0; iz < _config.grid_z; iz++) {
        for (uint32_t iy = 0; iy < _config.grid_y; iy++) {
            for (uint32_t ix = 0; ix < _config.grid_x; ix++) {
                float kx = twoPi * fftfreq(ix, _config.grid_x, spacing);
                float ky = twoPi * fftfreq(iy, _config.grid_y, spacing);
                float kz = twoPi * fftfreq(iz, _config.grid_z, spacing);
                float k2 = kx * kx + ky * ky + kz * kz;
                size_t cell = (size_t)ix +
                    (size_t)iy * _config.grid_x +
                    (size_t)iz * _config.grid_x * _config.grid_y;
                _fftInvK2[cell] = k2 > 0.0f ? 1.0f / k2 : 0.0f;
            }
        }
    }

    return self;
}

- (void)dealloc {
    if (_fftSetup != nullptr) {
        vDSP_destroy_fftsetup(_fftSetup);
        _fftSetup = nullptr;
    }
}

- (id<MTLBuffer>)buffer:(size_t)length {
    return [_device newBufferWithLength:length options:MTLResourceStorageModeShared];
}

- (id<MTLBuffer>)privateBuffer:(size_t)length {
    return [_device newBufferWithLength:length options:MTLResourceStorageModePrivate];
}

- (BOOL)blitFrom:(id<MTLBuffer>)source
    sourceOffset:(NSUInteger)sourceOffset
              to:(id<MTLBuffer>)destination
 destinationOffset:(NSUInteger)destinationOffset
           bytes:(NSUInteger)bytes
           error:(NSString **)error {
    if (bytes == 0u) {
        return YES;
    }

    if (source == nil || destination == nil) {
        if (error != nil) {
            *error = @"blit requires source and destination buffers";
        }

        return NO;
    }

    id<MTLCommandBuffer> command = [_queue commandBuffer];
    id<MTLBlitCommandEncoder> encoder = [command blitCommandEncoder];
    [encoder copyFromBuffer:source
               sourceOffset:sourceOffset
                   toBuffer:destination
          destinationOffset:destinationOffset
                       size:bytes];
    [encoder endEncoding];
    [command commit];
    [command waitUntilCompleted];

    if (command.status == MTLCommandBufferStatusError) {
        if (error != nil) {
            *error = command.error.localizedDescription ?: @"Metal blit failed";
        }

        return NO;
    }

    return YES;
}

- (BOOL)pullParticles:(NSString **)error {
    if (_particleCount == 0u) {
        return YES;
    }

    size_t liveScalar = (size_t)_particleCount * sizeof(float);
    size_t liveVector = liveScalar * 3u;
    size_t liveIndex = (size_t)_particleCount * sizeof(uint32_t);

    return [self blitFrom:_position sourceOffset:0 to:_hostPosition destinationOffset:0 bytes:liveVector error:error] &&
        [self blitFrom:_velocity sourceOffset:0 to:_hostVelocity destinationOffset:0 bytes:liveVector error:error] &&
        [self blitFrom:_mass sourceOffset:0 to:_hostMass destinationOffset:0 bytes:liveScalar error:error] &&
        [self blitFrom:_heat sourceOffset:0 to:_hostHeat destinationOffset:0 bytes:liveScalar error:error] &&
        [self blitFrom:_energy sourceOffset:0 to:_hostEnergy destinationOffset:0 bytes:liveScalar error:error] &&
        [self blitFrom:_phase sourceOffset:0 to:_hostPhase destinationOffset:0 bytes:liveScalar error:error] &&
        [self blitFrom:_omega sourceOffset:0 to:_hostOmega destinationOffset:0 bytes:liveScalar error:error] &&
        [self blitFrom:_amplitude sourceOffset:0 to:_hostAmplitude destinationOffset:0 bytes:liveScalar error:error] &&
        [self blitFrom:_content sourceOffset:0 to:_hostContent destinationOffset:0 bytes:liveIndex error:error];
}

- (BOOL)pushParticleRange:(uint32_t)offset count:(uint32_t)count error:(NSString **)error {
    if (count == 0u) {
        return YES;
    }

    size_t scalarBytes = (size_t)count * sizeof(float);
    size_t vectorBytes = scalarBytes * 3u;
    size_t indexBytes = (size_t)count * sizeof(uint32_t);
    NSUInteger scalarOffset = (NSUInteger)offset * sizeof(float);
    NSUInteger vectorOffset = scalarOffset * 3u;
    NSUInteger indexOffset = (NSUInteger)offset * sizeof(uint32_t);

    return [self blitFrom:_hostPosition sourceOffset:vectorOffset to:_position destinationOffset:vectorOffset bytes:vectorBytes error:error] &&
        [self blitFrom:_hostVelocity sourceOffset:vectorOffset to:_velocity destinationOffset:vectorOffset bytes:vectorBytes error:error] &&
        [self blitFrom:_hostMass sourceOffset:scalarOffset to:_mass destinationOffset:scalarOffset bytes:scalarBytes error:error] &&
        [self blitFrom:_hostHeat sourceOffset:scalarOffset to:_heat destinationOffset:scalarOffset bytes:scalarBytes error:error] &&
        [self blitFrom:_hostEnergy sourceOffset:scalarOffset to:_energy destinationOffset:scalarOffset bytes:scalarBytes error:error] &&
        [self blitFrom:_hostPhase sourceOffset:scalarOffset to:_phase destinationOffset:scalarOffset bytes:scalarBytes error:error] &&
        [self blitFrom:_hostOmega sourceOffset:scalarOffset to:_omega destinationOffset:scalarOffset bytes:scalarBytes error:error] &&
        [self blitFrom:_hostAmplitude sourceOffset:scalarOffset to:_amplitude destinationOffset:scalarOffset bytes:scalarBytes error:error] &&
        [self blitFrom:_hostContent sourceOffset:indexOffset to:_content destinationOffset:indexOffset bytes:indexBytes error:error];
}

- (id<MTLComputePipelineState>)pipeline:(NSString *)name error:(NSString **)error {
    id<MTLComputePipelineState> pipeline = _pipelines[name];

    if (pipeline != nil) {
        return pipeline;
    }

    id<MTLFunction> function = [_library newFunctionWithName:name];

    if (function == nil) {
        if (error != nil) {
            *error = [NSString stringWithFormat:@"Metal function %@ is missing", name];
        }

        return nil;
    }

    NSError *pipelineError = nil;
    pipeline = [_device newComputePipelineStateWithFunction:function error:&pipelineError];

    if (pipeline == nil) {
        if (error != nil) {
            *error = pipelineError.localizedDescription ?: @"Metal pipeline creation failed";
        }

        return nil;
    }

    _pipelines[name] = pipeline;
    return pipeline;
}

- (id<MTLComputeCommandEncoder>)encoder:(id<MTLComputePipelineState>)pipeline
                                  command:(id<MTLCommandBuffer> *)command {
    *command = [_queue commandBuffer];
    id<MTLComputeCommandEncoder> encoder = [*command computeCommandEncoder];
    [encoder setComputePipelineState:pipeline];
    return encoder;
}

- (BOOL)finish:(id<MTLComputeCommandEncoder>)encoder
        command:(id<MTLCommandBuffer>)command
          error:(NSString **)error {
    [encoder endEncoding];
    [command commit];
    [command waitUntilCompleted];

    if (command.status != MTLCommandBufferStatusError) {
        return YES;
    }

    if (error != nil) {
        *error = command.error.localizedDescription ?: @"Metal command failed";
    }

    return NO;
}

- (void)dispatch:(id<MTLComputeCommandEncoder>)encoder
          count:(NSUInteger)count
       pipeline:(id<MTLComputePipelineState>)pipeline {
    NSUInteger width = std::min(FluidThreads, pipeline.maxTotalThreadsPerThreadgroup);
    NSUInteger groups = (count + width - 1u) / width;
    [encoder dispatchThreadgroups:MTLSizeMake(groups, 1u, 1u)
            threadsPerThreadgroup:MTLSizeMake(width, 1u, 1u)];
}

- (BOOL)ensureCapacity:(uint32_t)capacity error:(NSString **)error {
    if (capacity <= _particleCapacity) {
        return YES;
    }

    // Grow Private resident storage by GPU blit of evolved history — Sensorium
    // device-side concat, not host re-upload of the full tape.
    uint32_t grown = _particleCapacity == 0u ? capacity : _particleCapacity;

    while (grown < capacity) {
        grown = grown < 1024u ? 1024u : grown * 2u;
    }

    size_t scalarBytes = (size_t)grown * sizeof(float);
    size_t vectorBytes = scalarBytes * 3u;
    size_t indexBytes = (size_t)grown * sizeof(uint32_t);
    id<MTLBuffer> position = [self privateBuffer:vectorBytes];
    id<MTLBuffer> velocity = [self privateBuffer:vectorBytes];
    id<MTLBuffer> mass = [self privateBuffer:scalarBytes];
    id<MTLBuffer> heat = [self privateBuffer:scalarBytes];
    id<MTLBuffer> energy = [self privateBuffer:scalarBytes];
    id<MTLBuffer> phase = [self privateBuffer:scalarBytes];
    id<MTLBuffer> omega = [self privateBuffer:scalarBytes];
    id<MTLBuffer> amplitude = [self privateBuffer:scalarBytes];
    id<MTLBuffer> content = [self privateBuffer:indexBytes];
    id<MTLBuffer> positionOutput = [self privateBuffer:vectorBytes];
    id<MTLBuffer> velocityOutput = [self privateBuffer:vectorBytes];
    id<MTLBuffer> heatOutput = [self privateBuffer:scalarBytes];
    id<MTLBuffer> cellIndex = [self privateBuffer:indexBytes];
    id<MTLBuffer> sortedOriginalIndex = [self privateBuffer:indexBytes];
    id<MTLBuffer> sortedPosition = [self privateBuffer:vectorBytes];
    id<MTLBuffer> sortedVelocity = [self privateBuffer:vectorBytes];
    id<MTLBuffer> sortedMass = [self privateBuffer:scalarBytes];
    id<MTLBuffer> sortedHeat = [self privateBuffer:scalarBytes];
    id<MTLBuffer> sortedEnergy = [self privateBuffer:scalarBytes];
    id<MTLBuffer> hostPosition = [self buffer:vectorBytes];
    id<MTLBuffer> hostVelocity = [self buffer:vectorBytes];
    id<MTLBuffer> hostMass = [self buffer:scalarBytes];
    id<MTLBuffer> hostHeat = [self buffer:scalarBytes];
    id<MTLBuffer> hostEnergy = [self buffer:scalarBytes];
    id<MTLBuffer> hostPhase = [self buffer:scalarBytes];
    id<MTLBuffer> hostOmega = [self buffer:scalarBytes];
    id<MTLBuffer> hostAmplitude = [self buffer:scalarBytes];
    id<MTLBuffer> hostContent = [self buffer:indexBytes];
    uint32_t padCapacity = 1u;

    while (padCapacity < grown) {
        padCapacity *= 2u;
    }

    size_t keyBytes = (size_t)padCapacity * sizeof(uint64_t);
    size_t padIndexBytes = (size_t)padCapacity * sizeof(uint32_t);
    id<MTLBuffer> mergeKeys = [self privateBuffer:keyBytes];
    id<MTLBuffer> mergeIndices = [self privateBuffer:padIndexBytes];
    id<MTLBuffer> mergePhase = [self privateBuffer:scalarBytes];
    id<MTLBuffer> mergeOmega = [self privateBuffer:scalarBytes];
    id<MTLBuffer> mergeAmplitude = [self privateBuffer:scalarBytes];
    id<MTLBuffer> mergeContent = [self privateBuffer:indexBytes];
    id<MTLBuffer> mergeCount = [self buffer:sizeof(uint32_t)];
    id<MTLBuffer> clamped = [self privateBuffer:indexBytes];
    id<MTLBuffer> hostClamped = [self buffer:indexBytes];
    id<MTLBuffer> cellMorton = [self privateBuffer:indexBytes];
    id<MTLBuffer> spatialTokenIDs = [self privateBuffer:indexBytes];

    if (position == nil || velocity == nil || mass == nil || heat == nil ||
        energy == nil || phase == nil || omega == nil || amplitude == nil ||
        content == nil || positionOutput == nil || velocityOutput == nil ||
        heatOutput == nil || cellIndex == nil || sortedOriginalIndex == nil ||
        sortedPosition == nil || sortedVelocity == nil || sortedMass == nil ||
        sortedHeat == nil || sortedEnergy == nil || hostPosition == nil ||
        hostVelocity == nil || hostMass == nil || hostHeat == nil ||
        hostEnergy == nil || hostPhase == nil || hostOmega == nil ||
        hostAmplitude == nil || hostContent == nil || mergeKeys == nil ||
        mergeIndices == nil || mergePhase == nil || mergeOmega == nil ||
        mergeAmplitude == nil || mergeContent == nil || mergeCount == nil ||
        clamped == nil || hostClamped == nil || cellMorton == nil ||
        spatialTokenIDs == nil) {
        if (error != nil) {
            *error = @"failed to grow resident particle buffers";
        }

        return NO;
    }

    if (_particleCount > 0u && _position != nil) {
        size_t liveScalar = (size_t)_particleCount * sizeof(float);
        size_t liveVector = liveScalar * 3u;
        size_t liveIndex = (size_t)_particleCount * sizeof(uint32_t);

        if (![self blitFrom:_position sourceOffset:0 to:position destinationOffset:0 bytes:liveVector error:error] ||
            ![self blitFrom:_velocity sourceOffset:0 to:velocity destinationOffset:0 bytes:liveVector error:error] ||
            ![self blitFrom:_mass sourceOffset:0 to:mass destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:_heat sourceOffset:0 to:heat destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:_energy sourceOffset:0 to:energy destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:_phase sourceOffset:0 to:phase destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:_omega sourceOffset:0 to:omega destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:_amplitude sourceOffset:0 to:amplitude destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:_content sourceOffset:0 to:content destinationOffset:0 bytes:liveIndex error:error] ||
            ![self blitFrom:_clamped sourceOffset:0 to:clamped destinationOffset:0 bytes:liveIndex error:error]) {
            return NO;
        }

        // Keep host staging coherent with the grown Private SoT.
        if (![self blitFrom:position sourceOffset:0 to:hostPosition destinationOffset:0 bytes:liveVector error:error] ||
            ![self blitFrom:velocity sourceOffset:0 to:hostVelocity destinationOffset:0 bytes:liveVector error:error] ||
            ![self blitFrom:mass sourceOffset:0 to:hostMass destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:heat sourceOffset:0 to:hostHeat destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:energy sourceOffset:0 to:hostEnergy destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:phase sourceOffset:0 to:hostPhase destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:omega sourceOffset:0 to:hostOmega destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:amplitude sourceOffset:0 to:hostAmplitude destinationOffset:0 bytes:liveScalar error:error] ||
            ![self blitFrom:content sourceOffset:0 to:hostContent destinationOffset:0 bytes:liveIndex error:error] ||
            ![self blitFrom:clamped sourceOffset:0 to:hostClamped destinationOffset:0 bytes:liveIndex error:error]) {
            return NO;
        }
    }

    _position = position;
    _velocity = velocity;
    _mass = mass;
    _heat = heat;
    _energy = energy;
    _phase = phase;
    _omega = omega;
    _amplitude = amplitude;
    _content = content;
    _positionOutput = positionOutput;
    _velocityOutput = velocityOutput;
    _heatOutput = heatOutput;
    _cellIndex = cellIndex;
    _sortedOriginalIndex = sortedOriginalIndex;
    _sortedPosition = sortedPosition;
    _sortedVelocity = sortedVelocity;
    _sortedMass = sortedMass;
    _sortedHeat = sortedHeat;
    _sortedEnergy = sortedEnergy;
    _hostPosition = hostPosition;
    _hostVelocity = hostVelocity;
    _hostMass = hostMass;
    _hostHeat = hostHeat;
    _hostEnergy = hostEnergy;
    _hostPhase = hostPhase;
    _hostOmega = hostOmega;
    _hostAmplitude = hostAmplitude;
    _hostContent = hostContent;
    _mergeKeys = mergeKeys;
    _mergeIndices = mergeIndices;
    _mergePhase = mergePhase;
    _mergeOmega = mergeOmega;
    _mergeAmplitude = mergeAmplitude;
    _mergeContent = mergeContent;
    _mergeCount = mergeCount;
    _clamped = clamped;
    _hostClamped = hostClamped;
    _cellMorton = cellMorton;
    _spatialTokenIDs = spatialTokenIDs;
    _particleCapacity = grown;
    _mergePadCapacity = padCapacity;
    return YES;
}

- (BOOL)writeParticles:(const FluidParticle *)particles
            contentIDs:(const uint32_t *)contentIDs
                 count:(uint32_t)count
                offset:(uint32_t)offset
                 error:(NSString **)error {
    float *position = (float *)_hostPosition.contents;
    float *velocity = (float *)_hostVelocity.contents;
    float *mass = (float *)_hostMass.contents;
    float *heat = (float *)_hostHeat.contents;
    float *energy = (float *)_hostEnergy.contents;
    float *phase = (float *)_hostPhase.contents;
    float *omega = (float *)_hostOmega.contents;
    float *amplitude = (float *)_hostAmplitude.contents;
    uint32_t *content = (uint32_t *)_hostContent.contents;
    uint32_t *clamped = (uint32_t *)_hostClamped.contents;
    float domainX = _config.grid_x * _config.spacing;
    float domainY = _config.grid_y * _config.spacing;
    float domainZ = _config.grid_z * _config.spacing;

    for (uint32_t index = 0; index < count; index++) {
        uint32_t slot = offset + index;
        uint32_t base = slot * 3u;
        position[base] = fluid_periodic(particles[index].position_x, domainX);
        position[base + 1u] = fluid_periodic(particles[index].position_y, domainY);
        position[base + 2u] = fluid_periodic(particles[index].position_z, domainZ);
        velocity[base] = particles[index].velocity_x;
        velocity[base + 1u] = particles[index].velocity_y;
        velocity[base + 2u] = particles[index].velocity_z;
        mass[slot] = particles[index].mass;
        heat[slot] = particles[index].heat;
        energy[slot] = particles[index].energy;
        phase[slot] = particles[index].phase;
        omega[slot] = particles[index].omega;
        amplitude[slot] = std::sqrt(std::max(particles[index].energy, 1.0e-8f));
        // Keep the full identity word; merge masks to 16 bits like Python's key.
        content[slot] = contentIDs != nullptr ? contentIDs[index] : 0u;
        // Market inject is unclamped; probe/crystallization sets the mask later.
        clamped[slot] = 0u;
    }

    size_t indexBytes = (size_t)count * sizeof(uint32_t);
    NSUInteger indexOffset = (NSUInteger)offset * sizeof(uint32_t);

    return [self pushParticleRange:offset count:count error:error] &&
        [self blitFrom:_hostClamped sourceOffset:indexOffset to:_clamped destinationOffset:indexOffset bytes:indexBytes error:error];
}

- (void)initializeWave {
    float omegaMinimum = _config.omega_min;
    float omegaMaximum = _config.omega_max;

    _omegaMinimum = omegaMinimum;
    _omegaSpacing = _modeCount > 1u
        ? (omegaMaximum - omegaMinimum) / (float)(_modeCount - 1u)
        : 0.0f;
    _gateMinimum = _omegaSpacing > 0.0f ? 0.25f * _omegaSpacing : 1.0e-6f;
    _gateMaximum = _omegaSpacing > 0.0f ? 4.0f * _omegaSpacing : 1.0f;
    _spatialSigma = 0.25f * std::min(
        _config.grid_x * _config.spacing,
        std::min(_config.grid_y * _config.spacing, _config.grid_z * _config.spacing)
    );
    float *modeOmega = (float *)_modeOmega.contents;
    float *linewidth = (float *)_modeLinewidth.contents;

    for (uint32_t mode = 0; mode < _modeCount; mode++) {
        modeOmega[mode] = omegaMinimum + (float)mode * _omegaSpacing;
        linewidth[mode] = _omegaSpacing > 0.0f ? _omegaSpacing : 1.0f;
    }

    CoherenceBinParams *bin = (CoherenceBinParams *)_binParams.contents;
    bin->omega_min = omegaMinimum;
    bin->inv_bin_width = _omegaSpacing > 0.0f ? 1.0f / _omegaSpacing : 0.0f;
    // Anchors are seeded each omegawave.step against the live population.
    _waveInitialized = YES;
}

- (void)seedAnchors {
    uint32_t *anchorIndex = (uint32_t *)_anchorIndex.contents;
    float *anchorWeight = (float *)_anchorWeight.contents;
    std::fill(anchorIndex, anchorIndex + (size_t)_modeCount * FluidAnchorSlots, UINT32_MAX);
    std::fill(anchorWeight, anchorWeight + (size_t)_modeCount * FluidAnchorSlots, 0.0f);

    if (!std::isfinite(_omegaSpacing) || _omegaSpacing <= 0.0f || _particleCount == 0u) {
        return;
    }

    NSString *pullError = nil;

    if (![self pullParticles:&pullError]) {
        return;
    }

    float *omega = (float *)_hostOmega.contents;
    float *energy = (float *)_hostEnergy.contents;

    for (uint32_t mode = 0; mode < _modeCount; mode++) {
        std::vector<std::pair<float, uint32_t>> candidates;

        for (uint32_t particle = 0; particle < _particleCount; particle++) {
            float scaled = (omega[particle] - _omegaMinimum) / _omegaSpacing;
            uint32_t nearest = (uint32_t)std::clamp(
                (int)std::lround(scaled), 0, (int)_modeCount - 1
            );

            if (nearest == mode) {
                candidates.emplace_back(std::sqrt(std::max(energy[particle], 1.0e-8f)), particle);
            }
        }

        std::sort(candidates.begin(), candidates.end(), [](const auto &left, const auto &right) {
            return left.first > right.first;
        });
        uint32_t selected = std::min((uint32_t)candidates.size(), FluidAnchorSlots);

        for (uint32_t slot = 0; slot < selected; slot++) {
            size_t destination = (size_t)mode * FluidAnchorSlots + slot;
            anchorIndex[destination] = candidates[slot].second;
            anchorWeight[destination] = candidates[slot].first;
        }
    }
}

- (BOOL)scatter:(NSString **)error {
    std::memset(_density.contents, 0, _density.length);
    std::memset(_momentum.contents, 0, _momentum.length);
    std::memset(_internalEnergy.contents, 0, _internalEnergy.length);
    std::memset(_cellCounts.contents, 0, _cellCounts.length);
    std::memset(_cellOffsets.contents, 0, _cellOffsets.length);
    SortScatterParams params = {
        _particleCount,
        _cellCount,
        _config.grid_x,
        _config.grid_y,
        _config.grid_z,
        _config.spacing,
        1.0f / _config.spacing,
    };
    id<MTLComputePipelineState> indexPipeline = [self pipeline:@"scatter_compute_cell_idx" error:error];
    id<MTLComputePipelineState> countPipeline = [self pipeline:@"scatter_count_cells" error:error];

    if (indexPipeline == nil || countPipeline == nil) {
        return NO;
    }

    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:indexPipeline command:&command];
    [encoder setBuffer:_position offset:0 atIndex:0];
    [encoder setBuffer:_cellIndex offset:0 atIndex:1];
    [encoder setBytes:&params length:sizeof(params) atIndex:2];
    [self dispatch:encoder count:_particleCount pipeline:indexPipeline];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    encoder = [self encoder:countPipeline command:&command];
    [encoder setBuffer:_cellIndex offset:0 atIndex:0];
    [encoder setBuffer:_cellCounts offset:0 atIndex:1];
    [encoder setBytes:&params length:sizeof(params) atIndex:2];
    [self dispatch:encoder count:_particleCount pipeline:countPipeline];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    uint32_t *counts = (uint32_t *)_cellCounts.contents;
    uint32_t *starts = (uint32_t *)_cellStarts.contents;
    uint32_t total = 0u;

    for (uint32_t cell = 0; cell < _cellCount; cell++) {
        starts[cell] = total;
        total += counts[cell];
    }

    if (total != _particleCount) {
        if (error != nil) {
            *error = @"PIC cell histogram did not account for every particle";
        }

        return NO;
    }

    id<MTLComputePipelineState> reorder = [self pipeline:@"scatter_reorder_particles" error:error];
    id<MTLComputePipelineState> scatter = [self pipeline:@"scatter_sorted" error:error];

    if (reorder == nil || scatter == nil) {
        return NO;
    }

    encoder = [self encoder:reorder command:&command];
    NSArray<id<MTLBuffer>> *reorderBuffers = @[
        _position, _velocity, _mass, _heat, _energy, _cellIndex, _cellStarts,
        _cellOffsets, _sortedPosition, _sortedVelocity, _sortedMass, _sortedHeat,
        _sortedEnergy, _sortedOriginalIndex,
    ];

    for (NSUInteger index = 0; index < reorderBuffers.count; index++) {
        [encoder setBuffer:reorderBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&params length:sizeof(params) atIndex:14];
    [self dispatch:encoder count:_particleCount pipeline:reorder];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    encoder = [self encoder:scatter command:&command];
    NSArray<id<MTLBuffer>> *scatterBuffers = @[
        _sortedPosition, _sortedVelocity, _sortedMass, _sortedHeat, _sortedEnergy,
        _density, _momentum, _internalEnergy,
    ];

    for (NSUInteger index = 0; index < scatterBuffers.count; index++) {
        [encoder setBuffer:scatterBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&params length:sizeof(params) atIndex:8];
    [self dispatch:encoder count:_particleCount pipeline:scatter];
    return [self finish:encoder command:command error:error];
}

- (BOOL)deriveDelta:(FluidDiagnostics *)diagnostics
              params:(GasGridParams *)params
               error:(NSString **)error {
    const float gamma = 1.4f;
    const float specificHeat = 1.0f;
    const float viscosity = 1.0e-4f;
    const float prandtl = 0.71f;
    const float conductivity = viscosity * gamma * specificHeat / prandtl;
    const float rhoFloor = 1.0e-3f;
    float *density = (float *)_density.contents;
    float *momentum = (float *)_momentum.contents;
    float *internalEnergy = (float *)_internalEnergy.contents;
    size_t liveScalar = (size_t)_particleCount * sizeof(float);

    if (![self blitFrom:_mass sourceOffset:0 to:_hostMass destinationOffset:0 bytes:liveScalar error:error]) {
        return NO;
    }

    float massTotal = 0.0f;
    float *hostMass = (float *)_hostMass.contents;

    for (uint32_t index = 0; index < _particleCount; index++) {
        massTotal += hostMass[index];
    }

    float domainVolume = (float)_cellCount * _config.spacing * _config.spacing * _config.spacing;
    float rhoMinimum = std::max(rhoFloor, massTotal / domainVolume * FLT_EPSILON);
    float maximumRate = 0.0f;
    float maximumDiffusion = 0.0f;

    for (uint32_t cell = 0; cell < _cellCount; cell++) {
        float rho = density[cell];
        float energy = internalEnergy[cell];
        uint32_t base = cell * 3u;
        float momentumMagnitude = std::sqrt(
            momentum[base] * momentum[base] +
            momentum[base + 1u] * momentum[base + 1u] +
            momentum[base + 2u] * momentum[base + 2u]
        );

        if (!std::isfinite(rho) || !std::isfinite(energy) || !std::isfinite(momentumMagnitude)) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"gas input is non-finite at cell %u", cell];
            }

            return NO;
        }

        float rhoSafe = rho;
        float energyUsed = energy;

        if (std::fabs(rho) <= rhoMinimum) {
            if (energy < -4.0f * rhoMinimum * FLT_EPSILON) {
                if (error != nil) {
                    *error = [NSString stringWithFormat:@"gas low-density energy is inadmissible at cell %u", cell];
                }

                return NO;
            }

            rhoSafe = rhoMinimum;
            energyUsed = std::max(energy, 0.0f);
        }

        if (std::fabs(rho) > rhoMinimum && (!(rho > rhoMinimum) || energy < 0.0f)) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"gas input is inadmissible at cell %u", cell];
            }

            return NO;
        }

        float pressure = (gamma - 1.0f) * energyUsed;
        float sound = std::sqrt(gamma * pressure / rhoSafe);
        float velocityRate = (
            std::fabs(momentum[base] / rhoSafe) +
            std::fabs(momentum[base + 1u] / rhoSafe) +
            std::fabs(momentum[base + 2u] / rhoSafe) + 3.0f * sound
        ) / _config.spacing;
        maximumRate = std::max(maximumRate, velocityRate);
        maximumDiffusion = std::max(
            maximumDiffusion,
            std::max(viscosity / rhoSafe, conductivity / (rhoSafe * specificHeat))
        );
    }

    float deltaAdv = maximumRate > 0.0f ? 0.4f / maximumRate : _config.spacing;
    float deltaDiffuse = maximumDiffusion > 0.0f
        ? 0.15f * _config.spacing * _config.spacing / maximumDiffusion
        : _config.spacing;
    float delta = std::min(_config.max_delta, std::min(deltaAdv, deltaDiffuse));

    if (!(delta > 0.0f) || !std::isfinite(delta)) {
        if (error != nil) {
            *error = @"derived gas timestep is not finite and positive";
        }

        return NO;
    }

    diagnostics->cfl_rate = maximumRate;
    diagnostics->delta_adv = deltaAdv;
    diagnostics->delta_diffuse = deltaDiffuse;
    diagnostics->delta_derived = delta;
    *params = GasGridParams{
        _cellCount,
        _config.grid_x,
        _config.grid_y,
        _config.grid_z,
        _config.spacing,
        delta,
        gamma,
        specificHeat,
        rhoMinimum,
        1.0e-3f,
        viscosity,
        conductivity,
    };
    return YES;
}

- (BOOL)gasAttempt:(GasGridParams)params error:(NSString **)error {
    id<MTLComputePipelineState> stageOne = [self pipeline:@"gas_rk2_stage1" error:error];
    id<MTLComputePipelineState> stageTwo = [self pipeline:@"gas_rk2_stage2" error:error];

    if (stageOne == nil || stageTwo == nil) {
        return NO;
    }

    std::memset(_debugHead.contents, 0, _debugHead.length);
    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:stageOne command:&command];
    NSArray<id<MTLBuffer>> *stageOneBuffers = @[
        _density, _momentum, _internalEnergy,
        _stageDensity, _stageMomentum, _stageEnergy,
        _k1Density, _k1Momentum, _k1Energy,
    ];

    for (NSUInteger index = 0; index < stageOneBuffers.count; index++) {
        [encoder setBuffer:stageOneBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&params length:sizeof(params) atIndex:9];
    [encoder setBuffer:_debugHead offset:0 atIndex:10];
    [encoder setBuffer:_debugWords offset:0 atIndex:11];
    uint32_t debugCapacity = FluidDebugCapacity;
    [encoder setBytes:&debugCapacity length:sizeof(debugCapacity) atIndex:12];
    [self dispatch:encoder count:_cellCount pipeline:stageOne];
    [encoder endEncoding];
    encoder = [command computeCommandEncoder];
    [encoder setComputePipelineState:stageTwo];
    NSArray<id<MTLBuffer>> *stageTwoBuffers = @[
        _density, _momentum, _internalEnergy,
        _stageDensity, _stageMomentum, _stageEnergy,
        _k1Density, _k1Momentum, _k1Energy,
        _trialDensity, _trialMomentum, _trialEnergy,
    ];

    for (NSUInteger index = 0; index < stageTwoBuffers.count; index++) {
        [encoder setBuffer:stageTwoBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&params length:sizeof(params) atIndex:12];
    [encoder setBuffer:_debugHead offset:0 atIndex:13];
    [encoder setBuffer:_debugWords offset:0 atIndex:14];
    [encoder setBytes:&debugCapacity length:sizeof(debugCapacity) atIndex:15];
    [self dispatch:encoder count:_cellCount pipeline:stageTwo];
    return [self finish:encoder command:command error:error];
}

- (BOOL)trialIsFinite {
    float *density = (float *)_trialDensity.contents;
    float *momentum = (float *)_trialMomentum.contents;
    float *energy = (float *)_trialEnergy.contents;

    for (uint32_t cell = 0; cell < _cellCount; cell++) {
        uint32_t base = cell * 3u;

        if (!std::isfinite(density[cell]) || !std::isfinite(energy[cell]) ||
            !std::isfinite(momentum[base]) || !std::isfinite(momentum[base + 1u]) ||
            !std::isfinite(momentum[base + 2u])) {
            return NO;
        }
    }

    return YES;
}

- (BOOL)advanceGas:(FluidDiagnostics *)diagnostics error:(NSString **)error {
    GasGridParams params;

    if (![self deriveDelta:diagnostics params:&params error:error]) {
        return NO;
    }

    uint32_t halvings = 0u;

    while (YES) {
        if (![self gasAttempt:params error:error]) {
            return NO;
        }

        if ([self trialIsFinite]) {
            break;
        }

        float nextDelta = params.dt * 0.5f;

        if (!(nextDelta > 0.0f) || !(nextDelta < params.dt)) {
            if (error != nil) {
                uint32_t events = *(uint32_t *)_debugHead.contents;
                uint32_t *words = (uint32_t *)_debugWords.contents;
                *error = events > 0u
                    ? [NSString stringWithFormat:
                        @"gas RK2 exhausted representable timesteps after %u halvings; gpu tag=0x%x gid=%u values=(%g,%g,%g,%g)",
                        halvings,
                        words[0],
                        words[1],
                        fluid_debug_float(words[2]),
                        fluid_debug_float(words[3]),
                        fluid_debug_float(words[4]),
                        fluid_debug_float(words[5])]
                    : [NSString stringWithFormat:
                        @"gas RK2 exhausted representable timesteps after %u halvings",
                        halvings];
            }

            return NO;
        }

        params.dt = nextDelta;
        halvings++;
    }

    std::memcpy(_density.contents, _trialDensity.contents, _trialDensity.length);
    std::memcpy(_momentum.contents, _trialMomentum.contents, _trialMomentum.length);
    std::memcpy(_internalEnergy.contents, _trialEnergy.contents, _trialEnergy.length);
    diagnostics->delta_used = params.dt;
    diagnostics->halvings = halvings;
    return YES;
}

- (SpectralModeParams)spectralParams:(float)delta {
    SpectralModeParams params = {};
    params.num_osc = _particleCount;
    params.max_carriers = _modeCount;
    params.dt = delta;
    params.gate_width_min = _gateMinimum;
    params.gate_width_max = _gateMaximum;
    params.offender_weight_floor = std::sqrt(FLT_EPSILON);
    params.volatile_decay_mul = 1.0f;
    params.stable_decay_mul = 1.0f;
    params.crystallized_decay_mul = 1.0f;
    params.crystallized_coupling_boost = 1.0f;
    params.crystallize_age = 1u;
    params.domain_x = _config.grid_x * _config.spacing;
    params.domain_y = _config.grid_y * _config.spacing;
    params.domain_z = _config.grid_z * _config.spacing;
    params.spatial_sigma = _spatialSigma;
    params.metabolic_rate = 0.5f;
    return params;
}

- (GPEParams)gpeParams:(float)delta {
    // The restored Sensorium model uses nondimensional natural units and an
    // attractive cubic interaction. These are model equations, not recovery
    // defaults; changing their sign or scale changes the represented physics.
    // energy_decay is filled by advanceWave (Lindblad/SOC); unused here.
    return GPEParams{
        delta,
        1.0f,
        1.0f,
        -1.0f,
        0.0f,
        0.0f,
        _omegaSpacing > 0.0f ? 1.0f / (_omegaSpacing * _omegaSpacing) : 0.0f,
        FluidAnchorSlots,
        _randomSeed,
        std::sqrt(FLT_EPSILON),
    };
}

- (BOOL)gather:(float)delta error:(NSString **)error {
    id<MTLComputePipelineState> pipeline = [self pipeline:@"pic_gather_update_particles" error:error];

    if (pipeline == nil) {
        return NO;
    }

    // R_specific = (gamma - 1) * c_v = 0.4 in the restored nondimensional gas.
    // Gravity is enabled after solveGravity fills φ (thermo.step parity).
    PicGatherParams params = {
        _particleCount,
        _config.grid_x,
        _config.grid_y,
        _config.grid_z,
        _config.spacing,
        1.0f / _config.spacing,
        delta,
        _config.grid_x * _config.spacing,
        _config.grid_y * _config.spacing,
        _config.grid_z * _config.spacing,
        1.4f,
        0.4f,
        1.0f,
        1.0e-3f,
        1.0e-3f,
        1.0f,
    };
    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:pipeline command:&command];
    NSArray<id<MTLBuffer>> *buffers = @[
        _position, _mass, _positionOutput, _velocityOutput, _heatOutput,
        _density, _momentum, _internalEnergy, _gravityPotential,
    ];

    for (NSUInteger index = 0; index < buffers.count; index++) {
        [encoder setBuffer:buffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&params length:sizeof(params) atIndex:9];
    [encoder setBuffer:_debugHead offset:0 atIndex:10];
    [encoder setBuffer:_debugWords offset:0 atIndex:11];
    uint32_t debugCapacity = FluidDebugCapacity;
    [encoder setBytes:&debugCapacity length:sizeof(debugCapacity) atIndex:12];
    [self dispatch:encoder count:_particleCount pipeline:pipeline];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    // Validate gather heat only; commit/clamp happen after Planck (thermo.step).
    size_t liveScalar = (size_t)_particleCount * sizeof(float);

    if (![self blitFrom:_heatOutput sourceOffset:0 to:_hostHeat destinationOffset:0 bytes:liveScalar error:error]) {
        return NO;
    }

    float *heat = (float *)_hostHeat.contents;

    for (uint32_t index = 0; index < _particleCount; index++) {
        if (!std::isfinite(heat[index]) || heat[index] < 0.0f) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"PIC gather produced invalid heat at particle %u", index];
            }

            return NO;
        }
    }

    return YES;
}

- (BOOL)advancePilotWave:(float)delta
             diagnostics:(FluidDiagnostics *)diagnostics
                   error:(NSString **)error {
    // quantum_flow.step: project Ψ(x) → Bohm current advection
    // (Ψ smoothing omitted — QuantumFlowConfig defaults steps=0).
    id<MTLComputePipelineState> project = [self pipeline:@"project_modes_to_spatial_psi" error:error];
    id<MTLComputePipelineState> advect = [self pipeline:@"pic_gather_update_particles_pilot_wave" error:error];

    if (project == nil || advect == nil) {
        return NO;
    }

    std::memset(_spatialPsiReal.contents, 0, _spatialPsiReal.length);
    std::memset(_spatialPsiImaginary.contents, 0, _spatialPsiImaginary.length);
    ModeProjectParams projectParams = {
        _modeCount,
        _particleCount,
        FluidAnchorSlots,
        _config.grid_x,
        _config.grid_y,
        _config.grid_z,
        _config.spacing,
        1.0f / _config.spacing,
    };
    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:project command:&command];
    NSArray<id<MTLBuffer>> *projectBuffers = @[
        _psiReal, _psiImaginary, _anchorIndex, _anchorWeight, _position,
        _spatialPsiReal, _spatialPsiImaginary,
    ];

    for (NSUInteger index = 0; index < projectBuffers.count; index++) {
        [encoder setBuffer:projectBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&projectParams length:sizeof(projectParams) atIndex:7];
    [self dispatch:encoder count:_modeCount * FluidAnchorSlots pipeline:project];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    PilotWaveParams pilotParams = {
        _particleCount,
        _config.grid_x,
        _config.grid_y,
        _config.grid_z,
        _config.spacing,
        1.0f / _config.spacing,
        delta,
        _config.grid_x * _config.spacing,
        _config.grid_y * _config.spacing,
        _config.grid_z * _config.spacing,
        1.0f,
        1.0e-8f,
        1.0e-6f,
    };
    command = nil;
    encoder = [self encoder:advect command:&command];
    NSArray<id<MTLBuffer>> *pilotBuffers = @[
        _position, _mass, _positionOutput, _velocityOutput,
        _spatialPsiReal, _spatialPsiImaginary,
    ];

    for (NSUInteger index = 0; index < pilotBuffers.count; index++) {
        [encoder setBuffer:pilotBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&pilotParams length:sizeof(pilotParams) atIndex:6];
    [self dispatch:encoder count:_particleCount pipeline:advect];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    size_t liveVector = (size_t)_particleCount * 3u * sizeof(float);

    if (![self blitFrom:_positionOutput sourceOffset:0 to:_hostPosition destinationOffset:0 bytes:liveVector error:error] ||
        ![self blitFrom:_velocityOutput sourceOffset:0 to:_hostVelocity destinationOffset:0 bytes:liveVector error:error]) {
        return NO;
    }

    float *position = (float *)_hostPosition.contents;
    float *velocity = (float *)_hostVelocity.contents;
    float guidanceSquared = 0.0f;

    for (uint32_t index = 0; index < _particleCount * 3u; index++) {
        if (!std::isfinite(position[index]) || !std::isfinite(velocity[index])) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"pilot-wave transport is non-finite at component %u", index];
            }

            return NO;
        }

        guidanceSquared += velocity[index] * velocity[index];
    }

    diagnostics->guidance_rms = std::sqrt(guidanceSquared / (float)_particleCount);
    return [self blitFrom:_positionOutput sourceOffset:0 to:_position destinationOffset:0 bytes:liveVector error:error] &&
        [self blitFrom:_velocityOutput sourceOffset:0 to:_velocity destinationOffset:0 bytes:liveVector error:error];
}

- (BOOL)planckExchange:(float)delta error:(NSString **)error {
    id<MTLComputePipelineState> pipeline = [self pipeline:@"planck_exchange" error:error];

    if (pipeline == nil) {
        return NO;
    }

    PlanckExchangeParams params = {
        _particleCount,
        delta,
        1.0e-4f * 1.4f / 0.71f,
        0.5f * _config.spacing,
    };
    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:pipeline command:&command];
    [encoder setBuffer:_heat offset:0 atIndex:0];
    [encoder setBuffer:_energy offset:0 atIndex:1];
    [encoder setBuffer:_omega offset:0 atIndex:2];
    [encoder setBuffer:_mass offset:0 atIndex:3];
    [encoder setBytes:&params length:sizeof(params) atIndex:4];
    [self dispatch:encoder count:_particleCount pipeline:pipeline];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    size_t liveScalar = (size_t)_particleCount * sizeof(float);

    if (![self blitFrom:_heat sourceOffset:0 to:_hostHeat destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_energy sourceOffset:0 to:_hostEnergy destinationOffset:0 bytes:liveScalar error:error]) {
        return NO;
    }

    float *heat = (float *)_hostHeat.contents;
    float *energy = (float *)_hostEnergy.contents;

    for (uint32_t index = 0; index < _particleCount; index++) {
        if (!std::isfinite(heat[index]) || !std::isfinite(energy[index]) ||
            heat[index] < 0.0f || energy[index] < 0.0f) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"Planck exchange failed at particle %u", index];
            }

            return NO;
        }
    }

    return YES;
}

- (void)deriveSpatialSigma {
    // omegawave: σ_x = √(2π) / √(m̄ T̄) with c_v=1 in ω-natural units.
    NSString *pullError = nil;
    size_t liveScalar = (size_t)_particleCount * sizeof(float);

    if (![self blitFrom:_heat sourceOffset:0 to:_hostHeat destinationOffset:0 bytes:liveScalar error:&pullError] ||
        ![self blitFrom:_mass sourceOffset:0 to:_hostMass destinationOffset:0 bytes:liveScalar error:&pullError]) {
        return;
    }

    float *heat = (float *)_hostHeat.contents;
    float *mass = (float *)_hostMass.contents;
    float meanTemperature = 0.0f;
    float meanMass = 0.0f;
    uint32_t counted = 0u;

    for (uint32_t index = 0; index < _particleCount; index++) {
        if (!(mass[index] > 0.0f)) {
            continue;
        }

        meanTemperature += heat[index] / mass[index];
        meanMass += mass[index];
        counted++;
    }

    if (counted == 0u) {
        _meanTemperature = 0.0f;
        return;
    }

    meanTemperature /= (float)counted;
    meanMass /= (float)counted;
    _meanTemperature = meanTemperature;
    float domainLimit = 0.5f * std::min(
        _config.grid_x * _config.spacing,
        std::min(_config.grid_y * _config.spacing, _config.grid_z * _config.spacing)
    );
    float sigma = domainLimit;

    if (std::isfinite(meanMass * meanTemperature) && meanMass * meanTemperature > 0.0f) {
        sigma = std::sqrt(2.0f * (float)M_PI) / std::sqrt(meanMass * meanTemperature);
    }

    _spatialSigma = std::clamp(sigma, _config.spacing, domainLimit);
}

- (BOOL)applyCouplingWeights:(NSString **)error {
    // omegawave._renormalized_coupling_weights: κ(ω) ∝ 1/√ω_rel, mean-normalized.
    size_t liveScalar = (size_t)_particleCount * sizeof(float);

    if (![self blitFrom:_amplitude sourceOffset:0 to:_hostAmplitude destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_omega sourceOffset:0 to:_hostOmega destinationOffset:0 bytes:liveScalar error:error]) {
        return NO;
    }

    float *amplitude = (float *)_hostAmplitude.contents;
    float *omega = (float *)_hostOmega.contents;
    float omegaMin = omega[0];

    for (uint32_t index = 1; index < _particleCount; index++) {
        omegaMin = std::min(omegaMin, omega[index]);
    }

    float omegaFloor = _omegaSpacing > 0.0f ? _omegaSpacing : 1.0e-3f;
    std::vector<float> kappa((size_t)_particleCount);
    float meanKappa = 0.0f;

    for (uint32_t index = 0; index < _particleCount; index++) {
        float omegaRel = std::max(omega[index] - omegaMin + omegaFloor, omegaFloor);
        kappa[index] = 1.0f / std::sqrt(omegaRel);
        meanKappa += kappa[index];
    }

    meanKappa = std::max(meanKappa / (float)_particleCount, 1.0e-6f);

    for (uint32_t index = 0; index < _particleCount; index++) {
        amplitude[index] *= kappa[index] / meanKappa;
    }

    return [self blitFrom:_hostAmplitude sourceOffset:0 to:_amplitude destinationOffset:0 bytes:liveScalar error:error];
}

- (void)applySeparationSoliton:(float)delta {
    // omegawave._apply_separation_shift + _apply_soliton_projection on Ψ(ω).
    float *real = (float *)_psiReal.contents;
    float *imaginary = (float *)_psiImaginary.contents;
    const uint32_t modes = _modeCount;

    if (modes < 3u || !(delta > 0.0f)) {
        return;
    }

    std::vector<float> density((size_t)modes);
    std::vector<float> vRep((size_t)modes);
    const float sepSigma = 1.0f;
    const float sepStrength = 1.0f;
    const int radius = (int)std::ceil(4.0f * sepSigma);
    std::vector<float> kernel((size_t)(2 * radius + 1));
    float kernelSum = 0.0f;

    for (int offset = -radius; offset <= radius; offset++) {
        float weight = std::exp(-0.5f * ((float)offset / sepSigma) * ((float)offset / sepSigma));
        kernel[(size_t)(offset + radius)] = weight;
        kernelSum += weight;
    }

    kernelSum = std::max(kernelSum, 1.0e-12f);

    for (float &weight : kernel) {
        weight /= kernelSum;
    }

    for (uint32_t mode = 0; mode < modes; mode++) {
        density[mode] = real[mode] * real[mode] + imaginary[mode] * imaginary[mode];
    }

    for (uint32_t mode = 0; mode < modes; mode++) {
        float sum = 0.0f;

        for (int offset = -radius; offset <= radius; offset++) {
            int sample = (int)mode + offset;

            if (sample < 0 || sample >= (int)modes) {
                continue;
            }

            sum += density[(size_t)sample] * kernel[(size_t)(offset + radius)];
        }

        vRep[mode] = sum;
    }

    std::vector<float> shift((size_t)modes);
    shift[0] = std::clamp(-sepStrength * delta * (vRep[1] - vRep[0]), -0.5f, 0.5f);
    shift[modes - 1u] = std::clamp(
        -sepStrength * delta * (vRep[modes - 1u] - vRep[modes - 2u]), -0.5f, 0.5f
    );

    for (uint32_t mode = 1; mode + 1u < modes; mode++) {
        float grad = 0.5f * (vRep[mode + 1u] - vRep[mode - 1u]);
        shift[mode] = std::clamp(-sepStrength * delta * grad, -0.5f, 0.5f);
    }

    std::vector<float> outReal((size_t)modes);
    std::vector<float> outImag((size_t)modes);
    float denom = (float)std::max(modes - 1u, 1u);

    for (uint32_t mode = 0; mode < modes; mode++) {
        float sample = (float)mode - shift[mode];
        float x0 = std::floor(sample);
        int i0 = (int)x0;
        int i1 = i0 + 1;
        float frac = sample - x0;

        if (i0 < 0 || i1 >= (int)modes) {
            outReal[mode] = 0.0f;
            outImag[mode] = 0.0f;
            continue;
        }

        outReal[mode] = real[(size_t)i0] * (1.0f - frac) + real[(size_t)i1] * frac;
        outImag[mode] = imaginary[(size_t)i0] * (1.0f - frac) + imaginary[(size_t)i1] * frac;
        (void)denom;
    }

    // Soliton double-well projection (strength=1).
    std::vector<float> amp((size_t)modes);

    for (uint32_t mode = 0; mode < modes; mode++) {
        amp[mode] = std::hypot(outReal[mode], outImag[mode]);
    }

    std::vector<float> sorted = amp;
    std::nth_element(sorted.begin(), sorted.begin() + (sorted.size() * 3u) / 4u, sorted.end());
    float aStar = sorted[(sorted.size() * 3u) / 4u];

    if (!(aStar > 0.0f) || !std::isfinite(aStar)) {
        std::memcpy(real, outReal.data(), (size_t)modes * sizeof(float));
        std::memcpy(imaginary, outImag.data(), (size_t)modes * sizeof(float));
        return;
    }

    float lambda = 2.0f / std::max(aStar * aStar, 1.0e-6f);
    float stepCap = 0.25f * aStar;

    for (uint32_t mode = 0; mode < modes; mode++) {
        float a = amp[mode];
        float dV = 2.0f * lambda * a * (a - aStar) * (2.0f * a - aStar);
        float step = std::clamp(delta * dV, -stepCap, stepCap);
        float aNew = std::max(a - step, 0.0f);
        float aBlend = 0.5f * (a + aNew);
        float phase = std::atan2(outImag[mode], outReal[mode]);
        real[mode] = aBlend * std::cos(phase);
        imaginary[mode] = aBlend * std::sin(phase);
    }
}

- (BOOL)advanceWave:(float)delta diagnostics:(FluidDiagnostics *)diagnostics error:(NSString **)error {
    // omegawave.step (single-head): σₓ, Lindblad/SOC decay, κ(σ), Planck-bias
    // amp weights, accumulate → GPE → separation → soliton → phase update.
    // Re-seed anchors on the post-thermo population so quantum_flow projection
    // has support under the modes the particles actually occupy.
    [self seedAnchors];
    [self deriveSpatialSigma];
    id<MTLComputePipelineState> amplitudePipeline =
        [self pipeline:@"particle_amplitude_from_energy" error:error];

    if (amplitudePipeline == nil) {
        return NO;
    }

    id<MTLCommandBuffer> amplitudeCommand = nil;
    id<MTLComputeCommandEncoder> amplitudeEncoder =
        [self encoder:amplitudePipeline command:&amplitudeCommand];
    [amplitudeEncoder setBuffer:_energy offset:0 atIndex:0];
    [amplitudeEncoder setBuffer:_amplitude offset:0 atIndex:1];
    [amplitudeEncoder setBytes:&_particleCount length:sizeof(_particleCount) atIndex:2];
    [self dispatch:amplitudeEncoder count:_particleCount pipeline:amplitudePipeline];

    if (![self finish:amplitudeEncoder command:amplitudeCommand error:error] ||
        ![self applyCouplingWeights:error]) {
        return NO;
    }

    float domainScale = std::min(
        _config.grid_x * _config.spacing,
        std::min(_config.grid_y * _config.spacing, _config.grid_z * _config.spacing)
    );
    float kappaPhysical = _spatialSigma > 0.0f
        ? (domainScale / _spatialSigma) * (domainScale / _spatialSigma)
        : 1.0f;
    float couplingScale = kappaPhysical / std::sqrt((float)_modeCount);

    // Lindblad Γ = γ · T (ħ=k_B=1) with SOC flux boost from ||ΔΨ||.
    const float bathCoupling = 0.1f;
    float energyDecay = bathCoupling * std::max(_meanTemperature, 0.0f);
    float flux = 0.0f;

    if (_previousPsiReal.size() == _modeCount && _previousPsiImaginary.size() == _modeCount) {
        float *real = (float *)_psiReal.contents;
        float *imaginary = (float *)_psiImaginary.contents;
        float mag2 = 0.0f;

        for (uint32_t mode = 0; mode < _modeCount; mode++) {
            float dReal = real[mode] - _previousPsiReal[mode];
            float dImag = imaginary[mode] - _previousPsiImaginary[mode];
            mag2 += dReal * dReal + dImag * dImag;
        }

        mag2 /= (float)_modeCount;
        float dtSafe = delta > 0.0f ? delta : 1.0e-9f;
        flux = mag2 / (dtSafe * dtSafe);
    }

    energyDecay *= 1.0f + 10.0f * std::min(flux, 10.0f);
    energyDecay = std::clamp(energyDecay, 1.0e-6f, 20.0f);

    std::memset(_modeAccumulators.contents, 0, _modeAccumulators.length);
    _randomSeed++;
    SpectralModeParams spectral = [self spectralParams:delta];
    spectral.coupling_scale = couplingScale;
    spectral.rng_seed = _randomSeed;
    GPEParams gpe = [self gpeParams:delta];
    gpe.energy_decay = energyDecay;
    gpe.rng_seed = _randomSeed;
    id<MTLComputePipelineState> accumulate = [self pipeline:@"coherence_accumulate_forces" error:error];
    id<MTLComputePipelineState> gpePipeline = [self pipeline:@"coherence_gpe_step" error:error];
    id<MTLComputePipelineState> phasePipeline = [self pipeline:@"coherence_update_oscillator_phases" error:error];

    if (accumulate == nil || gpePipeline == nil || phasePipeline == nil) {
        return NO;
    }

    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:accumulate command:&command];
    NSArray<id<MTLBuffer>> *accumulateBuffers = @[
        _phase, _omega, _amplitude, _position, _modeOmega, _modeLinewidth,
        _anchorIndex, _anchorWeight, _modeAccumulators,
    ];

    for (NSUInteger index = 0; index < accumulateBuffers.count; index++) {
        [encoder setBuffer:accumulateBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&spectral length:sizeof(spectral) atIndex:9];
    [encoder setBuffer:_modeCountBuffer offset:0 atIndex:10];
    [encoder setBuffer:_binStarts offset:0 atIndex:11];
    [encoder setBuffer:_binnedIndex offset:0 atIndex:12];
    [encoder setBuffer:_binParams offset:0 atIndex:13];
    [encoder setBytes:&_modeCount length:sizeof(_modeCount) atIndex:14];
    [encoder setBuffer:_heat offset:0 atIndex:15];
    [encoder setThreadgroupMemoryLength:FluidMaximumWaveModes * 32u atIndex:0];
    [self dispatch:encoder count:_particleCount pipeline:accumulate];
    [encoder endEncoding];
    encoder = [command computeCommandEncoder];
    [encoder setComputePipelineState:gpePipeline];
    NSArray<id<MTLBuffer>> *gpeBuffers = @[
        _phase, _omega, _amplitude, _psiReal, _psiImaginary, _modeOmega,
        _modeLinewidth, _anchorIndex, _anchorWeight, _modeAccumulators,
        _modeCountBuffer, _position,
    ];

    for (NSUInteger index = 0; index < gpeBuffers.count; index++) {
        [encoder setBuffer:gpeBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&spectral length:sizeof(spectral) atIndex:12];
    [encoder setBytes:&gpe length:sizeof(gpe) atIndex:13];
    [encoder setThreadgroupMemoryLength:(NSUInteger)_modeCount * 4u * sizeof(float) atIndex:0];
    [encoder dispatchThreadgroups:MTLSizeMake(1u, 1u, 1u)
            threadsPerThreadgroup:MTLSizeMake(_modeCount, 1u, 1u)];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    [self applySeparationSoliton:delta];

    command = nil;
    encoder = [self encoder:phasePipeline command:&command];
    NSArray<id<MTLBuffer>> *phaseBuffers = @[
        _phase, _omega, _amplitude, _psiReal, _psiImaginary, _modeOmega,
        _modeLinewidth, _anchorIndex, _anchorWeight, _modeCountBuffer,
    ];

    for (NSUInteger index = 0; index < phaseBuffers.count; index++) {
        [encoder setBuffer:phaseBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&spectral length:sizeof(spectral) atIndex:10];
    [encoder setBuffer:_binStarts offset:0 atIndex:11];
    [encoder setBuffer:_binnedIndex offset:0 atIndex:12];
    [encoder setBuffer:_binParams offset:0 atIndex:13];
    [encoder setBytes:&_modeCount length:sizeof(_modeCount) atIndex:14];
    [encoder setBuffer:_position offset:0 atIndex:15];
    [self dispatch:encoder count:_particleCount pipeline:phasePipeline];

    if (![self finish:encoder command:command error:error]) {
        return NO;
    }

    float *real = (float *)_psiReal.contents;
    float *imaginary = (float *)_psiImaginary.contents;

    for (uint32_t mode = 0; mode < _modeCount; mode++) {
        if (!std::isfinite(real[mode]) || !std::isfinite(imaginary[mode])) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"omega wave is non-finite at mode %u", mode];
            }

            return NO;
        }
    }

    if (![self blitFrom:_phase sourceOffset:0 to:_hostPhase destinationOffset:0 bytes:(size_t)_particleCount * sizeof(float) error:error] ||
        ![self blitFrom:_heat sourceOffset:0 to:_hostHeat destinationOffset:0 bytes:(size_t)_particleCount * sizeof(float) error:error]) {
        return NO;
    }

    float *phase = (float *)_hostPhase.contents;
    float *heat = (float *)_hostHeat.contents;

    for (uint32_t particle = 0; particle < _particleCount; particle++) {
        if (!std::isfinite(phase[particle]) || !std::isfinite(heat[particle]) || heat[particle] < 0.0f) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"omega coupling is invalid at particle %u", particle];
            }

            return NO;
        }
    }

    float squared = 0.0f;
    float deltaSquared = 0.0f;
    BOOL hasPreviousAmplitude = _previousAmplitude.size() == _modeCount;

    if (!hasPreviousAmplitude) {
        _previousAmplitude.resize(_modeCount);
    }

    _previousPsiReal.resize(_modeCount);
    _previousPsiImaginary.resize(_modeCount);

    for (uint32_t mode = 0; mode < _modeCount; mode++) {
        float current = std::hypot(real[mode], imaginary[mode]);
        float difference = hasPreviousAmplitude ? current - _previousAmplitude[mode] : 0.0f;
        squared += current * current;
        deltaSquared += difference * difference;
        _previousAmplitude[mode] = current;
        _previousPsiReal[mode] = real[mode];
        _previousPsiImaginary[mode] = imaginary[mode];
    }

    diagnostics->psi_rms = std::sqrt(squared / (float)_modeCount);
    diagnostics->psi_delta_rms = std::sqrt(deltaSquared / (float)_modeCount);
    return YES;
}

/*
retainParticles keeps only the listed resident indices, preserving SoA state and
content IDs. Indices must be unique and in range. Used to drop inert mass after
Advance without inventing a parallel particle store.
*/
- (BOOL)retainParticles:(const uint32_t *)indices
                  count:(uint32_t)count
                  error:(NSString **)error {
    if (count > _particleCount) {
        if (error != nil) {
            *error = @"retain count exceeds resident population";
        }

        return NO;
    }

    if (count == 0u) {
        _particleCount = 0u;
        return YES;
    }

    if (indices == nullptr) {
        if (error != nil) {
            *error = @"retain indices are required";
        }

        return NO;
    }

    if (count == _particleCount) {
        return YES;
    }

    if (![self pullParticles:error]) {
        return NO;
    }

    std::vector<uint8_t> seen((size_t)_particleCount, 0u);
    std::vector<float> position((size_t)count * 3u);
    std::vector<float> velocity((size_t)count * 3u);
    std::vector<float> mass(count);
    std::vector<float> heat(count);
    std::vector<float> energy(count);
    std::vector<float> phase(count);
    std::vector<float> omega(count);
    std::vector<float> amplitude(count);
    std::vector<uint32_t> content(count);
    std::vector<uint32_t> clamped(count);

    float *srcPosition = (float *)_hostPosition.contents;
    float *srcVelocity = (float *)_hostVelocity.contents;
    float *srcMass = (float *)_hostMass.contents;
    float *srcHeat = (float *)_hostHeat.contents;
    float *srcEnergy = (float *)_hostEnergy.contents;
    float *srcPhase = (float *)_hostPhase.contents;
    float *srcOmega = (float *)_hostOmega.contents;
    float *srcAmplitude = (float *)_hostAmplitude.contents;
    uint32_t *srcContent = (uint32_t *)_hostContent.contents;
    uint32_t *srcClamped = (uint32_t *)_hostClamped.contents;

    for (uint32_t out = 0; out < count; out++) {
        uint32_t slot = indices[out];

        if (slot >= _particleCount || seen[slot] != 0u) {
            if (error != nil) {
                *error = @"retain indices must be unique and in range";
            }

            return NO;
        }

        seen[slot] = 1u;
        uint32_t base = slot * 3u;
        uint32_t outBase = out * 3u;
        position[outBase] = srcPosition[base];
        position[outBase + 1u] = srcPosition[base + 1u];
        position[outBase + 2u] = srcPosition[base + 2u];
        velocity[outBase] = srcVelocity[base];
        velocity[outBase + 1u] = srcVelocity[base + 1u];
        velocity[outBase + 2u] = srcVelocity[base + 2u];
        mass[out] = srcMass[slot];
        heat[out] = srcHeat[slot];
        energy[out] = srcEnergy[slot];
        phase[out] = srcPhase[slot];
        omega[out] = srcOmega[slot];
        amplitude[out] = srcAmplitude[slot];
        content[out] = srcContent[slot];
        clamped[out] = srcClamped[slot];
    }

    std::memcpy(srcPosition, position.data(), position.size() * sizeof(float));
    std::memcpy(srcVelocity, velocity.data(), velocity.size() * sizeof(float));
    std::memcpy(srcMass, mass.data(), mass.size() * sizeof(float));
    std::memcpy(srcHeat, heat.data(), heat.size() * sizeof(float));
    std::memcpy(srcEnergy, energy.data(), energy.size() * sizeof(float));
    std::memcpy(srcPhase, phase.data(), phase.size() * sizeof(float));
    std::memcpy(srcOmega, omega.data(), omega.size() * sizeof(float));
    std::memcpy(srcAmplitude, amplitude.data(), amplitude.size() * sizeof(float));
    std::memcpy(srcContent, content.data(), content.size() * sizeof(uint32_t));
    std::memcpy(srcClamped, clamped.data(), clamped.size() * sizeof(uint32_t));

    _particleCount = count;

    if (![self pushParticleRange:0u count:count error:error] ||
        ![self blitFrom:_hostClamped sourceOffset:0 to:_clamped destinationOffset:0
                 bytes:(size_t)count * sizeof(uint32_t) error:error]) {
        return NO;
    }

    if (_particleCount == 0u || !_waveInitialized) {
        return YES;
    }

    return [self computeSpatialIDs:error];
}

- (BOOL)readParticles:(FluidParticle *)particles
                start:(uint32_t)start
                count:(uint32_t)count
                error:(NSString **)error {
    if (particles == nullptr || count == 0u) {
        if (error != nil) {
            *error = @"particle read buffer is required";
        }

        return NO;
    }

    if (start > _particleCount || count > _particleCount - start) {
        if (error != nil) {
            *error = @"particle read range exceeds resident population";
        }

        return NO;
    }

    if (![self pullParticles:error]) {
        return NO;
    }

    float *position = (float *)_hostPosition.contents;
    float *velocity = (float *)_hostVelocity.contents;
    float *mass = (float *)_hostMass.contents;
    float *heat = (float *)_hostHeat.contents;
    float *energy = (float *)_hostEnergy.contents;
    float *phase = (float *)_hostPhase.contents;
    float *omega = (float *)_hostOmega.contents;

    for (uint32_t index = 0; index < count; index++) {
        uint32_t slot = start + index;
        uint32_t base = slot * 3u;
        particles[index] = FluidParticle{
            position[base], position[base + 1u], position[base + 2u],
            velocity[base], velocity[base + 1u], velocity[base + 2u],
            mass[slot], heat[slot], energy[slot], phase[slot], omega[slot],
        };
    }

    return YES;
}

- (BOOL)appendParticles:(const FluidParticle *)particles
             contentIDs:(const uint32_t *)contentIDs
                  count:(uint32_t)count
                  start:(uint32_t *)start
                  error:(NSString **)error {
    if (particles == nullptr || contentIDs == nullptr || count == 0u || start == nullptr) {
        if (error != nil) {
            *error = @"append requires particles, content IDs, and a start out";
        }

        return NO;
    }

    uint32_t offset = _particleCount;

    if (![self ensureCapacity:offset + count error:error]) {
        return NO;
    }

    if (![self writeParticles:particles contentIDs:contentIDs count:count offset:offset error:error]) {
        return NO;
    }

    _particleCount = offset + count;
    *start = offset;
    return YES;
}

- (BOOL)mergeInelastic:(NSString **)error {
    // thermodynamics._merge_inelastic_collisions on device: key/sort/reduce.
    if (_particleCount < 2u) {
        return YES;
    }

    uint32_t padded = 1u;

    while (padded < _particleCount) {
        padded *= 2u;
    }

    if (padded > _mergePadCapacity) {
        if (error != nil) {
            *error = @"merge pad capacity is insufficient";
        }

        return NO;
    }

    id<MTLComputePipelineState> keysPipeline = [self pipeline:@"merge_compute_keys" error:error];
    id<MTLComputePipelineState> fillPipeline = [self pipeline:@"bitonic_fill_indices" error:error];
    id<MTLComputePipelineState> bitonicPipeline =
        [self pipeline:@"bitonic_compare_exchange" error:error];
    id<MTLComputePipelineState> reducePipeline = [self pipeline:@"merge_reduce_runs" error:error];

    if (keysPipeline == nil || fillPipeline == nil || bitonicPipeline == nil ||
        reducePipeline == nil) {
        return NO;
    }

    MergeParams params = {
        _particleCount,
        padded,
        _config.grid_x,
        _config.grid_y,
        _config.grid_z,
        1.0f / _config.spacing,
    };
    *(uint32_t *)_mergeCount.contents = 0u;

    // One command buffer for keying, bitonic sort, and reduce — no mid-sort sync.
    id<MTLCommandBuffer> command = [_queue commandBuffer];
    id<MTLComputeCommandEncoder> encoder = [command computeCommandEncoder];
    [encoder setComputePipelineState:keysPipeline];
    [encoder setBuffer:_position offset:0 atIndex:0];
    [encoder setBuffer:_content offset:0 atIndex:1];
    [encoder setBuffer:_mergeKeys offset:0 atIndex:2];
    [encoder setBytes:&params length:sizeof(params) atIndex:3];
    [self dispatch:encoder count:padded pipeline:keysPipeline];
    [encoder endEncoding];

    encoder = [command computeCommandEncoder];
    [encoder setComputePipelineState:fillPipeline];
    [encoder setBuffer:_mergeIndices offset:0 atIndex:0];
    [encoder setBytes:&padded length:sizeof(padded) atIndex:1];
    [self dispatch:encoder count:padded pipeline:fillPipeline];
    [encoder endEncoding];

    for (uint32_t k = 2u; k <= padded; k <<= 1u) {
        for (uint32_t j = k >> 1u; j > 0u; j >>= 1u) {
            encoder = [command computeCommandEncoder];
            [encoder setComputePipelineState:bitonicPipeline];
            [encoder setBuffer:_mergeIndices offset:0 atIndex:0];
            [encoder setBuffer:_mergeKeys offset:0 atIndex:1];
            [encoder setBytes:&j length:sizeof(j) atIndex:2];
            [encoder setBytes:&k length:sizeof(k) atIndex:3];
            [encoder setBytes:&padded length:sizeof(padded) atIndex:4];
            [self dispatch:encoder count:padded pipeline:bitonicPipeline];
            [encoder endEncoding];
        }
    }

    encoder = [command computeCommandEncoder];
    [encoder setComputePipelineState:reducePipeline];
    // sortedOriginalIndex holds compacted clamped-OR flags until blit to _clamped.
    NSArray<id<MTLBuffer>> *reduceBuffers = @[
        _mergeIndices, _mergeKeys, _position, _velocity, _mass, _heat, _energy,
        _phase, _omega, _content, _clamped, _sortedPosition, _sortedVelocity,
        _sortedMass, _sortedHeat, _sortedEnergy, _mergePhase, _mergeOmega,
        _mergeAmplitude, _mergeContent, _sortedOriginalIndex, _mergeCount,
    ];

    for (NSUInteger index = 0; index < reduceBuffers.count; index++) {
        [encoder setBuffer:reduceBuffers[index] offset:0 atIndex:index];
    }

    [encoder setBytes:&params length:sizeof(params) atIndex:22];
    [self dispatch:encoder count:_particleCount pipeline:reducePipeline];
    // Encoders above already ended; commit the fused merge graph once.
    [encoder endEncoding];
    [command commit];
    [command waitUntilCompleted];

    if (command.status == MTLCommandBufferStatusError) {
        if (error != nil) {
            *error = command.error.localizedDescription ?: @"Metal merge failed";
        }

        return NO;
    }

    uint32_t written = *(uint32_t *)_mergeCount.contents;

    if (written == 0u) {
        if (error != nil) {
            *error = @"inelastic merge produced empty population";
        }

        return NO;
    }

    if (written == _particleCount) {
        return YES;
    }

    size_t liveScalar = (size_t)written * sizeof(float);
    size_t liveVector = liveScalar * 3u;
    size_t liveIndex = (size_t)written * sizeof(uint32_t);

    if (![self blitFrom:_sortedPosition sourceOffset:0 to:_position destinationOffset:0 bytes:liveVector error:error] ||
        ![self blitFrom:_sortedVelocity sourceOffset:0 to:_velocity destinationOffset:0 bytes:liveVector error:error] ||
        ![self blitFrom:_sortedMass sourceOffset:0 to:_mass destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_sortedHeat sourceOffset:0 to:_heat destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_sortedEnergy sourceOffset:0 to:_energy destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_mergePhase sourceOffset:0 to:_phase destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_mergeOmega sourceOffset:0 to:_omega destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_mergeAmplitude sourceOffset:0 to:_amplitude destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_mergeContent sourceOffset:0 to:_content destinationOffset:0 bytes:liveIndex error:error] ||
        ![self blitFrom:_sortedOriginalIndex sourceOffset:0 to:_clamped destinationOffset:0 bytes:liveIndex error:error]) {
        return NO;
    }

    _particleCount = written;
    return YES;
}

- (BOOL)advanceResident:(FluidDiagnostics *)diagnostics error:(NSString **)error {
    if (diagnostics == nullptr) {
        if (error != nil) {
            *error = @"diagnostics are required";
        }

        return NO;
    }

    if (_particleCount == 0u) {
        if (error != nil) {
            *error = @"resident particle population is empty";
        }

        return NO;
    }

    if (!_waveInitialized) {
        [self initializeWave];
    }

    *diagnostics = FluidDiagnostics{};

    // Sensorium manifold physics order:
    //   1) thermo.step  2) omegawave.step  3) quantum_flow.step
    if (![self scatter:error] ||
        ![self solveGravity:error] ||
        ![self advanceGas:diagnostics error:error] ||
        ![self gather:diagnostics->delta_used error:error]) {
        return NO;
    }

    size_t liveScalar = (size_t)_particleCount * sizeof(float);
    size_t liveVector = liveScalar * 3u;
    float dt = diagnostics->delta_used;

    // thermo: stash → Planck → clamp → commit → merge → spatial IDs.
    if (![self blitFrom:_heat sourceOffset:0 to:_sortedHeat destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_energy sourceOffset:0 to:_sortedEnergy destinationOffset:0 bytes:liveScalar error:error] ||
        ![self blitFrom:_heatOutput sourceOffset:0 to:_heat destinationOffset:0 bytes:liveScalar error:error] ||
        ![self planckExchange:dt error:error] ||
        ![self applyClamp:error] ||
        ![self blitFrom:_positionOutput sourceOffset:0 to:_position destinationOffset:0 bytes:liveVector error:error] ||
        ![self blitFrom:_velocityOutput sourceOffset:0 to:_velocity destinationOffset:0 bytes:liveVector error:error] ||
        ![self mergeInelastic:error] ||
        ![self computeSpatialIDs:error] ||
        ![self advanceWave:dt diagnostics:diagnostics error:error] ||
        ![self advancePilotWave:dt diagnostics:diagnostics error:error]) {
        return NO;
    }

    return YES;
}

- (BOOL)solveGravity:(NSString **)error {
    // Periodic Poisson ∇²φ = 4πGρ via FFT; φ̂(k) = -(4πG/k²)ρ̂(k), φ̂(0)=0.
    if (_fftSetup == nullptr || _fftInvK2.size() != (size_t)_cellCount) {
        if (error != nil) {
            *error = @"gravity FFT cache is not initialized";
        }

        return NO;
    }

    float *density = (float *)_density.contents;
    double mean = 0.0;

    for (uint32_t cell = 0; cell < _cellCount; cell++) {
        mean += (double)density[cell];
    }

    mean /= (double)_cellCount;

    for (uint32_t cell = 0; cell < _cellCount; cell++) {
        _fftScratch[cell] = {(float)((double)density[cell] - mean), 0.0f};
    }

    // Axis-contiguous 3D FFT: X fastest, then Y, then Z (matches density layout).
    const uint32_t nx = _config.grid_x;
    const uint32_t ny = _config.grid_y;
    const uint32_t nz = _config.grid_z;
    enum class Axis { X, Y, Z };
    auto transformAxis = [&](Axis axis, bool inverse) {
        uint32_t n = axis == Axis::X ? nx : (axis == Axis::Y ? ny : nz);
        uint32_t lines = _cellCount / n;
        uint32_t log2n = 0u;

        for (uint32_t value = n; value > 1u; value >>= 1u) {
            log2n++;
        }

        std::vector<float> real((size_t)n);
        std::vector<float> imag((size_t)n);
        DSPSplitComplex split{real.data(), imag.data()};

        for (uint32_t line = 0; line < lines; line++) {
            for (uint32_t index = 0; index < n; index++) {
                size_t cell = 0;

                if (axis == Axis::X) {
                    cell = (size_t)line * n + index;
                } else if (axis == Axis::Y) {
                    uint32_t z = line / nx;
                    uint32_t x = line % nx;
                    cell = (size_t)x + (size_t)index * nx + (size_t)z * nx * ny;
                } else {
                    uint32_t y = line / nx;
                    uint32_t x = line % nx;
                    cell = (size_t)x + (size_t)y * nx + (size_t)index * nx * ny;
                }

                real[index] = _fftScratch[cell].real();
                imag[index] = _fftScratch[cell].imag();
            }

            vDSP_fft_zip(_fftSetup, &split, 1, log2n, inverse ? FFT_INVERSE : FFT_FORWARD);

            for (uint32_t index = 0; index < n; index++) {
                size_t cell = 0;

                if (axis == Axis::X) {
                    cell = (size_t)line * n + index;
                } else if (axis == Axis::Y) {
                    uint32_t z = line / nx;
                    uint32_t x = line % nx;
                    cell = (size_t)x + (size_t)index * nx + (size_t)z * nx * ny;
                } else {
                    uint32_t y = line / nx;
                    uint32_t x = line % nx;
                    cell = (size_t)x + (size_t)y * nx + (size_t)index * nx * ny;
                }

                _fftScratch[cell] = {real[index], imag[index]};
            }
        }
    };

    transformAxis(Axis::X, false);
    transformAxis(Axis::Y, false);
    transformAxis(Axis::Z, false);

    const float scale = -4.0f * (float)M_PI * _config.gravity_g;

    for (uint32_t cell = 0; cell < _cellCount; cell++) {
        float factor = scale * _fftInvK2[cell];
        _fftScratch[cell] *= factor;
    }

    transformAxis(Axis::Z, true);
    transformAxis(Axis::Y, true);
    transformAxis(Axis::X, true);

    const float invCells = 1.0f / (float)_cellCount;
    float *potential = (float *)_gravityPotential.contents;

    for (uint32_t cell = 0; cell < _cellCount; cell++) {
        potential[cell] = _fftScratch[cell].real() * invCells;
    }

    return YES;
}

- (BOOL)applyClamp:(NSString **)error {
    // Restore step-start state for clamped particles into the gather/Planck
    // updates (sortedHeat/Energy hold the pre-gather originals).
    id<MTLComputePipelineState> pipeline =
        [self pipeline:@"apply_crystallization_clamp" error:error];

    if (pipeline == nil) {
        return NO;
    }

    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:pipeline command:&command];
    [encoder setBuffer:_clamped offset:0 atIndex:0];
    [encoder setBuffer:_position offset:0 atIndex:1];
    [encoder setBuffer:_velocity offset:0 atIndex:2];
    [encoder setBuffer:_sortedHeat offset:0 atIndex:3];
    [encoder setBuffer:_sortedEnergy offset:0 atIndex:4];
    [encoder setBuffer:_omega offset:0 atIndex:5];
    [encoder setBuffer:_positionOutput offset:0 atIndex:6];
    [encoder setBuffer:_velocityOutput offset:0 atIndex:7];
    [encoder setBuffer:_heat offset:0 atIndex:8];
    [encoder setBuffer:_energy offset:0 atIndex:9];
    [encoder setBuffer:_omega offset:0 atIndex:10];
    [encoder setBytes:&_particleCount length:sizeof(_particleCount) atIndex:11];
    [self dispatch:encoder count:_particleCount pipeline:pipeline];
    return [self finish:encoder command:command error:error];
}

- (BOOL)computeSpatialIDs:(NSString **)error {
    id<MTLComputePipelineState> pipeline =
        [self pipeline:@"compute_spatial_token_ids" error:error];

    if (pipeline == nil) {
        return NO;
    }

    MergeParams params = {
        _particleCount,
        _particleCount,
        _config.grid_x,
        _config.grid_y,
        _config.grid_z,
        1.0f / _config.spacing,
    };
    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:pipeline command:&command];
    [encoder setBuffer:_position offset:0 atIndex:0];
    [encoder setBuffer:_content offset:0 atIndex:1];
    [encoder setBuffer:_cellMorton offset:0 atIndex:2];
    [encoder setBuffer:_spatialTokenIDs offset:0 atIndex:3];
    [encoder setBytes:&params length:sizeof(params) atIndex:4];
    [self dispatch:encoder count:_particleCount pipeline:pipeline];
    return [self finish:encoder command:command error:error];
}

- (BOOL)readSpatialIDs:(uint32_t *)ids
                 start:(uint32_t)start
                 count:(uint32_t)count
                 error:(NSString **)error {
    if (ids == nullptr || count == 0u) {
        if (error != nil) {
            *error = @"spatial ID read requires an output buffer";
        }

        return NO;
    }

    if (start > _particleCount || count > _particleCount - start) {
        if (error != nil) {
            *error = @"spatial ID read range exceeds resident population";
        }

        return NO;
    }

    size_t liveIndex = (size_t)_particleCount * sizeof(uint32_t);

    // Staging through hostContent: same width as content/spatial uint32 lanes.
    if (![self blitFrom:_spatialTokenIDs sourceOffset:0
                     to:_hostContent destinationOffset:0 bytes:liveIndex error:error]) {
        return NO;
    }

    uint32_t *spatial = (uint32_t *)_hostContent.contents;
    std::memcpy(ids, spatial + start, (size_t)count * sizeof(uint32_t));
    return YES;
}

- (BOOL)stepParticles:(FluidParticle *)particles
                 count:(uint32_t)count
           diagnostics:(FluidDiagnostics *)diagnostics
                 error:(NSString **)error {
    if (particles == nullptr || diagnostics == nullptr || count == 0u) {
        if (error != nil) {
            *error = @"particles and diagnostics are required";
        }

        return NO;
    }

    // Legacy one-shot replace: host supplies the complete population for this
    // step. Streaming callers should prefer appendParticles + advanceResident.
    if (![self ensureCapacity:count error:error]) {
        return NO;
    }

    // Legacy Step has no tokenizer content IDs; give each host slot a distinct
    // identity so replace-path particles do not falsely inelastic-merge.
    std::vector<uint32_t> contentIDs((size_t)count);

    for (uint32_t index = 0; index < count; index++) {
        contentIDs[index] = index;
    }

    _particleCount = count;

    if (![self writeParticles:particles contentIDs:contentIDs.data() count:count offset:0u error:error]) {
        return NO;
    }

    if (![self advanceResident:diagnostics error:error]) {
        return NO;
    }

    // Merge may compact; copy the live prefix and clear any host ghosts.
    if (![self readParticles:particles start:0u count:_particleCount error:error]) {
        return NO;
    }

    for (uint32_t index = _particleCount; index < count; index++) {
        particles[index] = FluidParticle{};
    }

    return YES;
}

- (BOOL)readWave:(FluidWaveMode *)modes count:(uint32_t)count error:(NSString **)error {
    if (modes == nullptr || count != _modeCount) {
        if (error != nil) {
            *error = @"wave output length does not match the omega lattice";
        }

        return NO;
    }

    float *omega = (float *)_modeOmega.contents;
    float *real = (float *)_psiReal.contents;
    float *imaginary = (float *)_psiImaginary.contents;
    float *linewidth = (float *)_modeLinewidth.contents;

    for (uint32_t index = 0; index < count; index++) {
        modes[index] = FluidWaveMode{omega[index], real[index], imaginary[index], linewidth[index]};
    }

	return YES;
}

- (BOOL)read:(FluidReading *)reading error:(NSString **)error {
    if (reading == nullptr || _particleCount == 0u || !_waveInitialized) {
        if (error != nil) {
            *error = @"resident fluid reading is unavailable before the first step";
        }

        return NO;
    }

    if (![self pullParticles:error]) {
        return NO;
    }

    float *position = (float *)_hostPosition.contents;
    float *density = (float *)_density.contents;
    float *momentum = (float *)_momentum.contents;
    float *energy = (float *)_internalEnergy.contents;
    float *waveReal = (float *)_psiReal.contents;
    float *waveImaginary = (float *)_psiImaginary.contents;
    float *velocity = (float *)_hostVelocity.contents;
    uint32_t dimensions[3] = {_config.grid_x, _config.grid_y, _config.grid_z};
    uint32_t centroid[3] = {};

    for (uint32_t axis = 0u; axis < 3u; axis++) {
        float domainLength = (float)dimensions[axis] * _config.spacing;
        double sine = 0.0;
        double cosine = 0.0;

        for (uint32_t particle = 0u; particle < _particleCount; particle++) {
            double angle = 2.0 * M_PI * position[particle * 3u + axis] / domainLength;
            sine += std::sin(angle);
            cosine += std::cos(angle);
        }

        double angle = std::atan2(sine, cosine);

        if (angle < 0.0) {
            angle += 2.0 * M_PI;
        }

        centroid[axis] = std::min(
            dimensions[axis] - 1u,
            (uint32_t)std::floor(angle * (double)dimensions[axis] / (2.0 * M_PI))
        );
    }

    auto cellIndex = [&](int x, int y, int z) -> uint32_t {
        int wrappedX = (x % (int)_config.grid_x + (int)_config.grid_x) % (int)_config.grid_x;
        int wrappedY = (y % (int)_config.grid_y + (int)_config.grid_y) % (int)_config.grid_y;
        int wrappedZ = (z % (int)_config.grid_z + (int)_config.grid_z) % (int)_config.grid_z;
        return (uint32_t)wrappedX * _config.grid_y * _config.grid_z +
            (uint32_t)wrappedY * _config.grid_z + (uint32_t)wrappedZ;
    };
    auto gasVelocity = [&](uint32_t cell, uint32_t axis) -> float {
        return density[cell] > 0.0f ? momentum[cell * 3u + axis] / density[cell] : 0.0f;
    };
    int centerX = (int)centroid[0];
    int centerY = (int)centroid[1];
    int centerZ = (int)centroid[2];
    uint32_t minusX = cellIndex(centerX - 1, centerY, centerZ);
    uint32_t plusX = cellIndex(centerX + 1, centerY, centerZ);
    uint32_t minusY = cellIndex(centerX, centerY - 1, centerZ);
    uint32_t plusY = cellIndex(centerX, centerY + 1, centerZ);
    uint32_t minusZ = cellIndex(centerX, centerY, centerZ - 1);
    uint32_t plusZ = cellIndex(centerX, centerY, centerZ + 1);
    float inverseCentral = 0.5f / _config.spacing;
    const float gammaMinusOne = 0.4f;
    float pressureGradX = gammaMinusOne * (energy[plusX] - energy[minusX]) * inverseCentral;
    float pressureGradY = gammaMinusOne * (energy[plusY] - energy[minusY]) * inverseCentral;
    float pressureGradZ = gammaMinusOne * (energy[plusZ] - energy[minusZ]) * inverseCentral;
    float divergence =
        (gasVelocity(plusX, 0u) - gasVelocity(minusX, 0u)) * inverseCentral +
        (gasVelocity(plusY, 1u) - gasVelocity(minusY, 1u)) * inverseCentral +
        (gasVelocity(plusZ, 2u) - gasVelocity(minusZ, 2u)) * inverseCentral;
    float coherenceMag2 = 0.0f;

    for (uint32_t mode = 0u; mode < _modeCount; mode++) {
        coherenceMag2 += waveReal[mode] * waveReal[mode] +
            waveImaginary[mode] * waveImaginary[mode];
    }

    coherenceMag2 /= (float)_modeCount;
    float guidanceSpeed = 0.0f;

    for (uint32_t particle = 0u; particle < _particleCount; particle++) {
        uint32_t base = particle * 3u;
        guidanceSpeed += std::sqrt(
            velocity[base] * velocity[base] +
            velocity[base + 1u] * velocity[base + 1u] +
            velocity[base + 2u] * velocity[base + 2u]
        );
    }

    guidanceSpeed /= (float)_particleCount;
    *reading = FluidReading{
        pressureGradX,
        pressureGradY,
        pressureGradZ,
        std::sqrt(
            pressureGradX * pressureGradX +
            pressureGradY * pressureGradY +
            pressureGradZ * pressureGradZ
        ),
        divergence,
        coherenceMag2,
        guidanceSpeed,
        divergence != 0.0f ? 1.0f / std::fabs(divergence) : 0.0f,
    };
    float *values = (float *)reading;

    for (uint32_t index = 0u; index < sizeof(FluidReading) / sizeof(float); index++) {
        if (!std::isfinite(values[index])) {
            if (error != nil) {
                *error = @"resident fluid reading is not finite";
            }

            return NO;
        }
    }

    return YES;
}

- (BOOL)readProjection:(float *)densityProjection
             coherence:(float *)coherenceProjection
              guidanceX:(float *)guidanceX
              guidanceZ:(float *)guidanceZ
                  count:(uint32_t)count
                  error:(NSString **)error {
    uint32_t expected = _config.grid_x * _config.grid_z;

    if (_particleCount == 0u || !_waveInitialized) {
        if (error != nil) {
            *error = @"resident fluid projection is unavailable before the first step";
        }

        return NO;
    }

    if (densityProjection == nullptr || coherenceProjection == nullptr ||
        guidanceX == nullptr || guidanceZ == nullptr || count != expected) {
        if (error != nil) {
            *error = @"fluid projection buffers do not match the X-Z lattice";
        }

        return NO;
    }

    float *density = (float *)_density.contents;
    float *waveReal = (float *)_spatialPsiReal.contents;
    float *waveImaginary = (float *)_spatialPsiImaginary.contents;
    auto cellIndex = [&](int x, int y, int z) -> uint32_t {
        int wrappedX = (x % (int)_config.grid_x + (int)_config.grid_x) % (int)_config.grid_x;
        int wrappedY = (y % (int)_config.grid_y + (int)_config.grid_y) % (int)_config.grid_y;
        int wrappedZ = (z % (int)_config.grid_z + (int)_config.grid_z) % (int)_config.grid_z;
        return (uint32_t)wrappedX * _config.grid_y * _config.grid_z +
            (uint32_t)wrappedY * _config.grid_z + (uint32_t)wrappedZ;
    };
    float inverseCentral = 0.5f / _config.spacing;

    for (uint32_t z = 0u; z < _config.grid_z; z++) {
        for (uint32_t x = 0u; x < _config.grid_x; x++) {
            uint32_t destination = x + z * _config.grid_x;
            float maximumDensity = 0.0f;
            float maximumCoherence = 0.0f;
            uint32_t waveY = 0u;

            for (uint32_t y = 0u; y < _config.grid_y; y++) {
                uint32_t cell = cellIndex((int)x, (int)y, (int)z);
                maximumDensity = std::max(maximumDensity, density[cell]);
                float magnitude = waveReal[cell] * waveReal[cell] +
                    waveImaginary[cell] * waveImaginary[cell];

                if (magnitude > maximumCoherence) {
                    maximumCoherence = magnitude;
                    waveY = y;
                }
            }

            densityProjection[destination] = maximumDensity;
            coherenceProjection[destination] = maximumCoherence;
            guidanceX[destination] = 0.0f;
            guidanceZ[destination] = 0.0f;

            if (maximumCoherence == 0.0f) {
                continue;
            }

            uint32_t center = cellIndex((int)x, (int)waveY, (int)z);
            uint32_t minusX = cellIndex((int)x - 1, (int)waveY, (int)z);
            uint32_t plusX = cellIndex((int)x + 1, (int)waveY, (int)z);
            uint32_t minusZ = cellIndex((int)x, (int)waveY, (int)z - 1);
            uint32_t plusZ = cellIndex((int)x, (int)waveY, (int)z + 1);
            float gradientRealX = (waveReal[plusX] - waveReal[minusX]) * inverseCentral;
            float gradientImaginaryX = (waveImaginary[plusX] - waveImaginary[minusX]) * inverseCentral;
            float gradientRealZ = (waveReal[plusZ] - waveReal[minusZ]) * inverseCentral;
            float gradientImaginaryZ = (waveImaginary[plusZ] - waveImaginary[minusZ]) * inverseCentral;
            guidanceX[destination] = (
                waveReal[center] * gradientImaginaryX -
                waveImaginary[center] * gradientRealX
            ) / maximumCoherence;
            guidanceZ[destination] = (
                waveReal[center] * gradientImaginaryZ -
                waveImaginary[center] * gradientRealZ
            ) / maximumCoherence;
        }
    }

    for (uint32_t index = 0u; index < expected; index++) {
        if (!std::isfinite(densityProjection[index]) ||
            !std::isfinite(coherenceProjection[index]) ||
            !std::isfinite(guidanceX[index]) ||
            !std::isfinite(guidanceZ[index])) {
            if (error != nil) {
                *error = @"fluid projection is not finite";
            }

            return NO;
        }
    }

    return YES;
}

- (BOOL)readDisplay:(uint8_t *)rgba
              count:(uint32_t)byteCount
              stats:(FluidDisplayStats *)stats
              error:(NSString **)error {
    uint32_t width = _displayWidth;
    uint32_t height = _displayHeight;
    uint32_t expected = (uint32_t)_displayBytes;

    if (_particleCount == 0u || _waveInitialized == NO) {
        if (error != nil) *error = @"resident fluid display is unavailable before the first step";
        return NO;
    }

    if (rgba == nullptr || stats == nullptr || byteCount != expected || _displayRho == nil || _displayPsi == nil || _displayGuidanceX == nil || _displayGuidanceZ == nil || _displayExtents == nil || _displayGlow == nil || _displayCore == nil || _displayRGBA == nil) {
        if (error != nil) *error = @"fluid display buffers do not match derived display geometry";
        return NO;
    }

    id<MTLComputePipelineState> project = [self pipeline:@"display_project_xz" error:error];
    id<MTLComputePipelineState> particleStats = [self pipeline:@"display_particle_stats" error:error];
    id<MTLComputePipelineState> splat = [self pipeline:@"display_splat_particles" error:error];
    id<MTLComputePipelineState> resolve = [self pipeline:@"display_resolve" error:error];
    if (project == nil || particleStats == nil || splat == nil || resolve == nil) return NO;

    std::memset(_displayExtents.contents, 0, 12u * sizeof(uint32_t));
    std::memset(_displayGlow.contents, 0, _displayPixels * sizeof(uint32_t));
    std::memset(_displayCore.contents, 0, _displayPixels * sizeof(uint32_t));

    DisplayParams params{_config.grid_x, _config.grid_y, _config.grid_z, width, height, _config.spacing, 1.0f / _config.spacing, _particleCount};
    uint32_t cells = _config.grid_x * _config.grid_z;
    id<MTLCommandBuffer> command = nil;
    id<MTLComputeCommandEncoder> encoder = [self encoder:project command:&command];
    [encoder setBuffer:_density offset:0 atIndex:0]; [encoder setBuffer:_spatialPsiReal offset:0 atIndex:1]; [encoder setBuffer:_spatialPsiImaginary offset:0 atIndex:2]; [encoder setBuffer:_displayRho offset:0 atIndex:3]; [encoder setBuffer:_displayPsi offset:0 atIndex:4]; [encoder setBuffer:_displayGuidanceX offset:0 atIndex:5]; [encoder setBuffer:_displayGuidanceZ offset:0 atIndex:6]; [encoder setBuffer:_displayExtents offset:0 atIndex:7]; [encoder setBytes:&params length:sizeof(params) atIndex:8]; [self dispatch:encoder count:cells pipeline:project]; [encoder endEncoding];

    encoder = [command computeCommandEncoder]; [encoder setComputePipelineState:particleStats]; [encoder setBuffer:_energy offset:0 atIndex:0]; [encoder setBuffer:_displayExtents offset:0 atIndex:1]; [encoder setBytes:&params length:sizeof(params) atIndex:2]; [self dispatch:encoder count:_particleCount pipeline:particleStats]; [encoder endEncoding];

    encoder = [command computeCommandEncoder]; [encoder setComputePipelineState:splat]; [encoder setBuffer:_position offset:0 atIndex:0]; [encoder setBuffer:_energy offset:0 atIndex:1]; [encoder setBuffer:_displayExtents offset:0 atIndex:2]; [encoder setBuffer:_displayGlow offset:0 atIndex:3]; [encoder setBuffer:_displayCore offset:0 atIndex:4]; [encoder setBytes:&params length:sizeof(params) atIndex:5]; [self dispatch:encoder count:_particleCount pipeline:splat]; [encoder endEncoding];

    encoder = [command computeCommandEncoder]; [encoder setComputePipelineState:resolve]; [encoder setBuffer:_displayRho offset:0 atIndex:0]; [encoder setBuffer:_displayPsi offset:0 atIndex:1]; [encoder setBuffer:_displayGuidanceX offset:0 atIndex:2]; [encoder setBuffer:_displayGuidanceZ offset:0 atIndex:3]; [encoder setBuffer:_displayExtents offset:0 atIndex:4]; [encoder setBuffer:_displayGlow offset:0 atIndex:5]; [encoder setBuffer:_displayCore offset:0 atIndex:6]; [encoder setBuffer:_displayRGBA offset:0 atIndex:7]; [encoder setBytes:&params length:sizeof(params) atIndex:8]; [self dispatch:encoder count:(uint32_t)_displayPixels pipeline:resolve];
    if ([self finish:encoder command:command error:error] == NO) return NO;

    std::memcpy(rgba, _displayRGBA.contents, _displayBytes);
    uint32_t *extents = (uint32_t *)_displayExtents.contents;
    float rhoMax = fluid_debug_float(extents[0]), psiMax = fluid_debug_float(extents[1]), guidanceMax = fluid_debug_float(extents[2]);
    if (std::isfinite(rhoMax) == false || rhoMax < 0.0f) rhoMax = 0.0f;
    if (std::isfinite(psiMax) == false || psiMax < 0.0f) psiMax = 0.0f;
    if (std::isfinite(guidanceMax) == false || guidanceMax < 0.0f) guidanceMax = 0.0f;
    *stats = FluidDisplayStats{width, height, extents[4], extents[5], rhoMax, psiMax, extents[6], guidanceMax};
    return YES;
}
@end

extern "C" void *fluid_domain_new(
    const FluidConfig *config,
    const void *metallibBytes,
    size_t metallibLength,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        NSString *error = nil;
        SensoriumFluidDomain *domain = [[SensoriumFluidDomain alloc]
            initWithConfig:config
            metallibBytes:metallibBytes
            length:metallibLength
            error:&error
        ];

        if (domain == nil) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid domain creation failed");
            return nullptr;
        }

        return (__bridge_retained void *)domain;
    }
}

extern "C" void fluid_domain_free(void *handle) {
    if (handle != nullptr) {
        CFBridgingRelease(handle);
    }
}

extern "C" int fluid_domain_step(
    void *handle,
    FluidParticle *particles,
    uint32_t particleCount,
    FluidDiagnostics *diagnostics,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;
        BOOL success = [domain
            stepParticles:particles
            count:particleCount
            diagnostics:diagnostics
            error:&error
        ];

        if (!success) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid domain step failed");
            return 0;
        }

        return 1;
    }
}

extern "C" int fluid_domain_append(
    void *handle,
    const FluidParticle *particles,
    const uint32_t *contentIDs,
    uint32_t particleCount,
    uint32_t *startOut,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;
        BOOL success = [domain
            appendParticles:particles
            contentIDs:contentIDs
            count:particleCount
            start:startOut
            error:&error
        ];

        if (!success) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid domain append failed");
            return 0;
        }

        return 1;
    }
}

extern "C" int fluid_domain_advance(
    void *handle,
    FluidDiagnostics *diagnostics,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;

        if (![domain advanceResident:diagnostics error:&error]) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid domain advance failed");
            return 0;
        }

        return 1;
    }
}

extern "C" uint32_t fluid_domain_particle_count(void *handle) {
    if (handle == nullptr) {
        return 0u;
    }

    SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
    return domain->_particleCount;
}

extern "C" int fluid_domain_retain(
    void *handle,
    const uint32_t *indices,
    uint32_t count,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;

        if (![domain retainParticles:indices count:count error:&error]) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid particle retain failed");
            return 0;
        }

        return 1;
    }
}

extern "C" int fluid_domain_read_particles(
    void *handle,
    FluidParticle *particles,
    uint32_t start,
    uint32_t count,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;

        if (![domain readParticles:particles start:start count:count error:&error]) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid particle read failed");
            return 0;
        }

        return 1;
    }
}

extern "C" int fluid_domain_read_spatial_ids(
    void *handle,
    uint32_t *ids,
    uint32_t start,
    uint32_t count,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;

        if (![domain readSpatialIDs:ids start:start count:count error:&error]) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"spatial ID read failed");
            return 0;
        }

        return 1;
    }
}

extern "C" uint32_t fluid_domain_mode_count(void *handle) {
    if (handle == nullptr) {
        return 0u;
    }

    SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
    return domain->_modeCount;
}

extern "C" int fluid_domain_read_wave(
    void *handle,
    FluidWaveMode *modes,
    uint32_t modeCount,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;
        BOOL success = [domain readWave:modes count:modeCount error:&error];

        if (!success) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"wave read failed");
            return 0;
        }

        return 1;
    }
}

extern "C" int fluid_domain_read(
    void *handle,
    FluidReading *reading,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;

        if (![domain read:reading error:&error]) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid reading failed");
            return 0;
        }

        return 1;
    }
}

extern "C" int fluid_domain_read_projection(
    void *handle,
    float *density,
    float *coherence,
    float *guidanceX,
    float *guidanceZ,
    uint32_t projectionCount,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;
        BOOL success = [domain
            readProjection:density
            coherence:coherence
            guidanceX:guidanceX
            guidanceZ:guidanceZ
            count:projectionCount
            error:&error
        ];

        if (!success) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid projection failed");
            return 0;
        }

        return 1;
    }
}

extern "C" int fluid_domain_read_display(
    void *handle,
    uint8_t *rgba,
    uint32_t byteCount,
    FluidDisplayStats *stats,
    char *errorOutput,
    int errorCapacity
) {
    @autoreleasepool {
        if (handle == nullptr) {
            fluid_write_error(errorOutput, errorCapacity, @"fluid domain handle is required");
            return 0;
        }

        SensoriumFluidDomain *domain = (__bridge SensoriumFluidDomain *)handle;
        NSString *error = nil;

        if (![domain readDisplay:rgba count:byteCount stats:stats error:&error]) {
            fluid_write_error(errorOutput, errorCapacity, error ?: @"fluid display failed");
            return 0;
        }

        return 1;
    }
}
