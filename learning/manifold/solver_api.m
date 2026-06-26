//go:build darwin && cgo
// +build darwin,cgo

#import "solver_private.h"

void *batch_solver_create(
    const ResonanceConfig *config,
    const uint32_t *arch, uint32_t arch_len, uint32_t target_dim, uint32_t batch,
    const void *metallib_bytes, size_t metallib_length,
    char *err_out, int err_cap
) {
    @autoreleasepool {
        NSString *error = nil;
        BatchResonanceSolver *solver = [[BatchResonanceSolver alloc]
            initWithConfig:config arch:arch archLen:arch_len targetDim:target_dim batch:batch
             metallibBytes:metallib_bytes metallibLength:metallib_length error:&error];
        if (solver == nil) {
            resonance_write_error(err_out, err_cap, error ?: @"failed to create batch solver");
            return NULL;
        }
        return (void *)CFBridgingRetain(solver);
    }
}

void batch_solver_destroy(void *handle) {
    if (handle == NULL) return;
    @autoreleasepool {
        BatchResonanceSolver *solver = (BatchResonanceSolver *)CFBridgingRelease(handle);
        solver = nil;
    }
}

static BatchResonanceSolver *from(void *handle) { return (__bridge BatchResonanceSolver *)handle; }

int batch_solver_seed_weights(
    void *handle, uint32_t slot,
    const float *w, size_t wl, const float *r, size_t rl,
    const float *a, size_t al, const float *v, size_t vl,
    char *err_out, int err_cap
) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool {
        NSString *error = nil;
        if (![from(handle) seedSlot:slot w:w wLen:wl r:r rLen:rl a:a aLen:al v:v vLen:vl error:&error]) {
            resonance_write_error(err_out, err_cap, error ?: @"seed failed");
            return 1;
        }
    }
    return 0;
}

int batch_solver_seed_all_weights(
    void *handle,
    const float *w, size_t wl, const float *r, size_t rl,
    const float *a, size_t al, const float *v, size_t vl,
    char *err_out, int err_cap
) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool {
        NSString *error = nil;
        if (![from(handle) seedAllSlotsW:w wLen:wl r:r rLen:rl a:a aLen:al v:v vLen:vl error:&error]) {
            resonance_write_error(err_out, err_cap, error ?: @"seed all failed");
            return 1;
        }
    }
    return 0;
}

int batch_solver_reset_state(void *handle, uint32_t reset_precision, char *err_out, int err_cap) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool { [from(handle) resetState:(reset_precision != 0u)]; }
    return 0;
}

int batch_solver_set_input(
    void *handle, uint32_t slot,
    const float *input, uint32_t input_len,
    const float *target, uint32_t target_len,
    char *err_out, int err_cap
) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    BatchResonanceSolver *s = from(handle);
    if (slot >= s.batch) { resonance_write_error(err_out, err_cap, @"slot out of range"); return 1; }
    if (input == NULL || input_len != s.arch[0]) { resonance_write_error(err_out, err_cap, @"input dimension mismatch"); return 1; }
    BOOL hasTarget = NO;
    if (target != NULL || target_len != 0u) {
        if (s.targetDim == 0u || target == NULL || target_len != s.targetDim) {
            resonance_write_error(err_out, err_cap, @"target dimension mismatch");
            return 1;
        }
        hasTarget = YES;
    }
    @autoreleasepool { [s setInputSlot:slot input:input target:target hasTarget:hasTarget]; }
    return 0;
}

int batch_solver_set_inputs(
    void *handle,
    const float *inputs, uint32_t input_len, uint32_t input_stride,
    const float *targets, uint32_t target_len, uint32_t target_stride,
    char *err_out, int err_cap
) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    BatchResonanceSolver *s = from(handle);
    uint32_t expectedInputLen = s.batch * s.arch[0];
    if (inputs == NULL || input_stride != s.arch[0] || input_len != expectedInputLen) {
        resonance_write_error(err_out, err_cap, @"batch input dimension mismatch");
        return 1;
    }

    BOOL hasTarget = NO;
    if (targets != NULL || target_len != 0u || target_stride != 0u) {
        uint32_t expectedTargetLen = s.batch * s.targetDim;
        if (s.targetDim == 0u || targets == NULL || target_stride != s.targetDim || target_len != expectedTargetLen) {
            resonance_write_error(err_out, err_cap, @"batch target dimension mismatch");
            return 1;
        }
        hasTarget = YES;
    }

    @autoreleasepool {
        [s setInputBatch:inputs inputStride:input_stride
                  target:targets targetStride:target_stride hasTarget:hasTarget];
    }
    return 0;
}

int batch_solver_settle(void *handle, uint32_t advance_temporal, char *err_out, int err_cap) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool { [from(handle) settleAdvanceTemporal:(advance_temporal != 0u)]; }
    return 0;
}

int batch_solver_learn(void *handle, char *err_out, int err_cap) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool { [from(handle) learn]; }
    return 0;
}

int batch_solver_read_latent(void *handle, uint32_t slot, float *out, uint32_t out_len, char *err_out, int err_cap) {
    if (handle == NULL || out == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool { [from(handle) readLatentSlot:slot out:out length:out_len]; }
    return 0;
}

int batch_solver_read_energy(void *handle, uint32_t slot, float *out, char *err_out, int err_cap) {
    if (handle == NULL || out == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool { *out = [from(handle) energySlotCompute:slot]; }
    return 0;
}

int batch_solver_read_reconstruction(void *handle, uint32_t slot, float *out, char *err_out, int err_cap) {
    if (handle == NULL || out == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool { *out = [from(handle) reconstructionSlotCompute:slot]; }
    return 0;
}

int batch_solver_read_outcomes(
    void *handle,
    float *latent, uint32_t latent_len,
    float *energy, uint32_t energy_len,
    float *reconstruction, uint32_t reconstruction_len,
    char *err_out, int err_cap
) {
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool {
        NSString *error = nil;
        if (![from(handle) readOutcomesBatchLatent:latent
                                         latentLen:latent_len
                                            energy:energy
                                          energyLen:energy_len
                                    reconstruction:reconstruction
                                          reconLen:reconstruction_len
                                             error:&error]) {
            resonance_write_error(err_out, err_cap, error ?: @"read outcomes failed");
            return 1;
        }
    }
    return 0;
}

int batch_solver_read_wire_layers(
    void *handle,
    uint32_t slot,
    float *state, uint32_t state_len,
    float *prediction, uint32_t prediction_len,
    float *error_norm, uint32_t error_norm_len,
    char *err_out, int err_cap
) {
    if (handle == NULL) {
        resonance_write_error(err_out, err_cap, @"solver is not initialized");
        return 1;
    }
    @autoreleasepool {
        NSString *error = nil;
        if (![from(handle) readWireSlot:slot
                                  state:state
                               stateLen:state_len
                             prediction:prediction
                          predictionLen:prediction_len
                              errorNorm:error_norm
                           errorNormLen:error_norm_len
                                   error:&error]) {
            resonance_write_error(err_out, err_cap, error ?: @"read wire layers failed");
            return 1;
        }
    }
    return 0;
}

int batch_solver_read_weights(
    void *handle, uint32_t slot,
    float *w, size_t wl, float *r, size_t rl, float *a, size_t al, float *v, size_t vl,
    char *err_out, int err_cap
) {
    (void)wl; (void)rl; (void)al; (void)vl;
    if (handle == NULL) { resonance_write_error(err_out, err_cap, @"solver is not initialized"); return 1; }
    @autoreleasepool { [from(handle) readSlot:slot w:w r:r a:a v:v]; }
    return 0;
}


