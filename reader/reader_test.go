package reader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRejectGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.fm")
	if err := os.WriteFile(path, []byte(strings.Repeat("X", 130)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("expected invalid magic rejection")
	}
	if _, err := ReadDirectory(path); err == nil {
		t.Fatal("expected invalid directory rejection")
	}
}
func TestProvidedSample(t *testing.T) {
	path := os.Getenv("FM27_SAMPLE")
	if path == "" {
		t.Skip("set FM27_SAMPLE to run save integration test")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// These offsets describe two observed saves. Other saves can still exercise
	// the metadata checks below without assuming identical frame sizes.
	knownFrames := map[int64][]struct {
		off  int64
		want int64
	}{
		129921398: {{26, 2654}, {2680, 5024}, {7704, 1005}, {8709, 62725118}},
		133544049: {{26, 2664}, {2690, 4542}, {7232, 997}, {8229, 62990572}},
	}
	if frames, ok := knownFrames[stat.Size()]; ok {
		for _, tt := range frames {
			got, err := FrameSize(f, tt.off, stat.Size())
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("FrameSize(%d)=%d want %d", tt.off, got, tt.want)
			}
		}
	}
	s, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !SupportsGameVersion(s.GameVersion) || s.Manager != "Binbin Carpenter" || s.Club != "Pau FC" {
		t.Fatalf("wrong metadata: %+v", s)
	}
	knownSummaries := map[int64]struct{ version, date string }{
		151111254: {"27.0.1+2503222", "2027-08-20"},
		142185783: {"27.1.0+2505117", "2027-06-06"},
	}
	if want, ok := knownSummaries[stat.Size()]; ok && (s.GameVersion != want.version || s.GameDate != want.date) {
		t.Fatalf("version/date = %s/%s, want %s/%s", s.GameVersion, s.GameDate, want.version, want.date)
	}
	if stat.Size() == 140931722 && s.GameDate != "2027-06-04" {
		t.Fatalf("game date=%q want 2027-06-04", s.GameDate)
	}
	if (stat.Size() == 129921398 || stat.Size() == 133544049) && len(s.Names) != 35 {
		t.Fatalf("unexpected names / frame count: %d %d", len(s.Names), s.Frames)
	}
	if len(s.Names) > 80 {
		t.Fatalf("too many summary names: %d", len(s.Names))
	}
	if stat.Size() == 129921398 && s.Frames != 1278 || stat.Size() == 133544049 && s.Frames != 1287 {
		t.Fatalf("unexpected frame count for known save: %d", s.Frames)
	}
	if len(s.Players) != 0 {
		t.Fatal("must not fabricate player records")
	}
	directory, err := ReadDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(directory.Entries) != s.Frames {
		t.Fatalf("directory entries=%d frames=%d", len(directory.Entries), s.Frames)
	}
	gameDB := 0
	for _, entry := range directory.Entries {
		if entry.Name == "game_db" && entry.Extension == ".dat" {
			gameDB++
			if entry.Uncompressed < 100_000_000 {
				t.Fatalf("game_db unexpectedly small: %d", entry.Uncompressed)
			}
		}
	}
	if gameDB != 1 {
		t.Fatalf("game_db entries=%d", gameDB)
	}
	if stat.Size() == 133544049 {
		data, err := readGameDB(path)
		if err != nil {
			t.Fatal(err)
		}
		brice, err := findAbility(data, 34001981)
		if err != nil {
			t.Fatal(err)
		}
		if brice.CA != 112 || brice.PA != 120 || brice.HeightCM != 192 || brice.HomeReputation != 5788 || brice.CurrentReputation != 5949 || brice.WorldReputation != 3344 {
			t.Fatalf("Brice ability mismatch: %+v", brice)
		}
		mathys, err := findAbility(data, 49054454)
		if err != nil {
			t.Fatal(err)
		}
		if mathys.CA != 119 || mathys.PA != 129 || mathys.HeightCM != 176 || mathys.HomeReputation != 5559 || mathys.CurrentReputation != 5876 || mathys.WorldReputation != 2945 {
			t.Fatalf("Mathys ability mismatch: %+v", mathys)
		}
		for _, witness := range []struct {
			ability PlayerAbility
			values  map[string]uint8
		}{
			{brice, map[string]uint8{"aerial_reach": 14, "handling": 12, "first_touch": 8, "passing": 10, "technique": 8, "aggression": 7, "acceleration": 9, "consistency": 12, "dirtiness": 6}},
			{mathys, map[string]uint8{"crossing": 13, "dribbling": 13, "first_touch": 11, "passing": 13, "technique": 12, "aggression": 14, "acceleration": 13, "consistency": 12, "dirtiness": 12}},
		} {
			got := decodeAttributes(data, witness.ability.recordOffset)
			if len(got) != 52 {
				t.Fatalf("player %d has %d attributes", witness.ability.ID, len(got))
			}
			for key, want := range witness.values {
				if got[key] != want {
					t.Fatalf("player %d %s=%d want %d", witness.ability.ID, key, got[key], want)
				}
			}
		}
	}
	if stat.Size() == 140408478 {
		data, err := readGameDB(path)
		if err != nil {
			t.Fatal(err)
		}
		pools, err := locateNamePools(data)
		if err != nil {
			t.Fatal(err)
		}
		for id, want := range map[uint32]string{34001981: "Brice Maubleu", 49054454: "Mathys Tourraine"} {
			ability, err := findAbility(data, id)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := findPlayerIdentity(data, ability, pools)
			if err != nil {
				t.Fatal(err)
			}
			if identity.Name != want {
				t.Fatalf("player %d name=%q want %q", id, identity.Name, want)
			}
		}
	}
	entries, err := ReadIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	wantIndex := map[int64]struct {
		entries int
		players uint32
	}{
		129921398: {537, 77616},
		133544049: {485, 76150},
	}
	if want, ok := wantIndex[stat.Size()]; ok {
		if len(entries) != want.entries {
			t.Fatalf("index entries=%d want %d", len(entries), want.entries)
		}
		found := false
		for _, entry := range entries {
			if entry.Name == "ACTUAL_PLAYER" {
				found = true
				if entry.Count != want.players {
					t.Fatalf("ACTUAL_PLAYER=%d want %d", entry.Count, want.players)
				}
			}
		}
		if !found {
			t.Fatal("ACTUAL_PLAYER missing from index")
		}
	}
}

func TestCurrentSaveCatalog(t *testing.T) {
	path := os.Getenv("FM27_SAMPLE")
	if path == "" {
		t.Skip("set FM27_SAMPLE to run catalog integration test")
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != 140408478 && stat.Size() != 140931722 {
		t.Skip("catalog witnesses refer to two observed FM27 saves")
	}
	catalog, err := ReadPlayerCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Coverage.Indexed != 80217 || catalog.Coverage.WithID != len(catalog.Players) || catalog.Coverage.WithID < 79900 || catalog.Coverage.WithName < 79000 || catalog.Coverage.WithName > catalog.Coverage.WithID || catalog.Coverage.WithClub < 70000 || catalog.Coverage.Men+catalog.Coverage.Women != catalog.Coverage.WithID || catalog.Coverage.Women < 10000 {
		t.Fatalf("unexpected catalog coverage: %+v", catalog.Coverage)
	}
	named, withClub, men, women := 0, 0, 0, 0
	ids := make(map[uint32]bool, len(catalog.Players))
	for _, player := range catalog.Players {
		if ids[player.ID] {
			t.Fatalf("duplicate ID %d", player.ID)
		}
		ids[player.ID] = true
		if player.NameKnown {
			named++
		} else if !strings.HasPrefix(player.Name, "Joueur ID ") {
			t.Fatalf("missing name is not clearly labelled: %+v", player)
		}
		if player.Club != "" {
			withClub++
		}
		switch player.Gender {
		case "male":
			men++
		case "female":
			women++
		default:
			t.Fatalf("player %d has invalid gender %q", player.ID, player.Gender)
		}
	}
	if named != catalog.Coverage.WithName || withClub != catalog.Coverage.WithClub || men != catalog.Coverage.Men || women != catalog.Coverage.Women {
		t.Fatalf("named=%d withClub=%d men=%d women=%d coverage=%+v", named, withClub, men, women, catalog.Coverage)
	}
	for id, want := range map[uint32]struct {
		name     string
		ca       uint16
		pa       int16
		attr     string
		value    byte
		age      int
		position string
		birth    string
		club     string
	}{
		34001981: {"Brice Maubleu", 111, 120, "handling", 12, 37, "GK", "1989-12-01", "Pau FC"},
		49054454: {"Mathys Tourraine", 119, 129, "crossing", 13, 26, "DR, WBR", "2001-01-14", "Pau FC"},
	} {
		found := false
		for _, player := range catalog.Players {
			if player.ID != id {
				continue
			}
			found = true
			if player.Name != want.name || player.CA != want.ca || player.PA != want.pa || len(player.AttrsB64) != len(attributeNames) || player.Age == nil || *player.Age != want.age || player.Position != want.position || player.BirthDate != want.birth || player.Club != want.club || player.ClubID != 1446 || player.Country != "France" || player.TeamID != 1651 || player.TeamSlot == nil || *player.TeamSlot != 0 || player.Squad != "Équipe première" || player.Gender != "male" {
				t.Fatalf("player %d mismatch: %+v", id, player)
			}
			for slot, name := range attributeNames {
				if name == want.attr && player.AttrsB64[slot] != want.value {
					t.Fatalf("player %d %s=%d want %d", id, name, player.AttrsB64[slot], want.value)
				}
			}
		}
		if !found {
			t.Fatalf("player %d missing from catalog", id)
		}
	}
	if stat.Size() == 140931722 {
		pauTeams := map[uint32]int{}
		pauLabels := map[uint32]string{1651: "Équipe première", 14871: "Équipe B", 28964: "U19"}
		for _, player := range catalog.Players {
			if player.ClubID != 1446 {
				continue
			}
			if player.TeamSlot == nil || player.Country != "France" {
				t.Fatalf("Pau squad metadata missing: %+v", player)
			}
			if want := pauLabels[player.TeamID]; player.Squad != want {
				t.Fatalf("Pau team %d label %q, want %q", player.TeamID, player.Squad, want)
			}
			pauTeams[player.TeamID]++
		}
		for id, count := range map[uint32]int{1651: 21, 14871: 21, 28964: 20} {
			if pauTeams[id] != count {
				t.Fatalf("Pau team %d has %d players, want %d", id, pauTeams[id], count)
			}
		}
		if len(pauTeams) != 3 {
			t.Fatalf("unexpected Pau squads: %v", pauTeams)
		}
		leagueClubs := map[string]map[uint32]bool{}
		witnessLeagues := map[uint32]string{1651: "Ligue 2 BKT", 695: "Ligue 2 BKT", 469: "Premier League", 545: "Premier League"}
		for _, player := range catalog.Players {
			if player.ClubID == 1446 && player.League != "Ligue 2 BKT" {
				t.Fatalf("Pau league = %q, want Ligue 2 BKT", player.League)
			}
			if want, ok := witnessLeagues[player.TeamID]; ok && player.League != want {
				t.Fatalf("team %d league = %q, want %q", player.TeamID, player.League, want)
			}
			if player.League == "" {
				continue
			}
			if leagueClubs[player.League] == nil {
				leagueClubs[player.League] = map[uint32]bool{}
			}
			leagueClubs[player.League][player.ClubID] = true
		}
		for league, count := range map[string]int{"Ligue 2 BKT": 18, "Premier League": 20} {
			if len(leagueClubs[league]) != count {
				t.Fatalf("%s has %d clubs, want %d", league, len(leagueClubs[league]), count)
			}
		}
		for teamID, want := range map[uint32]string{27618: "U21", 28964: "U19"} {
			found := false
			for _, player := range catalog.Players {
				if player.TeamID != teamID {
					continue
				}
				found = true
				if player.Squad != want {
					t.Fatalf("team %d label = %q, want %q", teamID, player.Squad, want)
				}
			}
			if !found {
				t.Fatalf("team %d has no players in catalog", teamID)
			}
		}
	}
	if stat.Size() == 140931722 {
		femaleWitness := false
		for _, player := range catalog.Players {
			if player.ID == 2000101300 {
				femaleWitness = player.Name == "Sam Kerr" && player.Gender == "female"
				break
			}
		}
		if !femaleWitness {
			t.Fatal("Sam Kerr must be classified as female in the current save")
		}
	}
}
