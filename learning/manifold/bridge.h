#pragma once

#include <stdint.h>
#include <stddef.h>

/*
ResonanceConfig is the float32 mirror of learning.ResonanceConfig handed to the
GPU solver at creation time.
*/
typedef struct ResonanceConfig {
    uint32_t max_inference_steps;
    uint32_t min_inference_steps;
    float lr_state;
    float early_stop_tol;
    uint32_t early_stop_patience;
    uint32_t monotone_state_steps;
    uint32_t line_search_halvings;

    float lr_generative;
    float lr_temporal;
    float lr_recognition;

    float temporal_weight;
    float top_down_init_mix;

    uint32_t use_precision;
    float precision_beta;
    float precision_min;
    float precision_max;
    float precision_eps;

    float latent_decay;
    float sparsity;
    float weight_decay;
    float grad_clip;
    float state_clip;
} ResonanceConfig;

/*
manifold_solver_create builds the Metal resonance manifold.

arch points to layer dimensions (length arch_len, at least 2). target_dim may be
0 to disable the supervised task head. The weight buffers (W, R, A, V) are
optional seeds laid out flat; pass NULL to let the GPU initialize them with the
same He/Xavier scheme the gonum reference uses (seeded RNG handled host-side and
fed in via the seed buffers in practice). Returns NULL on error.
*/
void *manifold_solver_create(
    const ResonanceConfig *config,
    const uint32_t *arch,
    uint32_t arch_len,
    uint32_t target_dim,
    const void *metallib_bytes,
    size_t metallib_length,
    char *err_out,
    int err_cap
);

void manifold_solver_destroy(void *handle);

/*
manifold_solver_seed_weights uploads the initial weight matrices into the GPU
buffers. Each pointer is a flat float32 array in row-major order; w/r are
concatenations across links, a/v are single matrices (v may be NULL when
target_dim == 0). Lengths are validated against the architecture.
*/
int manifold_solver_seed_weights(
    void *handle,
    const float *w_flat,
    size_t w_len,
    const float *r_flat,
    size_t r_len,
    const float *a_flat,
    size_t a_len,
    const float *v_flat,
    size_t v_len,
    char *err_out,
    int err_cap
);

int manifold_solver_reset_state(void *handle, uint32_t reset_precision, char *err_out, int err_cap);

/*
manifold_solver_settle runs generative inference (no supervised contamination).
input has length arch[0]. advance_temporal mirrors the gonum flag.
*/
int manifold_solver_settle(
    void *handle,
    const float *input,
    uint32_t input_len,
    uint32_t advance_temporal,
    char *err_out,
    int err_cap
);

/*
manifold_solver_learn applies the weight updates for the current settled state.
target has length target_dim (may be 0/NULL to skip the task head).
*/
int manifold_solver_learn(
    void *handle,
    const float *target,
    uint32_t target_len,
    char *err_out,
    int err_cap
);

/*
manifold_solver_energy / reconstruction read back the scalar observables.
*/
int manifold_solver_energy(void *handle, float *out, char *err_out, int err_cap);
int manifold_solver_reconstruction_error(void *handle, float *out, char *err_out, int err_cap);

/*
manifold_solver_read_latent copies the top-layer latent state (length arch[-1]).
*/
int manifold_solver_read_latent(
    void *handle,
    float *out,
    uint32_t out_len,
    char *err_out,
    int err_cap
);

/*
manifold_solver_read_weights copies the current weight matrices back out, using
the same flat layout as manifold_solver_seed_weights. Pass NULL for buffers you
do not need.
*/
int manifold_solver_read_weights(
    void *handle,
    float *w_flat,
    size_t w_len,
    float *r_flat,
    size_t r_len,
    float *a_flat,
    size_t a_len,
    float *v_flat,
    size_t v_len,
    char *err_out,
    int err_cap
);
