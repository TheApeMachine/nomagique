#pragma once

#include <stdint.h>
#include <stddef.h>

/*
ResonanceConfig is the float32 mirror of learning.ResonanceConfig handed to the
batched GPU solver at creation time. Every symbol in the batch shares the same
hyperparameters (they derive from alpha + architecture), but each symbol carries
its OWN weights and settles independently per column.
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
batch_solver_create builds a Metal resonance manifold that settles `batch`
symbols in lockstep, each with its own weights. arch (length arch_len >= 2) and
target_dim are shared. Returns NULL on error (message in err_out).
*/
void *batch_solver_create(
    const ResonanceConfig *config,
    const uint32_t *arch,
    uint32_t arch_len,
    uint32_t target_dim,
    uint32_t batch,
    const void *metallib_bytes,
    size_t metallib_length,
    char *err_out,
    int err_cap
);

void batch_solver_destroy(void *handle);

/*
batch_solver_seed_weights uploads weights for symbol `slot`. Flat row-major,
same per-symbol layout as the single-model layout (W and R concatenated across
links, A the top matrix, V the task head or NULL).
*/
int batch_solver_seed_weights(
    void *handle,
    uint32_t slot,
    const float *w_flat, size_t w_len,
    const float *r_flat, size_t r_len,
    const float *a_flat, size_t a_len,
    const float *v_flat, size_t v_len,
    char *err_out, int err_cap
);

/*
batch_solver_seed_all_weights uploads the same initial weights into every slot.
This avoids one cgo/Objective-C transition per slot during solver construction.
*/
int batch_solver_seed_all_weights(
    void *handle,
    const float *w_flat, size_t w_len,
    const float *r_flat, size_t r_len,
    const float *a_flat, size_t a_len,
    const float *v_flat, size_t v_len,
    char *err_out, int err_cap
);

/* Reset latent state (and optionally precision) for every symbol. */
int batch_solver_reset_state(void *handle, uint32_t reset_precision, char *err_out, int err_cap);

/*
batch_solver_set_input writes symbol `slot`'s input vector (length arch[0]) and,
if target_dim>0 and target!=NULL, its supervised target (length target_dim).
Inputs/targets are staged; settle/learn consume the whole batch at once.
*/
int batch_solver_set_input(
    void *handle,
    uint32_t slot,
    const float *input, uint32_t input_len,
    const float *target, uint32_t target_len,
    char *err_out, int err_cap
);

/*
batch_solver_set_inputs stages a full batch in one host transition. Inputs are
slot-major with input_stride floats per slot; targets use target_stride when
present. Passing NULL targets leaves the task buffer unchanged.
*/
int batch_solver_set_inputs(
    void *handle,
    const float *inputs, uint32_t input_len, uint32_t input_stride,
    const float *targets, uint32_t target_len, uint32_t target_stride,
    char *err_out, int err_cap
);

/*
batch_solver_settle settles all symbols (generative inference, no supervised
contamination), advancing the temporal prior when advance_temporal != 0. Each
column line-searches and early-stops independently; the call returns when every
column has converged or max steps is reached.
*/
int batch_solver_settle(void *handle, uint32_t advance_temporal, char *err_out, int err_cap);

/* batch_solver_learn applies per-symbol weight updates using staged targets. */
int batch_solver_learn(void *handle, char *err_out, int err_cap);

/* Read symbol `slot`'s top latent state (length arch[-1]). */
int batch_solver_read_latent(
    void *handle, uint32_t slot, float *out, uint32_t out_len, char *err_out, int err_cap);

/* Read symbol `slot`'s scalar energy / reconstruction error. */
int batch_solver_read_energy(void *handle, uint32_t slot, float *out, char *err_out, int err_cap);
int batch_solver_read_reconstruction(void *handle, uint32_t slot, float *out, char *err_out, int err_cap);

/*
batch_solver_read_outcomes refreshes post-learn energy and reconstruction for every
slot in one GPU pass, then copies latent/energy/surprise into caller buffers.
Latent is slot-major with topDim = arch[-1] floats per slot.
*/
int batch_solver_read_outcomes(
    void *handle,
    float *latent, uint32_t latent_len,
    float *energy, uint32_t energy_len,
    float *reconstruction, uint32_t reconstruction_len,
    char *err_out, int err_cap
);

/*
batch_solver_read_wire_layers copies settled per-layer state, prediction, and
error norms for one slot. Call after settle/read_outcomes so prediction buffers
reflect the current weights and state.
*/
int batch_solver_read_wire_layers(
    void *handle,
    uint32_t slot,
    float *state, uint32_t state_len,
    float *prediction, uint32_t prediction_len,
    float *error_norm, uint32_t error_norm_len,
    char *err_out, int err_cap
);

/* Read back symbol `slot`'s weights (same flat layout as seed). NULL to skip. */
int batch_solver_read_weights(
    void *handle, uint32_t slot,
    float *w_flat, size_t w_len,
    float *r_flat, size_t r_len,
    float *a_flat, size_t a_len,
    float *v_flat, size_t v_len,
    char *err_out, int err_cap
);


