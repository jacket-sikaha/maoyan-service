package pkg

import (
	"testing"
)

func TestStonefontDecode(t *testing.T) {
	mapping := map[int]string{
		0xE8D7: "3",
		0xE99C: "8",
		0xF85E: "5",
	}
	tests := []struct {
		input    string
		expected float64
	}{
		{"&#xE8D7;&#xE99C;", 38},
		{"&#xE8D7;&#xE99C;.&#xF85E;", 38.5},
		{"123", 123},
		{"45.67", 45.67},
		{"&#xE8D7;5", 35},
		{"", 0},
	}
	for _, tt := range tests {
		result := stonefontDecode(tt.input, mapping)
		if result != tt.expected {
			t.Errorf("stonefontDecode(%q) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestArialSelfMatch(t *testing.T) {
	// 确保 Arial 基准轮廓已加载
	loadArialRef()

	for d := 0; d <= 9; d++ {
		ref := refContours[d]
		if len(ref) == 0 {
			t.Errorf("digit %d has no reference contour", d)
			continue
		}
		bestDist := 1e308
		bestDigit := -1
		for dd := 0; dd <= 9; dd++ {
			if len(refContours[dd]) == 0 {
				continue
			}
			dist := bidirectionalChamferDistance(ref, refContours[dd])
			if dist < bestDist {
				bestDist = dist
				bestDigit = dd
			}
		}
		if bestDigit != d {
			t.Errorf("Arial digit %d matched as %d (dist=%.6f)", d, bestDigit, bestDist)
		} else {
			t.Logf("Arial digit %d self-match OK (dist=%.6f)", d, bestDist)
		}
	}
}
