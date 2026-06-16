#include <metal_stdlib>
using namespace metal;

// =============================================================================
// Resonance Manifold Kernels (predictive-coding inference + learning)
// =============================================================================
// The resonance manifold is a stack of small column-vector latents z[l] linked
// by generative weights W[l] (z[l] <- tanh(W[l] z[l+1])) and recognition
// weights R[l]. Every primitive here operates on flat float buffers; the host
// (Objective-C) supplies per-call offsets/lengths and orchestrates the settle
// line-search and the learning updates. Vectors are tiny, so each kernel is a
// 1-D dispatch over the active layer dimension.
//
// float32 GPU arithmetic means parity with the gonum reference is behavioral
// (tolerance-checked), not bit-exact.
// =============================================================================

// -----------------------------------------------------------------------------
// Generic matrix-vector products. Matrices are row-major (rows x cols).
// -----------------------------------------------------------------------------

// out[rows] = M[rows x cols] * x[cols]
kernel void gemv(
    device const float* matrix [[buffer(0)]],
    device const float* x      [[buffer(1)]],
    device float* out          [[buffer(2)]],
    constant uint& rows        [[buffer(3)]],
    constant uint& cols        [[buffer(4)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= rows) return;
    float acc = 0.0f;
    uint base = gid * cols;
    for (uint c = 0u; c < cols; ++c) {
        acc += matrix[base + c] * x[c];
    }
    out[gid] = acc;
}

// out[rows] = tanh(M[rows x cols] * x[cols])
kernel void gemv_tanh(
    device const float* matrix [[buffer(0)]],
    device const float* x      [[buffer(1)]],
    device float* out          [[buffer(2)]],
    constant uint& rows        [[buffer(3)]],
    constant uint& cols        [[buffer(4)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= rows) return;
    float acc = 0.0f;
    uint base = gid * cols;
    for (uint c = 0u; c < cols; ++c) {
        acc += matrix[base + c] * x[c];
    }
    out[gid] = tanh(acc);
}

// out[cols] = M[rows x cols]^T * x[rows]  (column gid of M dotted with x)
kernel void gemv_transpose(
    device const float* matrix [[buffer(0)]],
    device const float* x      [[buffer(1)]],
    device float* out          [[buffer(2)]],
    constant uint& rows        [[buffer(3)]],
    constant uint& cols        [[buffer(4)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= cols) return;
    float acc = 0.0f;
    for (uint r = 0u; r < rows; ++r) {
        acc += matrix[r * cols + gid] * x[r];
    }
    out[gid] = acc;
}

// -----------------------------------------------------------------------------
// Elementwise vector primitives (length n).
// -----------------------------------------------------------------------------

kernel void vec_copy(
    device const float* src [[buffer(0)]],
    device float* dst       [[buffer(1)]],
    constant uint& n        [[buffer(2)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    dst[gid] = src[gid];
}

// out = a - b
kernel void vec_sub(
    device const float* a [[buffer(0)]],
    device const float* b [[buffer(1)]],
    device float* out     [[buffer(2)]],
    constant uint& n      [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    out[gid] = a[gid] - b[gid];
}

// out = a + b
kernel void vec_add(
    device const float* a [[buffer(0)]],
    device const float* b [[buffer(1)]],
    device float* out     [[buffer(2)]],
    constant uint& n      [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    out[gid] = a[gid] + b[gid];
}

// out = a .* b
kernel void vec_mulelem(
    device const float* a [[buffer(0)]],
    device const float* b [[buffer(1)]],
    device float* out     [[buffer(2)]],
    constant uint& n      [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    out[gid] = a[gid] * b[gid];
}

// out = scalar * src
kernel void vec_scale(
    device const float* src [[buffer(0)]],
    device float* out       [[buffer(1)]],
    constant float& scalar  [[buffer(2)]],
    constant uint& n        [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    out[gid] = scalar * src[gid];
}

// out = out + scalar * src   (axpy, accumulating in place)
kernel void vec_axpy(
    device const float* src [[buffer(0)]],
    device float* out       [[buffer(1)]],
    constant float& scalar  [[buffer(2)]],
    constant uint& n        [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    out[gid] += scalar * src[gid];
}

// dst = clamp(src, -limit, +limit)
kernel void vec_clamp(
    device const float* src [[buffer(0)]],
    device float* dst       [[buffer(1)]],
    constant float& limit   [[buffer(2)]],
    constant uint& n        [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    dst[gid] = clamp(src[gid], -limit, limit);
}

// out = 1 - p .* p  (derivative signal for tanh prediction p)
kernel void tanh_deriv(
    device const float* p [[buffer(0)]],
    device float* out     [[buffer(1)]],
    constant uint& n      [[buffer(2)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    float v = p[gid];
    out[gid] = 1.0f - v * v;
}

// merged = mix * td + (1 - mix) * bu, then clamp to +/- limit
kernel void merge_clamp(
    device const float* td  [[buffer(0)]],
    device const float* bu  [[buffer(1)]],
    device float* out       [[buffer(2)]],
    constant float& mix     [[buffer(3)]],
    constant float& limit   [[buffer(4)]],
    constant uint& n        [[buffer(5)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    float v = mix * td[gid] + (1.0f - mix) * bu[gid];
    out[gid] = clamp(v, -limit, limit);
}

// Subgradient of the L1 sparsity penalty: out += sparsity * sign(z)
kernel void sparsity_subgrad(
    device const float* z   [[buffer(0)]],
    device float* out       [[buffer(1)]],
    constant float& sparsity [[buffer(2)]],
    constant uint& n        [[buffer(3)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    float v = z[gid];
    if (v > 0.0f) out[gid] += sparsity;
    else if (v < 0.0f) out[gid] -= sparsity;
}

// -----------------------------------------------------------------------------
// Precision update: EMA of squared error -> clamped inverse-variance precision.
// variance <- (1-beta) variance + beta * err^2 ; precision = clamp(1/(var+eps)).
// -----------------------------------------------------------------------------
kernel void precision_update(
    device const float* err   [[buffer(0)]],
    device float* variance    [[buffer(1)]],
    device float* precision   [[buffer(2)]],
    constant float& beta      [[buffer(3)]],
    constant float& eps       [[buffer(4)]],
    constant float& pmin      [[buffer(5)]],
    constant float& pmax      [[buffer(6)]],
    constant uint& n          [[buffer(7)]],
    uint gid [[thread_position_in_grid]]
) {
    if (gid >= n) return;
    float e = err[gid];
    float var = (1.0f - beta) * variance[gid] + beta * (e * e);
    variance[gid] = var;
    float raw = 1.0f / (var + eps);
    precision[gid] = clamp(raw, pmin, pmax);
}

// -----------------------------------------------------------------------------
// Weight update via outer product: M[rows x cols] += scalar * (a[rows] x b[cols])
// One thread per matrix element. Optional multiplicative weight decay applied
// after the additive update when decay > 0.
// -----------------------------------------------------------------------------
kernel void outer_update(
    device float* matrix    [[buffer(0)]],
    device const float* a   [[buffer(1)]],
    device const float* b   [[buffer(2)]],
    constant float& scalar  [[buffer(3)]],
    constant float& decay   [[buffer(4)]],
    constant uint& rows     [[buffer(5)]],
    constant uint& cols     [[buffer(6)]],
    uint gid [[thread_position_in_grid]]
) {
    uint total = rows * cols;
    if (gid >= total) return;
    uint r = gid / cols;
    uint c = gid - r * cols;
    float v = matrix[gid] + scalar * a[r] * b[c];
    if (decay > 0.0f) {
        v *= (1.0f - decay);
    }
    matrix[gid] = v;
}

// -----------------------------------------------------------------------------
// Reductions. Single-threadgroup tree reductions: the vectors involved are
// tiny (layer dimensions), so one threadgroup with a serial-ish reduce is
// ample and keeps the host orchestration simple. Launched with one threadgroup.
// -----------------------------------------------------------------------------

// out[0] = sum_i a[i] * b[i]
kernel void reduce_dot(
    device const float* a [[buffer(0)]],
    device const float* b [[buffer(1)]],
    device float* out     [[buffer(2)]],
    constant uint& n      [[buffer(3)]],
    uint tid [[thread_position_in_threadgroup]],
    uint tcount [[threads_per_threadgroup]]
) {
    threadgroup float scratch[256];
    float local = 0.0f;
    for (uint i = tid; i < n; i += tcount) {
        local += a[i] * b[i];
    }
    scratch[tid] = local;
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
        if (tid < stride) {
            scratch[tid] += scratch[tid + stride];
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }
    if (tid == 0u) {
        out[0] = scratch[0];
    }
}

// out[0] = sum_i |a[i]|   (L1 norm, for the sparsity energy term)
kernel void reduce_abs_sum(
    device const float* a [[buffer(0)]],
    device float* out     [[buffer(1)]],
    constant uint& n      [[buffer(2)]],
    uint tid [[thread_position_in_threadgroup]],
    uint tcount [[threads_per_threadgroup]]
) {
    threadgroup float scratch[256];
    float local = 0.0f;
    for (uint i = tid; i < n; i += tcount) {
        local += fabs(a[i]);
    }
    scratch[tid] = local;
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
        if (tid < stride) {
            scratch[tid] += scratch[tid + stride];
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }
    if (tid == 0u) {
        out[0] = scratch[0];
    }
}

// Grad-clip in place: if ||g||_2 > clip then g *= clip / (||g|| + 1e-12).
// Single threadgroup computes the norm then rescales.
kernel void grad_clip(
    device float* g        [[buffer(0)]],
    constant float& clip   [[buffer(1)]],
    constant uint& n       [[buffer(2)]],
    uint tid [[thread_position_in_threadgroup]],
    uint tcount [[threads_per_threadgroup]]
) {
    threadgroup float scratch[256];
    threadgroup float factor;
    float local = 0.0f;
    for (uint i = tid; i < n; i += tcount) {
        float v = g[i];
        local += v * v;
    }
    scratch[tid] = local;
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint stride = tcount / 2u; stride > 0u; stride >>= 1u) {
        if (tid < stride) {
            scratch[tid] += scratch[tid + stride];
        }
        threadgroup_barrier(mem_flags::mem_threadgroup);
    }
    if (tid == 0u) {
        float norm = sqrt(scratch[0]);
        factor = (norm > clip) ? (clip / (norm + 1e-12f)) : 1.0f;
    }
    threadgroup_barrier(mem_flags::mem_threadgroup);
    for (uint i = tid; i < n; i += tcount) {
        g[i] *= factor;
    }
}
