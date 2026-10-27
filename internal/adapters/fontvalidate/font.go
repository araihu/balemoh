// Package fontvalidate reads bounded font containers and their naming metadata.
// It validates container structure, not every glyph instruction. Browsers retain
// their own font sanitizer when loading the original immutable bytes.
package fontvalidate

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/andybalholm/brotli"
	p "github.com/araihu/balemoh/internal/application/presentation"
)

const maxDecoded = 64 << 20
const maxMetadata = 1 << 20

var validator = make(chan struct{}, 1)
var bad = &p.Error{Code: "invalid_request", Message: "Malformed font container or metadata."}
var large = &p.Error{Code: "upload_too_large", Message: "Font exceeds upload or decoded-data limits."}
var unsupported = &p.Error{Code: "unsupported_font", Message: "Use a single WOFF2, WOFF, TTF, or OTF font."}
var be = binary.BigEndian

type entry struct {
	tag                    string
	offset, size, original uint64
	transformed            bool
}

func u16(b []byte, o int) uint16 { return be.Uint16(b[o : o+2]) }
func u32(b []byte, o int) uint32 { return be.Uint32(b[o : o+4]) }
func region(b []byte, offset, size uint64) ([]byte, error) {
	if offset > uint64(len(b)) || size > uint64(len(b))-offset {
		return nil, bad
	}
	return b[offset : offset+size], nil
}
func metadataTag(tag string) bool {
	return tag == "name" || tag == "OS/2" || tag == "head" || tag == "maxp" || tag == "cmap" || tag == "hhea" || tag == "fvar"
}
func tableBounds(entries []entry, total uint64) error {
	if len(entries) == 0 || len(entries) > 256 {
		return bad
	}
	seen := map[string]bool{}
	var sum uint64
	for _, t := range entries {
		if seen[t.tag] {
			return bad
		}
		seen[t.tag] = true
		sum += t.original
		if sum > maxDecoded || t.original > maxDecoded || metadataTag(t.tag) && t.original > maxMetadata {
			return large
		}
	}
	if total > maxDecoded {
		return large
	}
	return nil
}
func sfnt(data []byte) (map[string][]byte, error) {
	if len(data) < 12 {
		return nil, bad
	}
	n := int(u16(data, 4))
	if n == 0 || n > 256 || 12+16*n > len(data) {
		return nil, bad
	}
	es := make([]entry, 0, n)
	for i := 0; i < n; i++ {
		o := 12 + i*16
		es = append(es, entry{tag: string(data[o : o+4]), offset: uint64(u32(data, o+8)), size: uint64(u32(data, o+12)), original: uint64(u32(data, o+12))})
	}
	if e := tableBounds(es, uint64(len(data))); e != nil {
		return nil, e
	}
	return extract(data, es, 12+16*n, false)
}
func extract(data []byte, es []entry, start int, compressed bool) (map[string][]byte, error) {
	ordered := slices.Clone(es)
	slices.SortFunc(ordered, func(a, b entry) int {
		if a.offset < b.offset {
			return -1
		}
		if a.offset > b.offset {
			return 1
		}
		return 0
	})
	end := uint64(start)
	tables := map[string][]byte{}
	for _, t := range ordered {
		if t.original == 0 && t.size == 0 {
			tables[t.tag] = nil
			continue
		}
		if t.offset < end || t.offset%4 != 0 {
			return nil, bad
		}
		chunk, e := region(data, t.offset, t.size)
		if e != nil {
			return nil, e
		}
		end = t.offset + t.size
		if t.size > t.original {
			return nil, bad
		}
		if compressed && t.size < t.original {
			r, e := zlib.NewReader(bytes.NewReader(chunk))
			if e != nil {
				return nil, bad
			}
			chunk, e = io.ReadAll(io.LimitReader(r, int64(t.original)+1))
			closeErr := r.Close()
			if e != nil || closeErr != nil || uint64(len(chunk)) != t.original {
				return nil, bad
			}
		}
		if uint64(len(chunk)) != t.original {
			return nil, bad
		}
		if metadataTag(t.tag) {
			tables[t.tag] = chunk
		} else {
			tables[t.tag] = nil
		}
	}
	return tables, nil
}
func woff(data []byte) (map[string][]byte, error) {
	if len(data) < 44 || u32(data, 8) != uint32(len(data)) || u16(data, 14) != 0 {
		return nil, bad
	}
	flavor := u32(data, 4)
	if flavor != 0x00010000 && flavor != 0x4f54544f {
		return nil, unsupported
	}
	n := int(u16(data, 12))
	if n == 0 || n > 256 || 44+20*n > len(data) {
		return nil, bad
	}
	if u32(data, 16) > maxDecoded || u32(data, 32) > maxMetadata {
		return nil, large
	}
	es := make([]entry, 0, n)
	var sfntSize uint64 = uint64(12 + 16*n)
	for i := 0; i < n; i++ {
		o := 44 + 20*i
		t := entry{tag: string(data[o : o+4]), offset: uint64(u32(data, o+4)), size: uint64(u32(data, o+8)), original: uint64(u32(data, o+12))}
		es = append(es, t)
		sfntSize += (t.original + 3) &^ 3
	}
	if sfntSize != uint64(u32(data, 16)) {
		return nil, bad
	}
	if e := tableBounds(es, sfntSize); e != nil {
		return nil, e
	}
	if e := extraBlocks(data, 24, 36, 44+20*n, es); e != nil {
		return nil, e
	}
	return extract(data, es, 44+20*n, true)
}

// Metadata/private blocks are never decompressed, but must stay within the file.
func extraBlocks(data []byte, meta, private, start int, entries []entry) error {
	ranges := append([]entry{}, entries...)
	for _, o := range []int{meta, private} {
		offset, size := uint64(u32(data, o)), uint64(u32(data, o+4))
		if offset == 0 && size == 0 {
			continue
		}
		if offset == 0 || size == 0 || offset%4 != 0 {
			return bad
		}
		if _, e := region(data, offset, size); e != nil {
			return e
		}
		ranges = append(ranges, entry{offset: offset, size: size})
	}
	slices.SortFunc(ranges, func(a, b entry) int {
		if a.offset < b.offset {
			return -1
		}
		return 1
	})
	end := uint64(start)
	for _, r := range ranges {
		if r.offset < end {
			return bad
		}
		end = r.offset + r.size
	}
	return nil
}

var tags = strings.Split("cmap,head,hhea,hmtx,maxp,name,OS/2,post,cvt ,fpgm,glyf,loca,prep,CFF ,VORG,EBDT,EBLC,gasp,hdmx,kern,LTSH,PCLT,VDMX,vhea,vmtx,BASE,GDEF,GPOS,GSUB,EBSC,JSTF,MATH,CBDT,CBLC,COLR,CPAL,SVG ,sbix,acnt,avar,bdat,bloc,bsln,cvar,fdsc,feat,fmtx,fvar,gvar,hsty,just,lcar,mort,morx,opbd,prop,trak,Zapf,Silf,Glat,Gloc,Feat,Sill", ",")

func base128(data []byte, pos *int) (uint64, error) {
	var value uint64
	for i := 0; i < 5; i++ {
		if *pos >= len(data) {
			return 0, bad
		}
		b := data[*pos]
		*pos++
		if i == 0 && b == 0x80 || value&0xfe000000 != 0 {
			return 0, bad
		}
		value = value<<7 | uint64(b&127)
		if b&128 == 0 {
			return value, nil
		}
	}
	return 0, bad
}
func woff2(data []byte) (map[string][]byte, error) {
	if len(data) < 48 || u32(data, 8) != uint32(len(data)) || u16(data, 14) != 0 {
		return nil, bad
	}
	flavor := u32(data, 4)
	if flavor != 0x10000 && flavor != 0x4f54544f {
		return nil, unsupported
	}
	n := int(u16(data, 12))
	if n == 0 || n > 256 {
		return nil, bad
	}
	if u32(data, 16) > maxDecoded || u32(data, 36) > maxMetadata {
		return nil, large
	}
	pos := 48
	es := make([]entry, 0, n)
	var total, original uint64
	for i := 0; i < n; i++ {
		if pos >= len(data) {
			return nil, bad
		}
		flags := data[pos]
		pos++
		index, version := int(flags&63), int(flags>>6)
		tag := ""
		if index == 63 {
			b, e := region(data, uint64(pos), 4)
			if e != nil {
				return nil, e
			}
			tag = string(b)
			pos += 4
		} else {
			tag = tags[index]
		}
		length, e := base128(data, &pos)
		if e != nil {
			return nil, e
		}
		transformed := (tag == "glyf" || tag == "loca") && version == 0 || tag == "hmtx" && version == 1
		if !transformed && version != 0 && !((tag == "glyf" || tag == "loca") && version == 3) {
			return nil, bad
		}
		size := length
		if transformed {
			size, e = base128(data, &pos)
			if e != nil || tag == "loca" && size != 0 || tag != "loca" && size == 0 {
				return nil, bad
			}
		}
		es = append(es, entry{tag: tag, offset: total, size: size, original: length, transformed: transformed})
		total += size
		original += (length + 3) &^ 3
		if total > maxDecoded || original > maxDecoded {
			return nil, large
		}
	}
	if e := tableBounds(es, original); e != nil {
		return nil, e
	}
	compressedSize := uint64(u32(data, 20))
	compressed, e := region(data, uint64(pos), compressedSize)
	if e != nil || compressedSize == 0 {
		return nil, bad
	}
	if e = extraBlocks(data, 28, 40, pos, []entry{{offset: uint64(pos), size: compressedSize}}); e != nil {
		return nil, e
	}
	// WOFF2 totalSfntSize is advisory; actual Brotli output is bounded independently.
	decoded, e := io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(compressed)), int64(total)+1))
	if e != nil || uint64(len(decoded)) != total {
		return nil, bad
	}
	tables := map[string][]byte{}
	var glyf, loca *entry
	for i, t := range es {
		if t.tag == "glyf" {
			glyf = &es[i]
		}
		if t.tag == "loca" {
			loca = &es[i]
		}
		if metadataTag(t.tag) {
			tables[t.tag] = decoded[t.offset : t.offset+t.size]
		} else {
			tables[t.tag] = nil
		}
	}
	if (glyf == nil) != (loca == nil) || glyf != nil && glyf.transformed != loca.transformed {
		return nil, bad
	}
	if glyf != nil && glyf.transformed {
		if glyf.size < 36 {
			return nil, bad
		}
		b := decoded[glyf.offset : glyf.offset+glyf.size]
		if u16(b, 0) != 0 || u16(b, 6) > 1 {
			return nil, bad
		}
		var streamBytes uint64 = 36
		for i := 8; i < 36; i += 4 {
			streamBytes += uint64(u32(b, i))
		}
		if u16(b, 2)&1 != 0 {
			streamBytes += uint64((int(u16(b, 4)) + 7) / 8)
		}
		if streamBytes > glyf.size {
			return nil, bad
		}
	}
	return tables, nil
}
func naming(data []byte) (string, error) {
	if len(data) < 6 || u16(data, 0) > 1 {
		return "", bad
	}
	count, offset := int(u16(data, 2)), int(u16(data, 4))
	if 6+12*count > len(data) || offset < 6+12*count || offset > len(data) {
		return "", bad
	}
	if u16(data, 0) == 1 {
		base := 6 + 12*count
		if base+2 > len(data) {
			return "", bad
		}
		n := int(u16(data, base))
		if base+2+4*n > offset {
			return "", bad
		}
		for i := 0; i < n; i++ {
			o := base + 2 + 4*i
			if _, e := region(data, uint64(offset)+uint64(u16(data, o+2)), uint64(u16(data, o))); e != nil {
				return "", e
			}
		}
	}
	name := ""
	score := -1
	for i := 0; i < count; i++ {
		o := 6 + 12*i
		platform, id, lang := u16(data, o), u16(data, o+6), u16(data, o+4)
		chunk, e := region(data, uint64(offset)+uint64(u16(data, o+10)), uint64(u16(data, o+8)))
		if e != nil {
			return "", e
		}
		if id != 1 && id != 16 {
			continue
		}
		text := ""
		if platform == 0 || platform == 3 {
			if len(chunk)%2 != 0 {
				return "", bad
			}
			units := make([]uint16, len(chunk)/2)
			for j := range units {
				units[j] = u16(chunk, j*2)
			}
			text = string(utf16.Decode(units))
		} else {
			continue
		}
		text = strings.TrimSpace(text)
		if len([]rune(text)) > 200 || strings.IndexFunc(text, unicode.IsControl) >= 0 {
			continue
		}
		priority := 0
		if id == 16 {
			priority += 2
		}
		if lang == 0x409 || lang == 0 {
			priority++
		}
		if text != "" && priority > score {
			name = text
			score = priority
		}
	}
	return name, nil
}
func metadata(t map[string][]byte) (p.Font, error) {
	var f p.Font
	for key, min := range map[string]int{"head": 54, "maxp": 6, "cmap": 4, "hhea": 36, "name": 6} {
		if len(t[key]) < min {
			return f, bad
		}
	}
	if u32(t["head"], 12) != 0x5f0f3cf5 || u16(t["head"], 18) < 16 || u16(t["head"], 18) > 16384 || u16(t["maxp"], 4) == 0 {
		return f, bad
	}
	_, hasGlyf := t["glyf"]
	_, hasLoca := t["loca"]
	_, hasCFF := t["CFF "]
	_, hasCFF2 := t["CFF2"]
	_, hasHmtx := t["hmtx"]
	if !hasHmtx || hasGlyf != hasLoca || !hasGlyf && !hasCFF && !hasCFF2 {
		return f, bad
	}
	var e error
	f.Family, e = naming(t["name"])
	if e != nil {
		return f, e
	}
	if f.Family == "" {
		f.Family = "unknown"
	}
	f.Weight = "unknown"
	f.Style = "normal"
	if b, ok := t["OS/2"]; ok {
		if len(b) < 68 {
			return f, bad
		}
		weight := u16(b, 4)
		if weight < 1 || weight > 1000 {
			return f, bad
		}
		f.Weight = strconv.Itoa(int(weight))
		if u16(b, 62)&1 != 0 {
			f.Style = "italic"
		}
	} else if u16(t["head"], 44)&2 != 0 {
		f.Style = "italic"
	}
	if b, ok := t["fvar"]; ok {
		if len(b) < 16 || u16(b, 0) != 1 || u16(b, 2) != 0 {
			return f, bad
		}
		offset, count, size, instances, instanceSize := int(u16(b, 4)), int(u16(b, 8)), int(u16(b, 10)), int(u16(b, 12)), int(u16(b, 14))
		if count == 0 || count > 256 || size < 20 || offset < 16 || offset+count*size > len(b) || instanceSize < 4+4*count || offset+count*size+instances*instanceSize > len(b) {
			return f, bad
		}
		seen := map[string]bool{}
		for i := 0; i < count; i++ {
			o := offset + i*size
			tag := string(b[o : o+4])
			if seen[tag] {
				return f, bad
			}
			seen[tag] = true
			min, def, max := float64(int32(u32(b, o+4)))/65536, float64(int32(u32(b, o+8)))/65536, float64(int32(u32(b, o+12)))/65536
			if math.IsNaN(def) || min > def || def > max {
				return f, bad
			}
			if tag == "wght" && min >= 1 && max <= 1000 {
				f.Weight = strconv.FormatFloat(min, 'f', -1, 64) + " " + strconv.FormatFloat(max, 'f', -1, 64)
			}
		}
		f.Variable = true
	}
	return f, nil
}

// Validate serializes bounded decompression and returns metadata derived from bytes.
func Validate(ctx context.Context, data []byte) (p.Font, error) {
	select {
	case validator <- struct{}{}:
		defer func() { <-validator }()
	case <-ctx.Done():
		return p.Font{}, ctx.Err()
	}
	if len(data) > p.MaxFontBytes {
		return p.Font{}, large
	}
	if len(data) < 4 {
		return p.Font{}, bad
	}
	var tables map[string][]byte
	var err error
	format, mime := "", ""
	switch string(data[:4]) {
	case "wOFF":
		format, mime = "woff", "font/woff"
		tables, err = woff(data)
	case "wOF2":
		format, mime = "woff2", "font/woff2"
		tables, err = woff2(data)
	case "\x00\x01\x00\x00":
		format, mime = "truetype", "font/ttf"
		tables, err = sfnt(data)
	case "OTTO":
		format, mime = "opentype", "font/otf"
		tables, err = sfnt(data)
	default:
		return p.Font{}, unsupported
	}
	if err != nil {
		return p.Font{}, err
	}
	f, err := metadata(tables)
	f.Format, f.MIME = format, mime
	return f, err
}
