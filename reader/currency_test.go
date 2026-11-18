package reader

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

func TestEuroRateOnProvidedSave(t *testing.T) {
	path := os.Getenv("FM27_LEAGUE_SAMPLE")
	if path == "" {
		t.Skip("FM27_LEAGUE_SAMPLE non défini")
	}
	catalog, err := ReadPlayerCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.EuroRate == nil {
		t.Fatal("taux euro absent")
	}
	expected := map[uint32]int64{45107642: 9789255, 2000409768: 4010999}
	for _, p := range catalog.Players {
		want, ok := expected[p.ID]
		if !ok {
			continue
		}
		if p.Value == nil {
			t.Fatalf("valeur absente pour %d", p.ID)
		}
		got := int64(math.Round(float64(*p.Value) * *catalog.EuroRate))
		if got != want {
			t.Errorf("%s : %d €, attendu %d €", p.Name, got, want)
		}
		delete(expected, p.ID)
	}
	if len(expected) != 0 {
		t.Fatalf("joueurs absents : %v", expected)
	}
}

func TestEuroRateRequiresUniqueRecord(t *testing.T) {
	if euroRate(nil) != nil {
		t.Fatal("taux inventé à partir de données absentes")
	}
	data := append([]byte("\x0d\x00\x00\x00European Euro\x04\x00\x00\x00Euro\x00\x00\x00\x00\x03\x00\x00\x00\xe2\x82\xac"), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(data[len(data)-4:], math.Float32bits(1.2))
	if euroRate(data) == nil {
		t.Fatal("taux valide non reconnu")
	}
	if euroRate(append(append([]byte(nil), data...), data...)) != nil {
		t.Fatal("taux ambigu accepté")
	}
}
