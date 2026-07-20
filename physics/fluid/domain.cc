//go:build darwin && cgo

#import <Foundation/Foundation.h>
#import <Metal/Metal.h>

#include "bridge.h"

#include <algorithm>
#include <cfloat>
#include <cmath>
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
    uint32_t _modeCount;
    uint32_t _randomSeed;
    BOOL _waveInitialized;
    float _omegaMinimum;
    float _omegaSpacing;
    float _gateMinimum;
    float _gateMaximum;
    float _spatialSigma;
    std::vector<float> _previousAmplitude;

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

    id<MTLBuffer> _position;
    id<MTLBuffer> _velocity;
    id<MTLBuffer> _mass;
    id<MTLBuffer> _heat;
    id<MTLBuffer> _energy;
    id<MTLBuffer> _phase;
    id<MTLBuffer> _omega;
    id<MTLBuffer> _amplitude;
    id<MTLBuffer> _positionOutput;
    id<MTLBuffer> _velocityOutput;
    id<MTLBuffer> _heatOutput;

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
}

- (instancetype)initWithConfig:(const FluidConfig *)config
                metallibBytes:(const void *)metallibBytes
                        length:(size_t)length
                           error:(NSString **)error;
- (BOOL)stepParticles:(FluidParticle *)particles
                 count:(uint32_t)count
           diagnostics:(FluidDiagnostics *)diagnostics
                 error:(NSString **)error;
- (BOOL)readWave:(FluidWaveMode *)modes count:(uint32_t)count error:(NSString **)error;
- (BOOL)read:(FluidReading *)reading error:(NSString **)error;
- (BOOL)readProjection:(float *)density
             coherence:(float *)coherence
              guidanceX:(float *)guidanceX
              guidanceZ:(float *)guidanceZ
                  count:(uint32_t)count
                  error:(NSString **)error;

@end

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
    _modeCount = fluid_mode_count(*config);
    _randomSeed = 1u;
    _device = MTLCreateSystemDefaultDevice();

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

    uint32_t *binStarts = (uint32_t *)_binStarts.contents;
    uint32_t *binned = (uint32_t *)_binnedIndex.contents;

    for (uint32_t index = 0; index < _modeCount; index++) {
        binStarts[index] = index;
        binned[index] = index;
    }

    binStarts[_modeCount] = _modeCount;
    return self;
}

- (id<MTLBuffer>)buffer:(size_t)length {
    return [_device newBufferWithLength:length options:MTLResourceStorageModeShared];
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

- (BOOL)allocateParticles:(uint32_t)count error:(NSString **)error {
    if (_particleCount == count) {
        return YES;
    }

    // A step supplies the complete current population. Particle storage follows
    // that population while the gas and omega fields remain resident, allowing a
    // streaming caller to remove observations without accumulating ghost mass.
    _particleCount = count;
    size_t scalarBytes = (size_t)count * sizeof(float);
    size_t vectorBytes = scalarBytes * 3u;
    _position = [self buffer:vectorBytes];
    _velocity = [self buffer:vectorBytes];
    _mass = [self buffer:scalarBytes];
    _heat = [self buffer:scalarBytes];
    _energy = [self buffer:scalarBytes];
    _phase = [self buffer:scalarBytes];
    _omega = [self buffer:scalarBytes];
    _amplitude = [self buffer:scalarBytes];
    _positionOutput = [self buffer:vectorBytes];
    _velocityOutput = [self buffer:vectorBytes];
    _heatOutput = [self buffer:scalarBytes];
    _cellIndex = [self buffer:(size_t)count * sizeof(uint32_t)];
    _sortedOriginalIndex = [self buffer:(size_t)count * sizeof(uint32_t)];
    _sortedPosition = [self buffer:vectorBytes];
    _sortedVelocity = [self buffer:vectorBytes];
    _sortedMass = [self buffer:scalarBytes];
    _sortedHeat = [self buffer:scalarBytes];
    _sortedEnergy = [self buffer:scalarBytes];
    return YES;
}

- (void)loadParticles:(FluidParticle *)particles count:(uint32_t)count {
    float *position = (float *)_position.contents;
    float *velocity = (float *)_velocity.contents;
    float *mass = (float *)_mass.contents;
    float *heat = (float *)_heat.contents;
    float *energy = (float *)_energy.contents;
    float *phase = (float *)_phase.contents;
    float *omega = (float *)_omega.contents;
    float domainX = _config.grid_x * _config.spacing;
    float domainY = _config.grid_y * _config.spacing;
    float domainZ = _config.grid_z * _config.spacing;

    for (uint32_t index = 0; index < count; index++) {
        uint32_t base = index * 3u;
        position[base] = fluid_periodic(particles[index].position_x, domainX);
        position[base + 1u] = fluid_periodic(particles[index].position_y, domainY);
        position[base + 2u] = fluid_periodic(particles[index].position_z, domainZ);
        velocity[base] = particles[index].velocity_x;
        velocity[base + 1u] = particles[index].velocity_y;
        velocity[base + 2u] = particles[index].velocity_z;
        mass[index] = particles[index].mass;
        heat[index] = particles[index].heat;
        energy[index] = particles[index].energy;
        phase[index] = particles[index].phase;
        omega[index] = particles[index].omega;
    }
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
    [self seedAnchors];
    _waveInitialized = YES;
}

- (void)seedAnchors {
    uint32_t *anchorIndex = (uint32_t *)_anchorIndex.contents;
    float *anchorWeight = (float *)_anchorWeight.contents;
    float *omega = (float *)_omega.contents;
    float *energy = (float *)_energy.contents;
    std::fill(anchorIndex, anchorIndex + (size_t)_modeCount * FluidAnchorSlots, UINT32_MAX);
    std::fill(anchorWeight, anchorWeight + (size_t)_modeCount * FluidAnchorSlots, 0.0f);

    if (!std::isfinite(_omegaSpacing) || _omegaSpacing <= 0.0f) {
        return;
    }

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
    float massTotal = 0.0f;

    for (uint32_t index = 0; index < _particleCount; index++) {
        massTotal += ((float *)_mass.contents)[index];
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
    return GPEParams{
        delta,
        1.0f,
        1.0f,
        -1.0f,
        1.0f / (float)_modeCount,
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
    // Newtonian gravity is deliberately disabled: spatial transport is applied
    // by the pilot-wave probability current after the omega field advances.
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
        0.0f,
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

    float *heat = (float *)_heatOutput.contents;

    for (uint32_t index = 0; index < _particleCount; index++) {
        if (!std::isfinite(heat[index]) || heat[index] < 0.0f) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"PIC gather produced invalid heat at particle %u", index];
            }

            return NO;
        }
    }

    std::memcpy(_position.contents, _positionOutput.contents, _position.length);
    std::memcpy(_velocity.contents, _velocityOutput.contents, _velocity.length);
    std::memcpy(_heat.contents, _heatOutput.contents, _heat.length);
    return YES;
}

- (BOOL)advancePilotWave:(float)delta
             diagnostics:(FluidDiagnostics *)diagnostics
                   error:(NSString **)error {
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
    [encoder endEncoding];

    // The restored pilot-wave model uses hbar_eff=1. Particle validation already
    // requires positive inertial mass, while exact spatial vacuum has a defined
    // zero-guidance path, so no denominator or mass fallback is introduced here.
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
        0.0f,
        0.0f,
    };
    encoder = [command computeCommandEncoder];
    [encoder setComputePipelineState:advect];
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

    float *position = (float *)_positionOutput.contents;
    float *velocity = (float *)_velocityOutput.contents;
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
    std::memcpy(_position.contents, _positionOutput.contents, _position.length);
    std::memcpy(_velocity.contents, _velocityOutput.contents, _velocity.length);
    return YES;
}

- (BOOL)planckExchange:(float)delta error:(NSString **)error {
    float *heat = (float *)_heat.contents;
    float *energy = (float *)_energy.contents;
    float *omega = (float *)_omega.contents;
    float *mass = (float *)_mass.contents;
    const float conductivity = 1.0e-4f * 1.4f / 0.71f;
    const float radius = 0.5f * _config.spacing;

    for (uint32_t index = 0; index < _particleCount; index++) {
        float temperature = heat[index] / mass[index];
        float ratio = omega[index] / std::max(temperature, 1.0e-20f);
        float equilibrium = 0.0f;

        if (ratio < 1.0e-4f) {
            equilibrium = temperature;
        } else if (ratio > 50.0f) {
            equilibrium = omega[index] * std::exp(-ratio);
        } else {
            equilibrium = omega[index] / std::expm1(std::min(ratio, 80.0f));
        }

        if (!std::isfinite(equilibrium) || equilibrium < 0.0f) {
            equilibrium = 0.0f;
        }

        float relaxation = mass[index] / (4.0f * (float)M_PI * conductivity * radius);
        float alpha = 1.0f - std::exp(-delta / relaxation);
        float exchanged = alpha * (equilibrium - energy[index]);
        exchanged = std::min(exchanged, heat[index]);
        exchanged = std::max(exchanged, -energy[index]);
        energy[index] += exchanged;
        heat[index] -= exchanged;

        if (!std::isfinite(energy[index]) || !std::isfinite(heat[index]) ||
            energy[index] < 0.0f || heat[index] < 0.0f) {
            if (error != nil) {
                *error = [NSString stringWithFormat:@"Planck exchange failed at particle %u", index];
            }

            return NO;
        }
    }

    return YES;
}

- (void)deriveSpatialSigma {
    float *heat = (float *)_heat.contents;
    float *mass = (float *)_mass.contents;
    float meanTemperature = 0.0f;
    float meanMass = 0.0f;

    for (uint32_t index = 0; index < _particleCount; index++) {
        meanTemperature += heat[index] / mass[index];
        meanMass += mass[index];
    }

    meanTemperature /= (float)_particleCount;
    meanMass /= (float)_particleCount;
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

- (BOOL)advanceWave:(float)delta diagnostics:(FluidDiagnostics *)diagnostics error:(NSString **)error {
    [self deriveSpatialSigma];
    float *energy = (float *)_energy.contents;
    float *amplitude = (float *)_amplitude.contents;

    for (uint32_t index = 0; index < _particleCount; index++) {
        amplitude[index] = std::sqrt(std::max(energy[index], 1.0e-8f));
    }

    std::memset(_modeAccumulators.contents, 0, _modeAccumulators.length);
    _randomSeed++;
    SpectralModeParams spectral = [self spectralParams:delta];
    spectral.coupling_scale = 1.0f / std::sqrt((float)_modeCount);
    spectral.rng_seed = _randomSeed;
    GPEParams gpe = [self gpeParams:delta];
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
    [encoder endEncoding];
    encoder = [command computeCommandEncoder];
    [encoder setComputePipelineState:phasePipeline];
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

    float *phase = (float *)_phase.contents;
    float *heat = (float *)_heat.contents;

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

    for (uint32_t mode = 0; mode < _modeCount; mode++) {
        float current = std::hypot(real[mode], imaginary[mode]);
        float difference = hasPreviousAmplitude ? current - _previousAmplitude[mode] : 0.0f;
        squared += current * current;
        deltaSquared += difference * difference;
        _previousAmplitude[mode] = current;
    }

    diagnostics->psi_rms = std::sqrt(squared / (float)_modeCount);
    diagnostics->psi_delta_rms = std::sqrt(deltaSquared / (float)_modeCount);
    return YES;
}

- (void)storeParticles:(FluidParticle *)particles count:(uint32_t)count {
    float *position = (float *)_position.contents;
    float *velocity = (float *)_velocity.contents;
    float *mass = (float *)_mass.contents;
    float *heat = (float *)_heat.contents;
    float *energy = (float *)_energy.contents;
    float *phase = (float *)_phase.contents;
    float *omega = (float *)_omega.contents;

    for (uint32_t index = 0; index < count; index++) {
        uint32_t base = index * 3u;
        particles[index] = FluidParticle{
            position[base], position[base + 1u], position[base + 2u],
            velocity[base], velocity[base + 1u], velocity[base + 2u],
            mass[index], heat[index], energy[index], phase[index], omega[index],
        };
    }
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

    if (![self allocateParticles:count error:error]) {
        return NO;
    }

    [self loadParticles:particles count:count];

    if (!_waveInitialized) {
        [self initializeWave];
    }

    *diagnostics = FluidDiagnostics{};

    if (![self scatter:error] ||
        ![self advanceGas:diagnostics error:error] ||
        ![self gather:diagnostics->delta_used error:error] ||
        ![self planckExchange:diagnostics->delta_used error:error] ||
        ![self advanceWave:_config.spacing diagnostics:diagnostics error:error] ||
        ![self advancePilotWave:diagnostics->delta_used diagnostics:diagnostics error:error]) {
        return NO;
    }

    [self storeParticles:particles count:count];
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

    float *position = (float *)_position.contents;
    float *density = (float *)_density.contents;
    float *momentum = (float *)_momentum.contents;
    float *energy = (float *)_internalEnergy.contents;
    float *waveReal = (float *)_psiReal.contents;
    float *waveImaginary = (float *)_psiImaginary.contents;
    float *velocity = (float *)_velocity.contents;
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
