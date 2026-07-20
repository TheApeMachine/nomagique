#pragma once

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct FluidConfig {
    uint32_t grid_x;
    uint32_t grid_y;
    uint32_t grid_z;
    float spacing;
    float max_delta;
	float omega_min;
	float omega_max;
} FluidConfig;

typedef struct FluidParticle {
    float position_x;
    float position_y;
    float position_z;
    float velocity_x;
    float velocity_y;
    float velocity_z;
    float mass;
    float heat;
    float energy;
    float phase;
    float omega;
} FluidParticle;

typedef struct FluidDiagnostics {
    float cfl_rate;
    float delta_adv;
    float delta_diffuse;
    float delta_derived;
    float delta_used;
    uint32_t halvings;
    float psi_rms;
    float psi_delta_rms;
    float guidance_rms;
} FluidDiagnostics;

typedef struct FluidWaveMode {
    float omega;
    float real;
    float imaginary;
    float linewidth;
} FluidWaveMode;

typedef struct FluidReading {
    float pressure_grad_x;
    float pressure_grad_y;
    float pressure_grad_z;
    float pressure_grad_norm;
    float divergence;
    float coherence_mag2;
    float guidance_speed;
    float viscosity_proxy;
} FluidReading;

void *fluid_domain_new(
    const FluidConfig *config,
    const void *metallib_bytes,
    size_t metallib_length,
    char *error_out,
    int error_capacity
);

void fluid_domain_free(void *handle);

int fluid_domain_step(
    void *handle,
    FluidParticle *particles,
    uint32_t particle_count,
    FluidDiagnostics *diagnostics,
    char *error_out,
    int error_capacity
);

uint32_t fluid_domain_mode_count(void *handle);

int fluid_domain_read_wave(
    void *handle,
    FluidWaveMode *modes,
    uint32_t mode_count,
    char *error_out,
    int error_capacity
);

int fluid_domain_read(
    void *handle,
    FluidReading *reading,
    char *error_out,
    int error_capacity
);

int fluid_domain_read_projection(
    void *handle,
    float *density,
    float *coherence,
    float *guidance_x,
    float *guidance_z,
    uint32_t projection_count,
    char *error_out,
    int error_capacity
);

#ifdef __cplusplus
}
#endif
