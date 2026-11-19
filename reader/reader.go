// Package reader is an independent, read-only reader for the FM27 FMF container.
// It handles Zstandard framing, FMF section headers and save summary metadata.
// It deliberately does not guess the internal game_db/player layout.
package reader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"pauscout.local/app/reader/zstd"
)

const (
	saveMagic       = "\x02\x01fmf."
	zstdMagic       = 0xfd2fb528
	maxSectionBytes = 8 * 1024 * 1024 // metadata only
	maxFrames       = 4096
	lastTestedBuild = "27.1.0+2505117"
)

var versionRE = regexp.MustCompile(`\b\d+\.\d+\.\d+\+\d+\b`)

// SupportsWriteVersion identifies the builds with validated editing support.
// Read and ReadPlayerCatalog try compatible structures independently of the build.
func SupportsWriteVersion(version string) bool {
	switch version {
	case "27.0.1+2503222", lastTestedBuild:
		return true
	}
	return false
}

func compatibilityWarning(version string) string {
	if SupportsWriteVersion(version) {
		return ""
	}
	message := "Le build FM de cette sauvegarde reste inconnu."
	if version != "" {
		message = fmt.Sprintf("Le build FM %s n’a pas été testé avec PauScout.", version)
	}
	return message + " PauScout tente de lire la sauvegarde en consultation seule. Le dernier build testé pour l’édition est le " + lastTestedBuild + "."
}

type Frame struct {
	Offset       int64 `json:"-"`
	Compressed   int64 `json:"compressed"`
	Uncompressed int   `json:"uncompressed,omitempty"`
}

type Summary struct {
	File                 string           `json:"source"`
	GameVersion          string           `json:"version"`
	CompatibilityWarning string           `json:"compatibility_warning,omitempty"`
	GameDate             string           `json:"game_date,omitempty"`
	Manager              string           `json:"manager"`
	Club                 string           `json:"club"`
	SaveTitle            string           `json:"save_title,omitempty"`
	Names                []string         `json:"names"`
	Frames               int              `json:"sections_count"`
	Format               string           `json:"format"`
	Bytes                int64            `json:"save_bytes"`
	Note                 string           `json:"note"`
	Players              []CatalogPlayer  `json:"players"`
	Staff                []CatalogStaff   `json:"staff,omitempty"`
	Coverage             *CatalogCoverage `json:"coverage,omitempty"`
	AttributeOrder       []string         `json:"attribute_order,omitempty"`
	StaffAttributeOrder  []string         `json:"staff_attribute_order,omitempty"`
	StaffRoleOrder       []string         `json:"staff_role_order,omitempty"`
	EuroRate             *float64         `json:"euro_rate,omitempty"`
}

func parseLengthString(data []byte, at int) (s string, next int, ok bool) {
	if at+4 > len(data) {
		return "", at, false
	}
	n := int(binary.LittleEndian.Uint32(data[at:]))
	if n < 1 || n > 160 || at+4+n > len(data) {
		return "", at, false
	}
	raw := data[at+4 : at+4+n]
	if !utf8.Valid(raw) {
		return "", at, false
	}
	for _, r := range string(raw) {
		if r < 32 || r == 127 {
			return "", at, false
		}
	}
	return string(raw), at + 4 + n, true
}

// FrameSize returns the compressed length of one Zstd frame starting at pos.
// It doesn't need to decompress any bytes, and it checks every block boundary.
func FrameSize(r io.ReaderAt, pos int64, total int64) (int64, error) {
	if pos < 0 || pos+5 > total {
		return 0, io.ErrUnexpectedEOF
	}
	header := make([]byte, 18)
	n, err := r.ReadAt(header, pos)
	if err != nil && err != io.EOF {
		return 0, err
	}
	if n < 6 || binary.LittleEndian.Uint32(header) != zstdMagic {
		return 0, errors.New("trame Zstandard absente")
	}
	d := header[4]
	if d&0x08 != 0 {
		return 0, errors.New("en-tete Zstandard reserve")
	}
	extra := 0
	if d&0x20 == 0 {
		extra++
	}
	switch d & 3 {
	case 1:
		extra++
	case 2:
		extra += 2
	case 3:
		extra += 4
	}
	switch d >> 6 {
	case 0:
		if d&0x20 != 0 {
			extra++
		}
	case 1:
		extra += 2
	case 2:
		extra += 4
	case 3:
		extra += 8
	}
	at := pos + 5 + int64(extra)
	if at > total {
		return 0, io.ErrUnexpectedEOF
	}
	for i := 0; i < 1000000; i++ {
		if at+3 > total {
			return 0, io.ErrUnexpectedEOF
		}
		var bh [3]byte
		if _, err := r.ReadAt(bh[:], at); err != nil {
			return 0, err
		}
		h := uint32(bh[0]) | uint32(bh[1])<<8 | uint32(bh[2])<<16
		last := h&1 == 1
		kind := (h >> 1) & 3
		size := int64(h >> 3)
		at += 3
		if kind == 3 {
			return 0, errors.New("bloc Zstandard reserve")
		}
		if kind == 1 {
			size = 1
		}
		if at+size > total {
			return 0, io.ErrUnexpectedEOF
		}
		at += size
		if last {
			if d&4 != 0 {
				at += 4
			}
			if at > total {
				return 0, io.ErrUnexpectedEOF
			}
			return at - pos, nil
		}
	}
	return 0, errors.New("trop de blocs Zstandard")
}

func decompressSection(f *os.File, fr Frame) ([]byte, error) {
	r := zstd.NewReader(io.NewSectionReader(f, fr.Offset, fr.Compressed))
	data, err := io.ReadAll(io.LimitReader(r, maxSectionBytes+1))
	if err != nil {
		return nil, fmt.Errorf("decompression: %w", err)
	}
	if len(data) > maxSectionBytes {
		return nil, errors.New("section trop grande")
	}
	if !bytes.HasPrefix(data, []byte("\x03\x01tad.")) {
		return nil, errors.New("section FMF inattendue")
	}
	return data, nil
}

// Read parses a .fm file independently of external packages or services.
// It provides the metadata and names present in the save summary, NOT the player table.
func Read(path string) (*Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() < 100 {
		return nil, errors.New("fichier trop court")
	}
	var head [26]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return nil, err
	}
	if string(head[:6]) != saveMagic {
		return nil, errors.New("signature FMF absente")
	}
	s := &Summary{File: stat.Name(), Format: "FMF / Zstandard", Bytes: stat.Size(), Note: "Lecture seule du fichier FM.", Players: []CatalogPlayer{}, Names: []string{}}
	at := int64(26)
	for i := 0; i < maxFrames && at+5 <= stat.Size(); i++ {
		size, e := FrameSize(f, at, stat.Size())
		if e != nil {
			break
		}
		frame := Frame{Offset: at, Compressed: size}
		if i == 2 {
			b, e := decompressSection(f, frame)
			if e != nil {
				return nil, fmt.Errorf("resume de partie: %w", e)
			}
			parseSaveSummary(s, b)
		}
		at += size
		s.Frames++
	}
	if s.Frames < 3 {
		return nil, errors.New("sections FMF manquantes")
	}
	directory, err := ReadDirectory(path)
	if err != nil {
		return nil, fmt.Errorf("repertoire de sauvegarde: %w", err)
	}
	if len(directory.Entries) != s.Frames {
		return nil, errors.New("nombre de sections incoherent avec le repertoire")
	}
	s.SaveTitle = directory.Title
	return s, nil
}

func parseSaveSummary(s *Summary, b []byte) {
	version, versionEnd, ok := summaryVersionField(b)
	if !ok || versionRE.FindString(version) != version {
		version, versionEnd = "", 0
		if loc := versionRE.FindIndex(b); loc != nil {
			version, versionEnd = string(b[loc[0]:loc[1]]), loc[1]
		}
	}
	s.GameVersion = version
	s.CompatibilityWarning = compatibilityWarning(version)
	if date, ok := summaryGameDate(b, s.GameVersion); ok {
		s.GameDate = date.Format("2006-01-02")
	}
	// Stored summary starts with a text-list of competition labels. After that list,
	// two length-prefixed strings are the manager and current club display names.
	if versionEnd != 0 {
		p := versionEnd
		if p+4 <= len(b) {
			count := int(binary.LittleEndian.Uint32(b[p:]))
			p += 4
			if count > 0 && count < 120 {
				ok := true
				for i := 0; i < count; i++ {
					_, np, v := parseLengthString(b, p)
					if !v {
						ok = false
						break
					}
					p = np
				}
				if ok {
					// Preceding the two strings is a small, edition-dependent preamble.
					for off := p; off < p+32 && off+8 < len(b); off++ {
						m, next, a := parseLengthString(b, off)
						if !a || len(m) > 80 || len(m) < 3 {
							continue
						}
						c, _, bb := parseLengthString(b, next)
						if bb && len(c) < 80 && len(c) > 2 && !strings.Contains(c, ",") && strings.Contains(m, " ") {
							s.Manager = m
							s.Club = c
							break
						}
					}
				}
			}
		}
	}
	// The summary also stores one list of on-screen display names. These are NOT
	// all players; some may be abbreviated and refer to historical match events.
	best := []string{}
	for p := 0; p+8 < len(b); p++ {
		count := int(binary.LittleEndian.Uint32(b[p:]))
		if count < 9 || count > 80 {
			continue
		}
		names := make([]string, 0, count)
		next := p + 4
		valid := true
		for i := 0; i < count; i++ {
			name, np, ok := parseLengthString(b, next)
			if !ok || len([]rune(name)) < 3 || !strings.Contains(name, " ") || strings.ContainsAny(name, "\x00,;") {
				valid = false
				break
			}
			names = append(names, name)
			next = np
		}
		if valid && len(names) > len(best) {
			best = names
		}
	}
	s.Names = best
}
