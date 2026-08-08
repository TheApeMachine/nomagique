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
	/* Newtonian G in ω-natural simulation units (CODATA→UnitSystem). */
	float gravity_g;
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

/*
fluid_domain_append packs one batch through Shared staging and blits it into
Private resident Metal storage without advancing physics. content_ids are
Sensorium content identities for inelastic merge. Evolved history grows by GPU
blit, not host re-upload of the full tape.
*/
int fluid_domain_append(
    void *handle,
    const FluidParticle *particles,
    const uint32_t *content_ids,
    uint32_t particle_count,
    uint32_t *start_out,
    char *error_out,
    int error_capacity
);

/*
fluid_domain_advance steps the resident particle population and fields without
re-uploading host state.
*/
int fluid_domain_advance(
    void *handle,
    FluidDiagnostics *diagnostics,
    char *error_out,
    int error_capacity
);

uint32_t fluid_domain_particle_count(void *handle);

/*
fluid_domain_retain keeps only the listed resident indices (unique, in range),
preserving content IDs and clamp state so streaming merge identities survive.
*/
int fluid_domain_retain(
    void *handle,
    const uint32_t *indices,
    uint32_t count,
    char *error_out,
    int error_capacity
);

int fluid_domain_read_particles(
    void *handle,
    FluidParticle *particles,
    uint32_t start,
    uint32_t count,
    char *error_out,
    int error_capacity
);

/*
fluid_domain_read_spatial_ids copies post-merge Morton spatial token IDs
((cell_morton << 8) | byte) for one resident range.
*/
int fluid_domain_read_spatial_ids(
    void *handle,
    uint32_t *ids,
    uint32_t start,
    uint32_t count,
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

/*
fluid_domain_read_fields copies the complete post-step Eulerian gas state and
spatial complex wave field. Scalar arrays contain grid_x*grid_y*grid_z values;
momentum contains three values per cell in XYZ order.
*/
int fluid_domain_read_fields(
    void *handle,
    float *density,
    float *momentum,
    float *internal_energy,
    float *wave_real,
    float *wave_imaginary,
    uint32_t cell_count,
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

/*
FluidDisplayStats describes one GPU-composited XZ RGBA8 frame: lattice size and
field occupancy/maxima used by the UI meta panel.
*/
typedef struct FluidDisplayStats {
    uint32_t width;
    uint32_t height;
    uint32_t rho_occupied;
    uint32_t psi_occupied;
    float rho_max;
    float psi_max;
    uint32_t guidance_occupied;
    float guidance_max;
} FluidDisplayStats;

/*
fluid_domain_read_display runs the Metal display pass (project, particle stats,
splat, resolve) and copies the Shared RGBA8 buffer plus stats to the host.
*/
int fluid_domain_read_display(
    void *handle,
    uint8_t *rgba,
    uint32_t byte_count,
    FluidDisplayStats *stats,
    char *error_out,
    int error_capacity
);

#ifdef __cplusplus
}
#endif
