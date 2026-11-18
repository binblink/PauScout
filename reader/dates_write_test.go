package reader

import (
	"encoding/binary"
	"testing"
)

func TestWriteBirthDateValidation(t *testing.T) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint16(data, 1)
	binary.LittleEndian.PutUint16(data[2:], 2000)
	for _, v := range []string{"1900-12-31", "2201-01-01", "2023-02-29", "2020-13-01", "bad"} {
		before := append([]byte(nil), data...)
		if _, err := writeBirthDate(data, 0, v); err == nil {
			t.Errorf("date invalide acceptée : %s", v)
		}
		for i := range data {
			if data[i] != before[i] {
				t.Fatal("une date invalide a modifié les données")
			}
		}
	}
	for _, v := range []string{"2000-02-29", "1901-01-01", "2200-12-31"} {
		if _, err := writeBirthDate(data, 0, v); err != nil {
			t.Fatal(err)
		}
		d, ok := fmDate(data, 0)
		if !ok || d.Format("2006-01-02") != v {
			t.Fatalf("date mal relue : %s", v)
		}
	}
}
