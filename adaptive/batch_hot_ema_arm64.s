#include "textflag.h"

// EMAState: Value 0, Prev 8, Min 16, Max 24, Rate 32
// emaSamplesHotParams: state+0 count+8 samples+16 out+24
// func observeEMASamplesHotARM64(params *emaSamplesHotParams)
TEXT ·observeEMASamplesHotARM64(SB), NOSPLIT, $0-8
	MOVD	p+0(FP), R9

	MOVD	0(R9), R8
	FMOVD	0(R8), F5
	FMOVD	8(R8), F6
	FMOVD	16(R8), F10
	FMOVD	24(R8), F11
	MOVD	8(R9), R10
	MOVD	16(R9), R11
	MOVD	24(R9), R12

	CMP	$0, R10
	BLE	done

loop:
	FMOVD	(R11), F0

	FMINNMD	F10, F0, F1
	FMOVD	F1, F10
	FMAXNMD	F11, F0, F2
	FMOVD	F2, F11

	FSUBD	F10, F2, F7
	FCMPD	$(0.0), F7
	BNE	ema_step

	FMOVD	F0, F6
	FMOVD	F5, (R12)
	B	ema_advance

ema_step:
	FSUBD	F6, F0, F1
	FCMPD	$(0.0), F1
	BPL	ema_rate
	FNEGD	F1, F1
ema_rate:
	FDIVD	F7, F1, F1
	FMOVD	F1, 32(R8)
	FSUBD	F5, F0, F4
	FMADDD	F1, F5, F4, F5
	FMOVD	F0, F6
	FMOVD	F5, (R12)

ema_advance:
	ADD	$8, R11
	ADD	$8, R12
	SUB	$1, R10
	CMP	$0, R10
	BGT	loop

done:
	FMOVD	F5, 0(R8)
	FMOVD	F6, 8(R8)
	FMOVD	F10, 16(R8)
	FMOVD	F11, 24(R8)
	RET
