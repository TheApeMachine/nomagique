package runtime

import "github.com/bytedance/sonic"

/*
Sonic is the shared envelope encoder. NaN / ±Inf become JSON null so one
non-finite score cannot abort publish or tick persistence.
*/
var Sonic = sonic.Config{
	EncodeNullForInfOrNan: true,
}.Froze()
