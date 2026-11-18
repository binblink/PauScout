package reader

import (
	"sort"
	"strings"
)

var positionNames = [...]string{
	"GK", "SW", "DL", "DC", "DR", "DM", "ML", "MC", "MR",
	"AML", "AMC", "AMR", "STC", "WBL", "WBR",
}

// displayPositions lists natural and accomplished positions, strongest first.
func displayPositions(ratings []byte) string {
	if len(ratings) != len(positionNames) {
		return ""
	}
	indices := make([]int, 0, len(positionNames))
	for i, rating := range ratings {
		if rating >= 15 {
			indices = append(indices, i)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return ratings[indices[i]] > ratings[indices[j]]
	})
	positions := make([]string, len(indices))
	for i, index := range indices {
		positions[i] = positionNames[index]
	}
	return strings.Join(positions, ", ")
}
