package fontvalidate

import (
	"bytes"
	"compress/zlib"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/andybalholm/brotli"
	p "github.com/araihu/balemoh/internal/application/presentation"
	"golang.org/x/image/font/gofont/goregular"
)

func originalEntries() []entry {
	b := goregular.TTF
	out := []entry{}
	for i := 0; i < int(u16(b, 4)); i++ {
		o := 12 + 16*i
		out = append(out, entry{tag: string(b[o : o+4]), offset: uint64(u32(b, o+8)), size: uint64(u32(b, o+12)), original: uint64(u32(b, o+12))})
	}
	return out
}
func baseBytes(v uint64) []byte {
	out := []byte{byte(v & 127)}
	for v >>= 7; v > 0; v >>= 7 {
		out = append([]byte{byte(v&127) | 128}, out...)
	}
	return out
}
func asWOFF(t *testing.T) []byte {
	t.Helper()
	es := originalEntries()
	b := make([]byte, 44+20*len(es))
	copy(b, "wOFF")
	be.PutUint32(b[4:], 0x10000)
	be.PutUint16(b[12:], uint16(len(es)))
	var size uint32 = uint32(12 + 16*len(es))
	for i, e := range es {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		chunk := goregular.TTF[e.offset : e.offset+e.size]
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		_, _ = w.Write(chunk)
		_ = w.Close()
		if z.Len() < len(chunk) {
			chunk = z.Bytes()
		}
		o := 44 + 20*i
		copy(b[o:], e.tag)
		be.PutUint32(b[o+4:], uint32(len(b)))
		be.PutUint32(b[o+8:], uint32(len(chunk)))
		be.PutUint32(b[o+12:], uint32(e.original))
		size += uint32((e.original + 3) &^ 3)
		b = append(b, chunk...)
	}
	be.PutUint32(b[8:], uint32(len(b)))
	be.PutUint32(b[16:], size)
	return b
}
func asWOFF2(t *testing.T) []byte {
	t.Helper()
	es := originalEntries()
	b := make([]byte, 48)
	copy(b, "wOF2")
	be.PutUint32(b[4:], 0x10000)
	be.PutUint16(b[12:], uint16(len(es)))
	var uncompressed bytes.Buffer
	var size uint32 = uint32(12 + 16*len(es))
	for _, e := range es {
		flag := byte(63)
		if e.tag == "glyf" || e.tag == "loca" {
			flag |= 3 << 6
		}
		b = append(b, flag)
		b = append(b, []byte(e.tag)...)
		b = append(b, baseBytes(e.original)...)
		_, _ = uncompressed.Write(goregular.TTF[e.offset : e.offset+e.size])
		size += uint32((e.original + 3) &^ 3)
	}
	var compressed bytes.Buffer
	w := brotli.NewWriter(&compressed)
	_, _ = w.Write(uncompressed.Bytes())
	_ = w.Close()
	be.PutUint32(b[16:], size)
	be.PutUint32(b[20:], uint32(compressed.Len()))
	b = append(b, compressed.Bytes()...)
	be.PutUint32(b[8:], uint32(len(b)))
	return b
}
func TestContainersDeriveMetadataFromBytes(t *testing.T) {
	for _, c := range []struct {
		name, format string
		data         []byte
	}{{"ttf", "truetype", goregular.TTF}, {"woff", "woff", asWOFF(t)}, {"woff2", "woff2", asWOFF2(t)}} {
		t.Run(c.name, func(t *testing.T) {
			before := slices.Clone(c.data)
			f, e := Validate(t.Context(), c.data)
			if e != nil || f.Format != c.format || f.Family != "Go" || f.Weight != "400" || f.Style != "normal" || f.Variable {
				t.Fatalf("metadata %+v %v", f, e)
			}
			if !bytes.Equal(c.data, before) {
				t.Fatal("mutated original")
			}
		})
	}
}
func TestRejectsMalformedBoundsBombsAndCollections(t *testing.T) {
	largeWOFF := asWOFF(t)
	be.PutUint32(largeWOFF[16:], maxDecoded+1)
	overlap := slices.Clone(goregular.TTF)
	be.PutUint32(overlap[20:], 0)
	hugeWOFF2 := asWOFF2(t)
	be.PutUint32(hugeWOFF2[16:], maxDecoded+1)
	unknown := []byte("ttcf0000")
	for _, c := range []struct {
		name, code string
		data       []byte
	}{{"truncated", "invalid_request", goregular.TTF[:15]}, {"overlap", "invalid_request", overlap}, {"woff bomb", "upload_too_large", largeWOFF}, {"woff2 bomb", "upload_too_large", hugeWOFF2}, {"collection", "unsupported_font", unknown}, {"raw limit", "upload_too_large", make([]byte, p.MaxFontBytes+1)}} {
		t.Run(c.name, func(t *testing.T) {
			_, e := Validate(t.Context(), c.data)
			var problem *p.Error
			if !errors.As(e, &problem) || problem.Code != c.code {
				t.Fatalf("got %v want %s", e, c.code)
			}
		})
	}
}
func TestVariableMetadataUsesDefaultAxes(t *testing.T) {
	tables, e := sfnt(goregular.TTF)
	if e != nil {
		t.Fatal(e)
	}
	fvar := make([]byte, 36)
	be.PutUint16(fvar, 1)
	be.PutUint16(fvar[4:], 16)
	be.PutUint16(fvar[8:], 1)
	be.PutUint16(fvar[10:], 20)
	be.PutUint16(fvar[14:], 8)
	copy(fvar[16:], "wght")
	be.PutUint32(fvar[20:], 100<<16)
	be.PutUint32(fvar[24:], 400<<16)
	be.PutUint32(fvar[28:], 900<<16)
	tables["fvar"] = fvar
	f, e := metadata(tables)
	if e != nil || !f.Variable || f.Weight != "100 900" {
		t.Fatal(f, e)
	}
	be.PutUint32(fvar[24:], 1000<<16)
	if _, e = metadata(tables); e == nil {
		t.Fatal("invalid axis bounds accepted")
	}
}
func FuzzContainersNeverPanic(f *testing.F) {
	f.Add(goregular.TTF)
	f.Add([]byte("wOF2"))
	f.Add([]byte("wOFF"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > p.MaxFontBytes {
			return
		}
		_, _ = Validate(t.Context(), data)
	})
}

func TestWOFF2DoesNotTrustAdvisorySfntSize(t *testing.T) {
	data := asWOFF2(t)
	be.PutUint32(data[16:], 1)
	if _, e := Validate(t.Context(), data); e != nil {
		t.Fatal("advisory size rejected", e)
	}
}

func TestOpenTypeCFFContainer(t *testing.T) {
	data, e := os.ReadFile("testdata/CFFTest.otf")
	if e != nil {
		t.Fatal(e)
	}
	font, e := Validate(t.Context(), data)
	if e != nil || font.Format != "opentype" || font.Family == "unknown" {
		t.Fatal("CFF metadata", font, e)
	}
}

func TestRealVariableWOFF2KeepsMetadata(t *testing.T) {
	data, e := os.ReadFile("testdata/Roboto-latin-variable.woff2")
	if e != nil {
		t.Fatal(e)
	}
	font, e := Validate(t.Context(), data)
	if e != nil || !font.Variable || font.Weight != "100 900" || font.Format != "woff2" || font.Family != "Roboto" {
		t.Fatal("variable WOFF2", font, e)
	}
}
