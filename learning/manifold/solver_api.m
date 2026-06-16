#import "solver_private.h"

void *manifold_solver_create(
    const ResonanceConfig *config,
    const uint32_t *arch,
    uint32_t arch_len,
    uint32_t target_dim,
    const void *metallib_bytes,
    size_t metallib_length,
    char *err_out,
    int err_cap
) {
    @autoreleasepool {
        NSString *error = nil;
        ResonanceSolver *solver = [[ResonanceSolver alloc] initWithConfig:config
                                                                     arch:arch
                                                                  archLen:arch_len
                                                                targetDim:target_dim
                                                            metallibBytes:metallib_bytes
                                                           metallibLength:metallib_length
                                                                    error:&error];

        if (solver == nil) {
            resonance_write_error(err_out, err_cap, error ?: @"failed to create resonance solver");
            return NULL;
        }

        return (void *)CFBridgingRetain(solver);
    }
}

void manifold_solver_destroy(void *handle) {
    if (handle == NULL) {
        return;
    }

    @autoreleasepool {
        ResonanceSolver *solver = (ResonanceSolver *)CFBridgingRelease(handle);
        solver = nil;
    }
}

static ResonanceSolver *solver_from(void *handle) {
    return (__bridge ResonanceSolver *)handle;
}

int manifold_solver_seed_weights(
    void *handle,
    const float *w_flat, size_t w_len,
    const float *r_flat, size_t r_len,
    const float *a_flat, size_t a_len,
    const float *v_flat, size_t v_len,
    char *err_out, int err_cap
) {
    if (handle == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    @autoreleasepool {
        NSString *error = nil;
        BOOL ok = [solver_from(handle) seedW:w_flat len:w_len
                                          r:r_flat len:r_len
                                          a:a_flat len:a_len
                                          v:v_flat len:v_len
                                      error:&error];
        if (!ok) {
            resonance_write_error(err_out, err_cap, error ?: @"seed failed");
            return 1;
        }
    }

    return 0;
}

int manifold_solver_reset_state(void *handle, uint32_t reset_precision, char *err_out, int err_cap) {
    if (handle == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    @autoreleasepool {
        [solver_from(handle) resetState:(reset_precision != 0u)];
    }

    return 0;
}

int manifold_solver_settle(
    void *handle,
    const float *input,
    uint32_t input_len,
    uint32_t advance_temporal,
    char *err_out,
    int err_cap
) {
    if (handle == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    ResonanceSolver *solver = solver_from(handle);

    if (input_len != solver.arch[0]) {
        resonance_write_error(err_out, err_cap, @"input dimension mismatch");
        return 1;
    }

    @autoreleasepool {
        [solver settleInput:input advanceTemporal:(advance_temporal != 0u)];
    }

    return 0;
}

int manifold_solver_learn(
    void *handle,
    const float *target,
    uint32_t target_len,
    char *err_out,
    int err_cap
) {
    if (handle == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    ResonanceSolver *solver = solver_from(handle);
    BOOL hasTarget = (target != NULL && target_len == solver.targetDim && solver.targetDim > 0u);

    @autoreleasepool {
        [solver learnTarget:target hasTarget:hasTarget];
    }

    return 0;
}

int manifold_solver_energy(void *handle, float *out, char *err_out, int err_cap) {
    if (handle == NULL || out == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    @autoreleasepool {
        *out = [solver_from(handle) energy];
    }

    return 0;
}

int manifold_solver_reconstruction_error(void *handle, float *out, char *err_out, int err_cap) {
    if (handle == NULL || out == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    @autoreleasepool {
        *out = [solver_from(handle) reconstructionError];
    }

    return 0;
}

int manifold_solver_read_latent(
    void *handle,
    float *out,
    uint32_t out_len,
    char *err_out,
    int err_cap
) {
    if (handle == NULL || out == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    @autoreleasepool {
        [solver_from(handle) readLatent:out length:out_len];
    }

    return 0;
}

int manifold_solver_read_weights(
    void *handle,
    float *w_flat, size_t w_len,
    float *r_flat, size_t r_len,
    float *a_flat, size_t a_len,
    float *v_flat, size_t v_len,
    char *err_out, int err_cap
) {
    (void)w_len; (void)r_len; (void)a_len; (void)v_len;

    if (handle == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }

    @autoreleasepool {
        [solver_from(handle) readWeightsW:w_flat r:r_flat a:a_flat v:v_flat];
    }

    return 0;
}
