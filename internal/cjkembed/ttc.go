package cjkembed

import (
	"encoding/binary"
	"fmt"
)

const ttcTag = "ttcf"

// extractCollectionFace returns a standalone sfnt (TTF/OTF) for the given
// collection index. Non-collection files are returned as-is when index is 0.
// gogpu's LoadFont always parses face 0, so a Super OTC (JP+KR+SC+TC+HK)
// would otherwise keep every language in the process.
func extractCollectionFace(data []byte, index int) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("empty font")
	}
	if string(data[:4]) != ttcTag {
		if index != 0 {
			return nil, fmt.Errorf("face %d requested from a single font", index)
		}
		return data, nil
	}
	if len(data) < 12 {
		return nil, fmt.Errorf("truncated ttc header")
	}
	numFonts := int(binary.BigEndian.Uint32(data[8:12]))
	if index < 0 || index >= numFonts {
		return nil, fmt.Errorf("face %d out of range 0..%d", index, numFonts-1)
	}
	offPos := 12 + 4*index
	if offPos+4 > len(data) {
		return nil, fmt.Errorf("truncated ttc offset table")
	}
	fontOff := int(binary.BigEndian.Uint32(data[offPos : offPos+4]))
	return extractSfnt(data, fontOff)
}

func extractSfnt(data []byte, fontOff int) ([]byte, error) {
	if fontOff < 0 || fontOff+12 > len(data) {
		return nil, fmt.Errorf("sfnt offset %d out of range", fontOff)
	}
	numTables := int(binary.BigEndian.Uint16(data[fontOff+4 : fontOff+6]))
	if numTables <= 0 || numTables > 128 {
		return nil, fmt.Errorf("invalid table count %d", numTables)
	}
	dirOff := fontOff + 12
	dirEnd := dirOff + 16*numTables
	if dirEnd > len(data) {
		return nil, fmt.Errorf("truncated sfnt table directory")
	}

	type table struct {
		tag      [4]byte
		checksum uint32
		offset   int
		length   int
	}
	tables := make([]table, numTables)
	total := 12 + 16*numTables
	for i := 0; i < numTables; i++ {
		rec := data[dirOff+16*i : dirOff+16*(i+1)]
		copy(tables[i].tag[:], rec[:4])
		tables[i].checksum = binary.BigEndian.Uint32(rec[4:8])
		tables[i].offset = int(binary.BigEndian.Uint32(rec[8:12]))
		tables[i].length = int(binary.BigEndian.Uint32(rec[12:16]))
		if tables[i].length < 0 || tables[i].offset < 0 || tables[i].offset+tables[i].length > len(data) {
			return nil, fmt.Errorf("table %q is out of range", tables[i].tag)
		}
		total += (tables[i].length + 3) &^ 3
	}

	out := make([]byte, total)
	copy(out[:12], data[fontOff:fontOff+12])
	cursor := 12 + 16*numTables
	for i, t := range tables {
		padded := (t.length + 3) &^ 3
		copy(out[cursor:cursor+t.length], data[t.offset:t.offset+t.length])
		rec := out[12+16*i : 12+16*(i+1)]
		copy(rec[:4], t.tag[:])
		binary.BigEndian.PutUint32(rec[4:8], t.checksum)
		binary.BigEndian.PutUint32(rec[8:12], uint32(cursor))
		binary.BigEndian.PutUint32(rec[12:16], uint32(t.length))
		cursor += padded
	}
	return out, nil
}
