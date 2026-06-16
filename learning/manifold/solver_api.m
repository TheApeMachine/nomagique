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
    if (input_len != s.arch[0]) { resonance_write_error(err_out, err_cap, @"input dimension mismatch"); return 1; }
    BOOL hasTarget = (target != NULL && target_len == s.targetDim && s.targetDim > 0u);
    @autoreleasepool { [s setInputSlot:slot input:input target:target hasTarget:hasTarget]; }
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
