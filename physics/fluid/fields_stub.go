//go:build !darwin || !cgo

package fluid

import "fmt"

/*
Fields reports that no Metal domain exists on this platform.
*/
func (domain *Domain) Fields() (Fields, error) {
	return Fields{}, fmt.Errorf("fluid: Metal domain requires darwin with cgo")
}
