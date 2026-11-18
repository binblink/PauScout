package reader

// Attribute names follow the validated FM27 game_db byte order. The two foot
// strength bytes at raw slots 24 and 25 are handled separately by the game.
var attributeNames = [...]string{
	"crossing", "dribbling", "finishing", "heading", "long_shots", "marking",
	"off_the_ball", "passing", "penalty_taking", "tackling", "vision", "handling",
	"aerial_reach", "command_of_area", "communication", "kicking", "throwing",
	"anticipation", "decisions", "one_on_ones", "positioning", "reflexes",
	"first_touch", "technique", "flair", "corners", "teamwork", "work_rate",
	"long_throws", "eccentricity", "rushing_out", "punching", "acceleration",
	"free_kick_taking", "strength", "stamina", "pace", "jumping_reach",
	"leadership", "dirtiness", "balance", "bravery", "consistency", "aggression",
	"agility", "important_matches", "injury_proneness", "versatility",
	"natural_fitness", "determination", "composure", "concentration",
}

// AttributeOrder is the order of the compact values returned in attrs_b64.
func AttributeOrder() []string {
	return append([]string(nil), attributeNames[:]...)
}

func displayAttribute(raw byte) uint8 {
	v := (uint16(raw) + 2) / 5
	if v < 1 {
		return 1
	}
	return uint8(v)
}

func decodeAttributes(data []byte, recordOffset int) map[string]uint8 {
	values := make(map[string]uint8, len(attributeNames))
	for i, name := range attributeNames {
		slot := i
		if i >= 24 {
			slot += 2 // left and right foot occupy raw slots 24 and 25
		}
		values[name] = displayAttribute(data[recordOffset+39+slot])
	}
	return values
}

// ReadAttributesByID returns the 52 displayed player attributes for a unique
// database ID when its player record can be matched unambiguously.
func ReadAttributesByID(path string, id uint32) (map[string]uint8, error) {
	data, err := readGameDB(path)
	if err != nil {
		return nil, err
	}
	ability, err := findAbility(data, id)
	if err != nil {
		return nil, err
	}
	return decodeAttributes(data, ability.recordOffset), nil
}
