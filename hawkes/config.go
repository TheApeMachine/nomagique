package hawkes

import "github.com/theapemachine/datura"

/*
MomentConfig returns a Hawkes moment stage config artifact.
*/
func MomentConfig() *datura.Artifact {
	return datura.Acquire("hawkes-moment", datura.APPJSON)
}
