package reader

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"pauscout.local/app/reader/zstd"
)

const maxDirectoryBytes = 1024 * 1024

// DirectoryEntry describes a compressed section listed in the FMF footer.
// Offset is absolute from the beginning of the save file.
type DirectoryEntry struct {
	Group        string
	Name         string
	Extension    string
	Offset       int64
	Compressed   int64
	Uncompressed uint64
}

type Directory struct {
	Title   string
	Entries []DirectoryEntry
}

type directoryCursor struct {
	data []byte
	at   int
}

func (c *directoryCursor) word() (uint32, error) {
	if len(c.data)-c.at < 4 {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint32(c.data[c.at:])
	c.at += 4
	return v, nil
}

func (c *directoryCursor) string() (string, error) {
	s, next, ok := parseLengthString(c.data, c.at)
	if !ok {
		return "", errors.New("chaine de repertoire FMF invalide")
	}
	c.at = next
	return s, nil
}

func (c *directoryCursor) entries(group string, count uint32, dst *[]DirectoryEntry) error {
	if count == 0 || count > maxFrames || len(*dst)+int(count) > maxFrames {
		return errors.New("nombre de sections FMF invalide")
	}
	for i := uint32(0); i < count; i++ {
		name, err := c.string()
		if err != nil {
			return fmt.Errorf("section %d: %w", i, err)
		}
		ext, err := c.string()
		if err != nil {
			return fmt.Errorf("extension de section %d: %w", i, err)
		}
		if len(c.data)-c.at < 40 {
			return io.ErrUnexpectedEOF
		}
		offset := binary.LittleEndian.Uint64(c.data[c.at:])
		compressed := binary.LittleEndian.Uint64(c.data[c.at+8:])
		uncompressed := binary.LittleEndian.Uint64(c.data[c.at+16:])
		c.at += 40 // two remaining u64 fields have unknown meanings
		if offset > uint64(^uint64(0)>>1)-26 || compressed == 0 || compressed > uint64(^uint64(0)>>1) {
			return errors.New("position ou taille de section FMF invalide")
		}
		*dst = append(*dst, DirectoryEntry{Group: group, Name: name, Extension: ext, Offset: int64(offset) + 26, Compressed: int64(compressed), Uncompressed: uncompressed})
	}
	return nil
}

func parseDirectory(data []byte) (*Directory, error) {
	c := directoryCursor{data: data}
	title, err := c.string()
	if err != nil {
		return nil, err
	}
	topCount, err := c.word()
	if err != nil {
		return nil, err
	}
	d := &Directory{Title: title}
	if err := c.entries("", topCount, &d.Entries); err != nil {
		return nil, err
	}
	groupCount, err := c.word()
	if err != nil {
		return nil, err
	}
	if groupCount > 16 {
		return nil, errors.New("trop de groupes FMF")
	}
	for i := uint32(0); i < groupCount; i++ {
		group, err := c.string()
		if err != nil {
			return nil, err
		}
		count, err := c.word()
		if err != nil {
			return nil, err
		}
		if err := c.entries(group, count, &d.Entries); err != nil {
			return nil, err
		}
	}
	end, err := c.word()
	if err != nil || end != 0 || c.at != len(data) {
		return nil, errors.New("fin de repertoire FMF invalide")
	}
	return d, nil
}

// ReadDirectory validates the complete FMF frame directory without modifying
// the save or decompressing large game sections.
func ReadDirectory(path string) (*Directory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() < 35 {
		return nil, errors.New("fichier FMF trop court")
	}
	var head [26]byte
	if _, err := f.ReadAt(head[:], 0); err != nil {
		return nil, err
	}
	if string(head[:6]) != saveMagic {
		return nil, errors.New("signature FMF absente")
	}
	ptr := binary.LittleEndian.Uint64(head[9:17])
	if ptr > uint64(stat.Size()-18) {
		return nil, errors.New("position du repertoire FMF invalide")
	}
	suffix := int64(ptr) + 9
	if suffix < 26 || suffix+9 >= stat.Size() {
		return nil, errors.New("suffixe FMF invalide")
	}
	var footerHead [9]byte
	if _, err := f.ReadAt(footerHead[:], suffix); err != nil {
		return nil, err
	}
	if string(footerHead[:6]) != saveMagic {
		return nil, errors.New("signature de repertoire FMF absente")
	}
	footerSize, err := FrameSize(f, suffix+9, stat.Size())
	if err != nil || footerSize != stat.Size()-suffix-9 {
		return nil, errors.New("trame de repertoire FMF invalide")
	}
	r := zstd.NewReader(io.NewSectionReader(f, suffix+9, footerSize))
	data, err := io.ReadAll(io.LimitReader(r, maxDirectoryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("decompression du repertoire: %w", err)
	}
	if len(data) > maxDirectoryBytes {
		return nil, errors.New("repertoire FMF trop grand")
	}
	d, err := parseDirectory(data)
	if err != nil {
		return nil, err
	}
	frames := make(map[int64]int64, len(d.Entries))
	for at := int64(26); at < suffix; {
		if len(frames) >= maxFrames {
			return nil, errors.New("trop de trames FMF")
		}
		size, err := FrameSize(f, at, suffix)
		if err != nil {
			return nil, fmt.Errorf("trame FMF a %d: %w", at, err)
		}
		frames[at] = size
		at += size
	}
	if len(frames) != len(d.Entries) {
		return nil, errors.New("nombre de trames FMF et d'entrees different")
	}
	for _, entry := range d.Entries {
		size, ok := frames[entry.Offset]
		if !ok || size != entry.Compressed {
			return nil, fmt.Errorf("section %s%s absente ou taille incoherente", entry.Name, entry.Extension)
		}
		delete(frames, entry.Offset)
	}
	return d, nil
}
