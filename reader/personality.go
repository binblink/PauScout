package reader

var personalityNames = [...]string{
	"adaptability", "ambition", "loyalty", "pressure",
	"professionalism", "sportsmanship", "temperament", "controversy",
}

func personalityValues(identity PlayerIdentity) map[string]uint8 {
	values := make(map[string]uint8, len(personalityNames))
	for i, key := range personalityNames {
		values[key] = identity.Personality[i]
	}
	return values
}
