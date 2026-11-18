package reader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// IndexEntry describes one observed entry in the second FMF frame.
// Count is the stored value following the label; its exact semantics
// still require validation against the binary game data.
type IndexEntry struct {
	Name  string
	Count uint32
}

func parseIndex(data []byte) ([]IndexEntry, error) {
	if len(data) < 10 || !bytes.Equal(data[:6], []byte("\x03\x01tad.")) {
		return nil, errors.New("en-tete d'index FMF invalide")
	}
	count := int(binary.LittleEndian.Uint16(data[8:10]))
	if count == 0 || count > 4096 {
		return nil, errors.New("nombre d'entrees d'index invalide")
	}
	entries := make([]IndexEntry, 0, count)
	at := 10
	for i := 0; i < count; i++ {
		if len(data)-at < 4 {
			return nil, fmt.Errorf("entree %d tronquee", i)
		}
		n := int(binary.LittleEndian.Uint32(data[at:]))
		at += 4
		if n < 1 || n > 128 || n > len(data)-at-4 {
			return nil, fmt.Errorf("longueur de l'entree %d invalide", i)
		}
		name := data[at : at+n]
		for _, c := range name {
			if c < 32 || c > 126 {
				return nil, fmt.Errorf("libelle de l'entree %d invalide", i)
			}
		}
		at += n
		entries = append(entries, IndexEntry{Name: string(name), Count: binary.LittleEndian.Uint32(data[at:])})
		at += 4
	}
	if at != len(data) {
		return nil, errors.New("octets inattendus apres l'index")
	}
	return entries, nil
}

// ReadIndex reads only the small index frame from a save. It never writes
// to the save or decompresses the large game data frame.
func ReadIndex(path string) ([]IndexEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var head [26]byte
	if _, err := f.ReadAt(head[:], 0); err != nil {
		return nil, err
	}
	if string(head[:6]) != saveMagic {
		return nil, errors.New("signature FMF absente")
	}
	first, err := FrameSize(f, 26, stat.Size())
	if err != nil {
		return nil, err
	}
	offset := int64(26) + first
	second, err := FrameSize(f, offset, stat.Size())
	if err != nil {
		return nil, err
	}
	data, err := decompressSection(f, Frame{Offset: offset, Compressed: second})
	if err != nil {
		return nil, err
	}
	return parseIndex(data)
}
