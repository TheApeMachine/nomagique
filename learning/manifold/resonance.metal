#include <metal_stdlib>
using namespace metal;

// =============================================================================
// Batched Resonance Manifold Kernels (predictive-coding, N symbols in lockstep)
// =============================================================================
// Each of N symbols is an INDEPENDENT predictive-coding manifold with its OWN
// weights (W/R/A/V). A single settle cannot parallelize internally (step k needs
// step k-1), so the parallelism comes from the BATCH: all N symbols advance in
// lockstep, every kernel doing N x the work per GPU roundtrip. This amortizes
// the ~10-50us command-buffer latency across N symbols instead of paying it per
// symbol — the only structure in which the GPU beats the per-symbol BLAS CPU
// path for this workload.
//
// Layout (N = batch size):
//   State (z, pred, err, precision, variance): per layer l, scalar (row r,
//     slot s) at  z_off[l]*N + r*N + s.  Layer l occupies dim_l*N floats,
//     row-major [dim_l][N].
//   Weights (per symbol): slot s block of size w_total at offset w_total*s;
//     within it W[l] is row-major rows x cols at w_off[l].  R/A/V analogous with
//     r_total / a_total(=top*top) / v_total(=target*top).
//   Per-column scalars (step, energy_old/new, flags, done, active): length N.
//
// Line search is PER COLUMN: symbol A may accept at full step while symbol B
// halves — exactly as N independent CPU settles would behave. Converged columns
// are frozen via the active mask so later steps leave them untouched.
//
// float32 GPU vs float64 gonum: parity is behavioral (tolerance-checked).
// =============================================================================

struct BatchDims {
    uint n;            // batch size
    uint arch_len;
    uint num_links;
    uint top_dim;
    uint target_dim;
    uint z_total;      // sum dim_l  (per-symbol latent scalars, NOT x N)
    uint pred_total;   // sum rows_l
    uint w_total;      // per-symbol generative weight scalars
    uint r_total;      // per-symbol recognition weight scalars
    uint use_precision;
    float temporal_weight;
    float latent_decay;
    float sparsity;
    float state_clip;
    float grad_clip;
    float early_stop_tol;
};

// ---- batched matrix-vector products (per symbol weights) ------------------
// For link l: out[(row r, slot s)] = act( sum_c W_s[l][r,c] * x[(c,s)] ).
// Thread grid = rows * N ; gid -> (r, s).

inline void gemv_batched_impl(
    device const float* matrix, uint mat_base_stride, uint mat_off,
    device const float* x, uint x_layer_off,
    device float* out, uint out_layer_off,
    uint rows, uint cols, uint n, bool act, uint gid
) {
    uint total = rows * n;
    if (gid >= total) return;
    uint r = gid / n;
    uint s = gid - r * n;
    uint mbase = mat_base_stride * s + mat_off + r * cols;
    float acc = 0.0f;
    for (uint c = 0u; c < cols; ++c) {
        acc += matrix[mbase + c] * x[x_layer_off * n + c * n + s];
    }
    float v = act ? tanh(acc) : acc;
    out[out_layer_off * n + r * n + s] = v;
}

kernel void bgemv(
    device const float* matrix [[buffer(0)]],
    device const float* x      [[buffer(1)]],
    device float* out          [[buffer(2)]],
    constant uint& mat_base    [[buffer(3)]],
    constant uint& mat_off     [[buffer(4)]],
    constant uint& x_off       [[buffer(5)]],
    constant uint& out_off     [[buffer(6)]],
    constant uint& rows        [[buffer(7)]],
    constant uint& cols        [[buffer(8)]],
    constant uint& n           [[buffer(9)]],
    constant uint& act         [[buffer(10)]],
    uint gid [[thread_position_in_grid]]
) {
    gemv_batched_impl(matrix, mat_base, mat_off, x, x_off, out, out_off,
                      rows, cols, n, act != 0u, gid);
}

// NOTE: a threadgroup-tiled bgemv (cache input column in threadgroup memory, one
// group per symbol) was implemented and benchmarked — it ran 3-6x SLOWER across
// widths because one group/symbol under-occupies the GPU and the strided staging
// outweighs the saved input re-reads. The naive flat-grid kernel above wins on
// this hardware; the tiled variant was removed rather than kept as a dead path.

// out[(c,s)] = sum_r W_s[r,c] * x[(r,s)]   (transpose). grid = cols * N.
kernel void bgemv_t(
    device const float* matrix [[buffer(0)]],
    device const float* x      [[buffer(1)]],
    device float* out          [[buffer(2)]],
    constant uint& mat_base    [[buffer(3)]],
    constant uint& mat_off     [[buffer(4)]],
    constant uint& x_off       [[buffer(5)]],
    constant uint& out_off     [[buffer(6)]],
    constant uint& rows        [[buffer(7)]],
    constant uint& cols        [[buffer(8)]],
    constant uint& n           [[buffer(9)]],
    uint gid [[thread_position_in_grid]]
) {
    uint total = cols * n;
    if (gid >= total) return;
    uint c = gid / n;
    uint s = gid - c * n;
    uint mbase = mat_base * s + mat_off;
    float acc = 0.0f;
    for (uint r = 0u; r < rows; ++r) {
        acc += matrix[mbase + r * cols + c] * x[x_off * n + r * n + s];
    }
    out[out_off * n + c * n + s] = acc;
}

// ---- batched elementwise (over a layer block of dim*N scalars) ------------
// All operate on flat ranges [base .. base + dim*n).

kernel void bvec_sub(
    device const float* a [[buffer(0)]],
    device const float* b [[buffer(1)]],
    device float* out     [[buffer(2)]],
    constant uint& count  [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) { if (gid >= count) return; out[gid] = a[gid] - b[gid]; }

kernel void bvec_mulelem(
    device const float* a [[buffer(0)]],
    device const float* b [[buffer(1)]],
    device float* out     [[buffer(2)]],
    constant uint& count  [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) { if (gid >= count) return; out[gid] = a[gid] * b[gid]; }

kernel void bvec_copy(
    device const float* src [[buffer(0)]],
    device float* dst       [[buffer(1)]],
    constant uint& count    [[buffer(2)]],
    uint gid [[thread_position_in_grid]]
) { if (gid >= count) return; dst[gid] = src[gid]; }

kernel void btanh_deriv(
    device const float* p [[buffer(0)]],
    device float* out     [[buffer(1)]],
    constant uint& count  [[buffer(2)]],
    uint gid [[thread_position_in_grid]]
) { if (gid >= count) return; float v = p[gid]; out[gid] = 1.0f - v * v; }

// precision row r broadcast across slots: precision[(r,s)] indexes per-(r,s),
// so this is a plain elementwise mul over the layer block. Provided for clarity.

// ---- per-column reductions: energy per symbol -----------------------------
// One threadgroup per column s. Mirrors Energy() exactly, per symbol. Writes
// energy[out_slot_base + s]. Uses dims/offset tables to walk every layer.

kernel void benergy(
    device const float* z         [[buffer(0)]],
    device const float* err       [[buffer(1)]],
    device const float* precision [[buffer(2)]],
    device const float* temporalErr  [[buffer(3)]],
    device const float* temporalPrec [[buffer(4)]],
    device const uint* arch_dim   [[buffer(5)]],
    device const uint* z_off      [[buffer(6)]],
    device const uint* pred_off   [[buffer(7)]],
    constant BatchDims& d         [[buffer(8)]],
    device const uint* has_prev   [[buffer(9)]],   // [1] temporal active flag
    device float* energy_out      [[buffer(10)]],
    constant uint& out_base       [[buffer(11)]],
    uint s   [[threadgroup_position_in_grid]],
    uint tid [[thread_position_in_threadgroup]],
    uint tcount [[threads_per_threadgroup]]
) {
    // Sized to the Apple Silicon hardware ceiling (1024 = 32 SIMD groups). The
    // host launches a power-of-two threadgroup no larger than the reduction
    // length (clamped to the pipeline's maxTotalThreadsPerThreadgroup), so this
    // is always large enough and the binary-tree reduce below stays valid.
    threadgroup float scratch[1024];
    uint n = d.n;
    float local = 0.0f;

    for (uint l = 0u; l < d.num_links; ++l) {
        uint rows = arch_dim[l];
        uint base = pred_off[l] * n;
        for (uint r = tid; r < rows; r += tcount) {
            float e = err[base + r * n + s];
            float w = (d.use_precision != 0u) ? precision[base + r * n + s] * e : e;
            local += 0.5f * w * e;
        }
    }

    if (has_prev[0] != 0u) {
        uint top = d.top_dim;
        for (uint r = tid; r < top; r += tcount) {
            float e = temporalErr[r * n + s];
            float w = (d.use_precision != 0u) ? temporalPrec[r * n + s] * e : e;
            local += 0.5f * d.temporal_weight * w * e;
        }
    }

    if (d.latent_decay > 0.0f || d.sparsity > 0.0f) {
        for (uint l = 1u; l < d.arch_len; ++l) {
            uint dim = arch_dim[l];
            uint base = z_off[l] * n;
            for (uint r = tid; r < dim; r += tcount) {
                float v = z[base + r * n + s];
                if (d.latent_decay > 0.0f) local += 0.5f * d.latent_decay * v * v;
                if (d.sparsity > 0.0f) local += d.sparsity * fabs(v);
            }
        }
    }

    scratch[tid] = local;
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
        if (tid < stride) scratch[tid] += scratch[tid + stride];
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }
    if (tid == 0u) energy_out[out_base + s] = scratch[0];
}

// ---- per-layer gradient (batched), into grad cache [z layout] -------------
// Mirrors stateGradientForLayer for every column at once. grid = dim * N.
// Scratch tmpA/tmpB are per-(row,slot); gradient written to gradCache.
// belowSignal and correction are computed by the host as separate dispatches
// (reusing bgemv_t and elementwise); this kernel does the per-element assembly
// of the local terms. To keep parity exact and the code legible we instead
// assemble gradients with the same primitive kernels the host already drives.

// grad clip PER COLUMN: scale gradient column s by clip/(||g_s||+eps) if over.
// One threadgroup per column over the gradient's layer block.
kernel void bgrad_clip_layer(
    device float* grad         [[buffer(0)]],
    constant uint& layer_off   [[buffer(1)]],
    constant uint& dim         [[buffer(2)]],
    constant uint& n           [[buffer(3)]],
    constant float& clip       [[buffer(4)]],
    uint s   [[threadgroup_position_in_grid]],
    uint tid [[thread_position_in_threadgroup]],
    uint tcount [[threads_per_threadgroup]]
) {
    threadgroup float scratch[1024];   // hardware ceiling; see benergy note
    threadgroup float factor;
    uint base = layer_off * n;
    float local = 0.0f;
    for (uint r = tid; r < dim; r += tcount) {
        float v = grad[base + r * n + s];
        local += v * v;
    }
    scratch[tid] = local;
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
        if (tid < stride) scratch[tid] += scratch[tid + stride];
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }
    if (tid == 0u) {
        float norm = sqrt(scratch[0]);
        factor = (norm > clip) ? (clip / (norm + 1e-12f)) : 1.0f;
    }
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint r = tid; r < dim; r += tcount) {
        grad[base + r * n + s] *= factor;
    }
}

// axpy on a layer block, per element: out += scalar * src  (scalar uniform).
kernel void bvec_axpy(
    device const float* src [[buffer(0)]],
    device float* out       [[buffer(1)]],
    constant float& scalar  [[buffer(2)]],
    constant uint& count    [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) { if (gid >= count) return; out[gid] += scalar * src[gid]; }

kernel void bvec_scale(
    device const float* src [[buffer(0)]],
    device float* out       [[buffer(1)]],
    constant float& scalar  [[buffer(2)]],
    constant uint& count    [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) { if (gid >= count) return; out[gid] = scalar * src[gid]; }

// scale a layer block by temporal_weight (uniform) — alias of bvec_scale.

// sparsity subgradient over a layer block: out += sparsity * sign(z).
kernel void bsparsity_subgrad(
    device const float* z    [[buffer(0)]],
    device float* out        [[buffer(1)]],
    constant float& sparsity [[buffer(2)]],
    constant uint& count     [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= count) return;
    float v = z[gid];
    if (v > 0.0f) out[gid] += sparsity;
    else if (v < 0.0f) out[gid] -= sparsity;
}

// ---- per-column line search ----------------------------------------------
// apply_state_step: for active columns, z = clamp(saved - step[s]*grad) for
// layers >= 1. grid over z_total * N. layer_of[ scalar/n ] gives the layer; we
// derive (row,slot) from gid. step is per column; active masks frozen columns.
kernel void bapply_state(
    device float* z          [[buffer(0)]],
    device const float* saved[[buffer(1)]],
    device const float* grad [[buffer(2)]],
    device const uint* layer_row [[buffer(3)]], // [z_total] layer per latent row
    device const float* step [[buffer(4)]],     // [n]
    device const uint* active[[buffer(5)]],     // [n]
    constant uint& z_total   [[buffer(6)]],
    constant uint& n         [[buffer(7)]],
    constant float& clip     [[buffer(8)]],
    uint gid [[thread_position_in_grid]]
) {
    uint total = z_total * n;
    if (gid >= total) return;
    uint row = gid / n;          // global latent row index
    uint s = gid - row * n;
    if (active[s] == 0u) return;
    if (layer_row[row] == 0u) return;   // input layer fixed
    float v = saved[gid] - step[s] * grad[gid];
    z[gid] = clamp(v, -clip, clip);
}

// linesearch_decide per column. energy_old/new are [n]. For each active column:
//   accept if !monotone or new<=old+eps -> flags=1, carried=new
//   else if last halving -> flags=0 (revert), carried=old
//   else -> flags=2, carried=old, step*=0.5
kernel void bdecide(
    device const float* energy_old [[buffer(0)]],
    device const float* energy_new [[buffer(1)]],
    device uint* flags             [[buffer(2)]],
    device float* energy_carried   [[buffer(3)]],
    device float* step             [[buffer(4)]],
    device const uint* active      [[buffer(5)]],
    constant uint& monotone        [[buffer(6)]],
    constant uint& is_last         [[buffer(7)]],
    constant uint& n               [[buffer(8)]],
    uint s [[thread_position_in_grid]]
) {
    if (s >= n) return;
    if (active[s] == 0u) { flags[s] = 1u; return; }
    float o = energy_old[s];
    float ne = energy_new[s];
    if (monotone == 0u || ne <= o + 1e-12f) {
        flags[s] = 1u; energy_carried[s] = ne;
    } else if (is_last != 0u) {
        flags[s] = 0u; energy_carried[s] = o;
    } else {
        flags[s] = 2u; energy_carried[s] = o; step[s] *= 0.5f;
    }
}

// revert frozen/rejected columns: where flags[s]==0, restore z from saved.
kernel void brevert(
    device float* z          [[buffer(0)]],
    device const float* saved[[buffer(1)]],
    device const uint* layer_row [[buffer(2)]],
    device const uint* flags [[buffer(3)]],
    constant uint& z_total   [[buffer(4)]],
    constant uint& n         [[buffer(5)]],
    uint gid [[thread_position_in_grid]]
) {
    uint total = z_total * n;
    if (gid >= total) return;
    uint row = gid / n;
    uint s = gid - row * n;
    if (flags[s] != 0u) return;
    if (layer_row[row] == 0u) return;
    z[gid] = saved[gid];
}

// early stop per column: if past_min and relDelta<tol then mark done & inactive.
// Also reports whether ANY column is still active into any_active[0] (host reads
// one scalar to decide whether to keep looping).
kernel void bearly_stop(
    device const float* energy_old [[buffer(0)]],
    device const float* energy_new [[buffer(1)]],
    device uint* active            [[buffer(2)]],
    device atomic_uint* any_active [[buffer(3)]],
    constant float& tol            [[buffer(4)]],
    constant uint& past_min        [[buffer(5)]],
    constant uint& n               [[buffer(6)]],
    uint s [[thread_position_in_grid]]
) {
    if (s >= n) return;
    if (active[s] == 0u) return;
    float o = energy_old[s];
    float ne = energy_new[s];
    float rel = fabs(o - ne) / (fabs(o) + 1e-12f);
    if (past_min != 0u && rel < tol) {
        active[s] = 0u;
    } else {
        atomic_fetch_add_explicit(any_active, 1u, memory_order_relaxed);
    }
}

// init energy_old[s] = current energy (after copy from a benergy slot).

// ---- batched precision update (per (row,slot)) ----------------------------
kernel void bprecision_update(
    device const float* err   [[buffer(0)]],
    device float* variance    [[buffer(1)]],
    device float* precision   [[buffer(2)]],
    constant float& beta      [[buffer(3)]],
    constant float& eps       [[buffer(4)]],
    constant float& pmin      [[buffer(5)]],
    constant float& pmax      [[buffer(6)]],
    constant uint& count      [[buffer(7)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= count) return;
    float e = err[gid];
    float var = (1.0f - beta) * variance[gid] + beta * (e * e);
    variance[gid] = var;
    float raw = 1.0f / (var + eps);
    precision[gid] = clamp(raw, pmin, pmax);
}

// ---- batched outer-product weight update (per symbol) ---------------------
// For symbol s: M_s += factor_s * (a[(.,s)] (x) b[(.,s)]), per-symbol clip.
// Two kernels: bouter_factor computes factor[s]=clip-scaled lr per column;
// bouter_apply applies element (r,c) for every symbol.

kernel void bouter_factor(
    device const float* a    [[buffer(0)]],   // [rows x n] at a_off block
    device const float* b    [[buffer(1)]],   // [cols x n] at b_off block
    device float* factor     [[buffer(2)]],   // [n]
    constant uint& rows      [[buffer(3)]],
    constant uint& cols      [[buffer(4)]],
    constant uint& n         [[buffer(5)]],
    constant float& lr       [[buffer(6)]],
    constant float& clip     [[buffer(7)]],
    uint s [[thread_position_in_grid]]
) {
    if (s >= n) return;
    float aa = 0.0f;
    for (uint r = 0u; r < rows; ++r) { float v = a[r * n + s]; aa += v * v; }
    float bb = 0.0f;
    for (uint c = 0u; c < cols; ++c) { float v = b[c * n + s]; bb += v * v; }
    float norm = sqrt(aa) * sqrt(bb);
    float scale = lr;
    if (norm > clip) scale = lr * (clip / (norm + 1e-12f));
    factor[s] = scale;
}

// grid = rows*cols*n. M_s[r,c] += factor[s]*a[(r,s)]*b[(c,s)] ; *=(1-decay).
// Only active columns (active[s]) update, so frozen symbols stop learning in
// lockstep batches — matches independent per-symbol Learn being skipped.
kernel void bouter_apply(
    device float* matrix     [[buffer(0)]],
    device const float* a    [[buffer(1)]],
    device const float* b    [[buffer(2)]],
    device const float* factor [[buffer(3)]],
    device const uint* active[[buffer(4)]],
    constant uint& mat_base  [[buffer(5)]],   // per-symbol matrix stride
    constant uint& mat_off   [[buffer(6)]],
    constant uint& rows      [[buffer(7)]],
    constant uint& cols      [[buffer(8)]],
    constant uint& n         [[buffer(9)]],
    constant float& decay    [[buffer(10)]],
    constant uint& use_active[[buffer(11)]],
    uint gid [[thread_position_in_grid]]
) {
    uint per = rows * cols;
    uint total = per * n;
    if (gid >= total) return;
    uint elem = gid / n;
    uint s = gid - elem * n;
    if (use_active != 0u && active[s] == 0u) return;
    uint r = elem / cols;
    uint c = elem - r * cols;
    uint midx = mat_base * s + mat_off + r * cols + c;
    float v = matrix[midx] + factor[s] * a[r * n + s] * b[c * n + s];
    if (decay > 0.0f) v *= (1.0f - decay);
    matrix[midx] = v;
}

// merge_clamp for init: z[block] = clamp(mix*td + (1-mix)*bu).
kernel void bmerge_clamp(
    device const float* td [[buffer(0)]],
    device const float* bu [[buffer(1)]],
    device float* out      [[buffer(2)]],
    constant float& mix    [[buffer(3)]],
    constant float& clip   [[buffer(4)]],
    constant uint& count   [[buffer(5)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= count) return;
    float v = mix * td[gid] + (1.0f - mix) * bu[gid];
    out[gid] = clamp(v, -clip, clip);
}



