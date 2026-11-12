//go:build !darwin

package main

/*
Build-time Metal pipeline for learning.manifold — metallibLoadError keeps kernels.metallib aligned with the CPU ResonanceManifold path.
*/
func metallibLoadError(payload []byte) string {
	_ = payload
	return ""
}
