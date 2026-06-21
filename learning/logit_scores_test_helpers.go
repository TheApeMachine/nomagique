package learning

func pumpdumpLogitInputs() map[string]any {
	return map[string]any{
		"rvol": map[string]any{
			"source": "rvol",
			"scale":  2.5,
		},
		"precursor": map[string]any{
			"source": "precursor",
			"scale":  2.0,
		},
		"compression": map[string]any{
			"source": "value",
			"scale":  1.5,
			"terms":  []string{"compression", "precursor"},
			"inverts": []string{
				"precursor",
			},
		},
		"ignition": map[string]any{
			"terms":   []string{"rvol", "precursor"},
			"source":  "ignition",
			"combine": "ratio",
		},
		"trend": map[string]any{
			"terms":   []string{"precursor", "compression", "rvol"},
			"inverts": []string{"compression"},
		},
		"exhaustion": map[string]any{
			"terms":   []string{"rvol", "precursor"},
			"inverts": []string{"rvol", "precursor"},
			"gate":    "rvolDecline",
		},
		"decline": map[string]any{
			"source":    "rvolDecline",
			"output":    "exhaustion",
			"squash":    0.0,
			"attenuate": []string{"compression"},
		},
		"joint": map[string]any{
			"source":     "ignition",
			"output":     "ignition",
			"combine":    "ratio",
			"scaleMode":  "median",
		},
	}
}
