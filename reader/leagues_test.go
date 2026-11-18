package reader

import (
	"os"
	"testing"
)

func TestDomesticLeagueNames(t *testing.T) {
	for name, want := range map[string]bool{
		"Sky Bet Championship":    true,
		"UEFA Champions League":   false,
		"Spanish U19 Division 1":  false,
		"Premier League":          true,
		"Keuken Kampioen Divisie": true,
	} {
		if got := isDomesticLeague(name); got != want {
			t.Errorf("%q : %t, attendu %t", name, got, want)
		}
	}
}

func TestRecentFrenchLeagues(t *testing.T) {
	path := os.Getenv("FM27_LEAGUE_SAMPLE")
	if path == "" {
		t.Skip("FM27_LEAGUE_SAMPLE non défini")
	}
	catalog, err := ReadPlayerCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	clubs := map[string]map[uint32]bool{}
	for _, p := range catalog.Players {
		if p.Country != "France" || p.ClubID == 0 {
			continue
		}
		if clubs[p.League] == nil {
			clubs[p.League] = map[uint32]bool{}
		}
		clubs[p.League][p.ClubID] = true
	}
	for league, min := range map[string]int{"Ligue 1 McDonald's": 18, "Ligue 2 BKT": 18, "National (3e division)": 16} {
		if len(clubs[league]) < min {
			t.Errorf("%s : %d clubs, attendu au moins %d", league, len(clubs[league]), min)
		}
	}
	for id, want := range map[uint32]string{1446: "Ligue 1 McDonald's", 512: "Ligue 1 McDonald's", 541: "Ligue 2 BKT"} {
		found := false
		for _, p := range catalog.Players {
			if p.ClubID == id {
				found = true
				if p.League != want {
					t.Errorf("club %d : %q, attendu %q", id, p.League, want)
				}
				break
			}
		}
		if !found {
			t.Errorf("club %d introuvable", id)
		}
	}
	clubsNL := map[uint32]bool{}
	for _, p := range catalog.Players {
		if p.Country == "Netherlands" && p.League == "Keuken Kampioen Divisie" && p.ClubID != 0 {
			clubsNL[p.ClubID] = true
		}
	}
	if len(clubsNL) < 15 {
		t.Errorf("Keuken Kampioen Divisie : %d clubs, attendu au moins 15", len(clubsNL))
	}
}
