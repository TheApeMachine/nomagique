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

    // Fused settle parameters
    uint max_inference_steps;
    uint min_inference_steps;
    uint line_search_halvings;
    uint monotone_state_steps;
    float lr_state;
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

// ---- fused GPU-side settle loop -------------------------------------------
kernel void bsettle_fused(
    device float* z                     [[buffer(0)]],
    device float* err                   [[buffer(1)]],
    device const float* precision       [[buffer(2)]],
    device const float* W               [[buffer(3)]],
    device const float* R               [[buffer(4)]],
    device const float* A               [[buffer(5)]],
    device const float* prevTop         [[buffer(6)]],
    device const float* temporalPrec    [[buffer(7)]],
    device const uint* arch_dim         [[buffer(8)]],
    device const uint* z_off            [[buffer(9)]],
    device const uint* pred_off         [[buffer(10)]],
    device const uint* w_off            [[buffer(11)]],
    device const uint* r_off            [[buffer(12)]],
    device const uint* layer_row        [[buffer(13)]],
    device const uint* has_prev         [[buffer(14)]],
    device float* energy_out            [[buffer(15)]],
    device float* reconstruction_out    [[buffer(16)]],
    constant BatchDims& d               [[buffer(17)]],
    uint s                              [[threadgroup_position_in_grid]],
    uint tid                            [[thread_position_in_threadgroup]],
    uint tcount                         [[threads_per_threadgroup]]
) {
    threadgroup float sh_z[1024];
    threadgroup float sh_saved[1024];
    threadgroup float sh_grad[1024];
    threadgroup float sh_pred[1024];
    threadgroup float sh_err[1024];
    threadgroup float sh_precision[1024];
    threadgroup float sh_tmp_a[1024];
    threadgroup float sh_tmp_b[1024];
    
    threadgroup float sh_temporal_err[1024];
    threadgroup float sh_temporal_prec[1024];
    threadgroup float sh_prev_top[1024];

    threadgroup float sh_energy_old;
    threadgroup float sh_energy_new;
    threadgroup float sh_energy_start;
    threadgroup float sh_step;
    threadgroup uint sh_active;
    threadgroup uint sh_flag;

    uint n = d.n;

    // Load Z state
    for (uint i = tid; i < d.z_total; i += tcount) {
        sh_z[i] = z[i * n + s];
    }

    // Load precision
    for (uint i = tid; i < d.pred_total; i += tcount) {
        sh_precision[i] = precision[i * n + s];
    }

    // Load temporal states
    uint top_dim = d.top_dim;
    if (has_prev[0] != 0u) {
        for (uint i = tid; i < top_dim; i += tcount) {
            sh_prev_top[i] = prevTop[i * n + s];
            sh_temporal_prec[i] = temporalPrec[i * n + s];
        }
    }

    if (tid == 0u) {
        sh_active = 1u;
    }
    threadgroup_barrier(mem_flags::mem_threadgroup);

    // Helpers
    auto run_gemv_W = [&](uint l) {
        uint rows = arch_dim[l];
        uint cols = arch_dim[l + 1];
        uint mbase = d.w_total * s + w_off[l];
        uint x_layer_off = z_off[l + 1];
        uint out_layer_off = pred_off[l];

        for (uint r = tid; r < rows; r += tcount) {
            float acc = 0.0f;
            for (uint c = 0u; c < cols; ++c) {
                acc += W[mbase + r * cols + c] * sh_z[x_layer_off + c];
            }
            sh_pred[out_layer_off + r] = tanh(acc);
        }
    };

    auto run_gemv_T_W = [&](uint l, threadgroup const float* x, threadgroup float* out) {
        uint rows = arch_dim[l];
        uint cols = arch_dim[l + 1];
        uint mbase = d.w_total * s + w_off[l];

        for (uint c = tid; c < cols; c += tcount) {
            float acc = 0.0f;
            for (uint r = 0u; r < rows; ++r) {
                acc += W[mbase + r * cols + c] * x[r];
            }
            out[c] = acc;
        }
    };

    auto run_gemv_A = [&](threadgroup float* out) {
        uint rows = top_dim;
        uint cols = top_dim;
        uint mbase = top_dim * top_dim * s;
        for (uint r = tid; r < rows; r += tcount) {
            float acc = 0.0f;
            for (uint c = 0u; c < cols; ++c) {
                acc += A[mbase + r * cols + c] * sh_prev_top[c];
            }
            out[r] = tanh(acc);
        }
    };

    auto run_predict = [&]() {
        for (uint l = 0u; l < d.num_links; ++l) {
            run_gemv_W(l);
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        for (uint l = 0u; l < d.num_links; ++l) {
            uint rows = arch_dim[l];
            uint z_base = z_off[l];
            uint pred_base = pred_off[l];
            for (uint r = tid; r < rows; r += tcount) {
                sh_err[pred_base + r] = sh_z[z_base + r] - sh_pred[pred_base + r];
            }
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);
    };

    auto run_temporal_err = [&]() {
        if (has_prev[0] != 0u) {
            run_gemv_A(sh_tmp_b);
            threadgroup_barrier(mem_flags::mem_threadgroup);
            uint top_base = z_off[d.arch_len - 1u];
            for (uint r = tid; r < top_dim; r += tcount) {
                sh_temporal_err[r] = sh_z[top_base + r] - sh_tmp_b[r];
            }
            threadgroup_barrier(mem_flags::mem_threadgroup);
        }
    };

    auto run_energy = [&]() {
        float local = 0.0f;
        for (uint l = 0u; l < d.num_links; ++l) {
            uint rows = arch_dim[l];
            uint base = pred_off[l];
            for (uint r = tid; r < rows; r += tcount) {
                float e = sh_err[base + r];
                float w = (d.use_precision != 0u) ? sh_precision[base + r] * e : e;
                local += 0.5f * w * e;
            }
        }

        if (has_prev[0] != 0u) {
            for (uint r = tid; r < top_dim; r += tcount) {
                float e = sh_temporal_err[r];
                float w = (d.use_precision != 0u) ? sh_temporal_prec[r] * e : e;
                local += 0.5f * d.temporal_weight * w * e;
            }
        }

        if (d.latent_decay > 0.0f || d.sparsity > 0.0f) {
            for (uint l = 1u; l < d.arch_len; ++l) {
                uint dim = arch_dim[l];
                uint base = z_off[l];
                for (uint r = tid; r < dim; r += tcount) {
                    float v = sh_z[base + r];
                    if (d.latent_decay > 0.0f) local += 0.5f * d.latent_decay * v * v;
                    if (d.sparsity > 0.0f) local += d.sparsity * fabs(v);
                }
            }
        }

        sh_tmp_a[tid] = local;
        threadgroup_barrier(mem_flags::mem_threadgroup);
        for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
            if (tid < stride) sh_tmp_a[tid] += sh_tmp_a[tid + stride];
            threadgroup_barrier(mem_flags::mem_threadgroup);
        }
        return sh_tmp_a[0];
    };

    auto run_grad_layer = [&](uint layer) {
        uint dim = arch_dim[layer];
        uint gOff = z_off[layer];

        for (uint r = tid; r < dim; r += tcount) {
            sh_grad[gOff + r] = 0.0f;
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        uint topIndex = d.arch_len - 1u;
        if (layer < topIndex) {
            uint predOff = pred_off[layer];
            for (uint r = tid; r < dim; r += tcount) {
                float e = sh_err[predOff + r];
                float w = (d.use_precision != 0u) ? sh_precision[predOff + r] * e : e;
                sh_grad[gOff + r] += w;
            }
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        uint belowRows = arch_dim[layer - 1u];
        uint belowPredOff = pred_off[layer - 1u];
        for (uint r = tid; r < belowRows; r += tcount) {
            float p = sh_pred[belowPredOff + r];
            float deriv = 1.0f - p * p;
            float val = deriv * sh_err[belowPredOff + r];
            if (d.use_precision != 0u) {
                val *= sh_precision[belowPredOff + r];
            }
            sh_tmp_a[r] = val;
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        run_gemv_T_W(layer - 1u, sh_tmp_a, sh_tmp_b);
        threadgroup_barrier(mem_flags::mem_threadgroup);

        for (uint r = tid; r < dim; r += tcount) {
            sh_grad[gOff + r] -= sh_tmp_b[r];
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        if (layer == topIndex && has_prev[0] != 0u) {
            for (uint r = tid; r < dim; r += tcount) {
                float e = sh_z[gOff + r] - sh_tmp_b[r];
                if (d.use_precision != 0u) {
                    e *= sh_temporal_prec[r];
                }
                sh_grad[gOff + r] += d.temporal_weight * e;
            }
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        if (d.latent_decay > 0.0f) {
            for (uint r = tid; r < dim; r += tcount) {
                sh_grad[gOff + r] += d.latent_decay * sh_z[gOff + r];
            }
        }
        if (d.sparsity > 0.0f) {
            for (uint r = tid; r < dim; r += tcount) {
                float v = sh_z[gOff + r];
                if (v > 0.0f) sh_grad[gOff + r] += d.sparsity;
                else if (v < 0.0f) sh_grad[gOff + r] -= d.sparsity;
            }
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        float sumSq = 0.0f;
        for (uint r = tid; r < dim; r += tcount) {
            float g = sh_grad[gOff + r];
            sumSq += g * g;
        }
        sh_tmp_a[tid] = sumSq;
        threadgroup_barrier(mem_flags::mem_threadgroup);
        for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
            if (tid < stride) sh_tmp_a[tid] += sh_tmp_a[tid + stride];
            threadgroup_barrier(mem_flags::mem_threadgroup);
        }

        float norm = sqrt(sh_tmp_a[0]);
        if (norm > d.grad_clip) {
            float scale = d.grad_clip / (norm + 1e-12f);
            for (uint r = tid; r < dim; r += tcount) {
                sh_grad[gOff + r] *= scale;
            }
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);
    };

    auto run_all_grads = [&]() {
        uint topIndex = d.arch_len - 1u;
        for (uint l = 1u; l <= topIndex; ++l) {
            run_grad_layer(l);
        }
    };

    // First predict and energy
    run_temporal_err();
    run_predict();
    float initial_energy = run_energy();
    if (tid == 0u) {
        sh_energy_old = initial_energy;
    }
    threadgroup_barrier(mem_flags::mem_threadgroup);

    // Iterative settle loop
    for (uint step = 0u; step < d.max_inference_steps; ++step) {
        if (sh_active == 0u) break;

        if (tid == 0u) {
            sh_energy_start = sh_energy_old;
            sh_step = d.lr_state;
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        run_temporal_err();
        run_predict();
        run_all_grads();

        // Save starting z
        for (uint i = tid; i < d.z_total; i += tcount) {
            sh_saved[i] = sh_z[i];
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);

        uint halvings = d.line_search_halvings;
        for (uint h = 0u; h <= halvings; ++h) {
            // Apply state update
            for (uint i = tid; i < d.z_total; i += tcount) {
                if (layer_row[i] > 0u) {
                    float v = sh_saved[i] - sh_step * sh_grad[i];
                    sh_z[i] = clamp(v, -d.state_clip, d.state_clip);
                }
            }
            threadgroup_barrier(mem_flags::mem_threadgroup);

            run_temporal_err();
            run_predict();
            float new_energy = run_energy();

            if (tid == 0u) {
                sh_energy_new = new_energy;

                if (d.monotone_state_steps == 0u || sh_energy_new <= sh_energy_old + 1e-12f) {
                    sh_flag = 1u; // accept
                } else if (h == halvings) {
                    sh_flag = 0u; // revert
                } else {
                    sh_flag = 2u; // halve step
                }
            }
            threadgroup_barrier(mem_flags::mem_threadgroup);

            if (sh_flag == 1u) {
                if (tid == 0u) {
                    sh_energy_old = sh_energy_new;
                }
                threadgroup_barrier(mem_flags::mem_threadgroup);
                break;
            } else if (sh_flag == 0u) {
                for (uint i = tid; i < d.z_total; i += tcount) {
                    if (layer_row[i] > 0u) {
                        sh_z[i] = sh_saved[i];
                    }
                }
                threadgroup_barrier(mem_flags::mem_threadgroup);
                break;
            } else {
                if (tid == 0u) {
                    sh_step *= 0.5f;
                }
                threadgroup_barrier(mem_flags::mem_threadgroup);
            }
        }

        // Early stop check
        if (tid == 0u) {
            uint pastMin = (step + 1u >= d.min_inference_steps) ? 1u : 0u;
            float rel = fabs(sh_energy_start - sh_energy_old) / (fabs(sh_energy_start) + 1e-12f);
            if (pastMin && rel < d.early_stop_tol) {
                sh_active = 0u;
            }
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }

    // Write back states and errors
    for (uint i = tid; i < d.z_total; i += tcount) {
        z[i * n + s] = sh_z[i];
    }
    for (uint i = tid; i < d.pred_total; i += tcount) {
        err[i * n + s] = sh_err[i];
    }

    if (tid == 0u) {
        energy_out[s] = sh_energy_old;
    }
    threadgroup_barrier(mem_flags::mem_threadgroup);

    // Compute reconstruction error
    float reconSumSq = 0.0f;
    uint r0_rows = arch_dim[0];
    uint r0_base = pred_off[0];
    for (uint r = tid; r < r0_rows; r += tcount) {
        float val = sh_err[r0_base + r];
        reconSumSq += val * val;
    }
    sh_tmp_a[tid] = reconSumSq;
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
        if (tid < stride) sh_tmp_a[tid] += sh_tmp_a[tid + stride];
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }
    if (tid == 0u) {
        reconstruction_out[s] = sqrt(sh_tmp_a[0]);
    }
}




