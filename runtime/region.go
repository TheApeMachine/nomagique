package runtime

/*
Region is one confluence region's evidence in a frame.
*/
type Region struct {
	Symbol      string  `json:"symbol"`
	Key         uint8   `json:"key"`
	Deformation float64 `json:"deformation"`
	Brightness  float64 `json:"brightness"`
	Counts      int     `json:"counts"`
}
