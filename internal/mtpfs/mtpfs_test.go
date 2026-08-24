package mtpfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanJoinParent(t *testing.T) {
	if got := Clean("Internal/DCIM"); got != "/Internal/DCIM" {
		t.Fatalf("clean = %q", got)
	}
	if got := Join("/", "Internal", "DCIM"); got != "/Internal/DCIM" {
		t.Fatalf("join = %q", got)
	}
	if got := Parent("/Internal/DCIM"); got != "/Internal" {
		t.Fatalf("parent = %q", got)
	}
	if got := Parent("/"); got != "/" {
		t.Fatalf("parent of / = %q", got)
	}
	if got := Base("/Internal/a.txt"); got != "a.txt" {
		t.Fatalf("base = %q", got)
	}
}

func TestEncodeDecodeCommand(t *testing.T) {
	b := encodeCommand(opOpenSession, 7, []uint32{1})
	h, err := decodeHeader(b)
	if err != nil {
		t.Fatal(err)
	}
	if h.typ != containerCommand || h.code != opOpenSession || h.transactionID != 7 {
		t.Fatalf("header = %+v", h)
	}
	if h.length != uint32(len(b)) {
		t.Fatalf("length = %d want %d", h.length, len(b))
	}
	params := paramsFrom(b[12:])
	if len(params) != 1 || params[0] != 1 {
		t.Fatalf("params = %v", params)
	}
}

func TestObjectInfoRoundTrip(t *testing.T) {
	raw := encodeObjectInfo(0x00010001, 0, fmtAssociation, 0, "DCIM", assocGenericFolder)
	info, err := parseObjectInfo(raw)
	if err != nil {
		t.Fatal(err)
	}
	if info.filename != "DCIM" || info.format != fmtAssociation || info.storageID != 0x00010001 {
		t.Fatalf("info = %+v", info)
	}
	raw = encodeObjectInfo(1, 2, fmtUndefined, 1234, "写真.jpg", 0)
	info, err = parseObjectInfo(raw)
	if err != nil {
		t.Fatal(err)
	}
	if info.filename != "写真.jpg" || info.size != 1234 || info.parent != 2 {
		t.Fatalf("file info = %+v", info)
	}
}

func TestParseStorageAndDeviceInfo(t *testing.T) {
	var w byteWriter
	w.u16(1)
	w.u16(2)
	w.u16(0)
	w.u64(0)
	w.u64(0)
	w.u32(0)
	w.mtpString("内部ストレージ")
	w.mtpString("vol0")
	st, err := parseStorageInfo(w.b)
	if err != nil {
		t.Fatal(err)
	}
	if st.description != "内部ストレージ" || st.volume != "vol0" {
		t.Fatalf("storage = %+v", st)
	}

	var d byteWriter
	d.u16(100)
	d.u32(6)
	d.u16(100)
	d.mtpString("microsoft.com: 1.0")
	d.u16(0)
	for i := 0; i < 5; i++ {
		d.u32(0) // empty AUINT16
	}
	d.mtpString("Google")
	d.mtpString("Pixel 7")
	d.mtpString("13")
	d.mtpString("ABC123")
	man, model, serial, err := parseDeviceInfo(d.b)
	if err != nil {
		t.Fatal(err)
	}
	if man != "Google" || model != "Pixel 7" || serial != "ABC123" {
		t.Fatalf("device %q %q %q", man, model, serial)
	}
}

func TestParseMTPDate(t *testing.T) {
	y, mo, d, h, mi, se, ok := parseMTPDate("20260824T225900")
	if !ok || y != 2026 || mo != 8 || d != 24 || h != 22 || mi != 59 || se != 0 {
		t.Fatalf("date = %d-%d-%d %d:%d:%d ok=%v", y, mo, d, h, mi, se, ok)
	}
	if _, _, _, _, _, _, ok := parseMTPDate("nope"); ok {
		t.Fatal("expected fail")
	}
}

func TestMemPullPush(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fs := NewMem(Device{Serial: "dev1", State: "online", Model: "Pixel"})
	fs.PutDir("/Internal/DCIM", now)
	fs.PutFile("/Internal/DCIM/a.txt", []byte("hello"), now)

	ctx := context.Background()
	ents, err := fs.List(ctx, "dev1", "/Internal")
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || !ents[0].IsDir || ents[0].Name != "DCIM" {
		t.Fatalf("list = %+v", ents)
	}

	dir := t.TempDir()
	n, err := Pull(ctx, fs, "dev1", "/Internal/DCIM/a.txt", dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pulled %d", n)
	}
	got, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("got %q", got)
	}

	src := filepath.Join(dir, "frompc.txt")
	if err := os.WriteFile(src, []byte("pc"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err = Push(ctx, fs, "dev1", src, "/Internal")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pushed %d", n)
	}
	data, ok := fs.FileData("/Internal/frompc.txt")
	if !ok || string(data) != "pc" {
		t.Fatalf("pushed data %q ok=%v", data, ok)
	}
}

func TestUniqueName(t *testing.T) {
	used := map[string]int{}
	a := uniqueName(used, "DCIM")
	b := uniqueName(used, "DCIM")
	if a != "DCIM" || b == a {
		t.Fatalf("names %q %q", a, b)
	}
}

type seqTransport struct {
	writes [][]byte
	reads  [][]byte
	ri     int
}

func (t *seqTransport) Write(p []byte, timeout time.Duration) error {
	t.writes = append(t.writes, append([]byte(nil), p...))
	return nil
}

func (t *seqTransport) Read(max int, timeout time.Duration) ([]byte, error) {
	if t.ri >= len(t.reads) {
		return nil, fmt.Errorf("unexpected USB read")
	}
	b := t.reads[t.ri]
	t.ri++
	return append([]byte(nil), b...), nil
}

func (t *seqTransport) WriteStream(header []byte, r io.Reader, size int64, timeout time.Duration) error {
	return fmt.Errorf("WriteStream not implemented")
}

func (t *seqTransport) Close() error { return nil }

func encodeResponse(code uint16, tx uint32, params []uint32) []byte {
	b := make([]byte, containerHeaderSize+4*len(params))
	binary.LittleEndian.PutUint32(b[0:4], uint32(len(b)))
	binary.LittleEndian.PutUint16(b[4:6], containerResponse)
	binary.LittleEndian.PutUint16(b[6:8], code)
	binary.LittleEndian.PutUint32(b[8:12], tx)
	for i, p := range params {
		binary.LittleEndian.PutUint32(b[12+4*i:], p)
	}
	return b
}

func encodeU32Array(ids []uint32) []byte {
	var w byteWriter
	w.u32(uint32(len(ids)))
	for _, id := range ids {
		w.u32(id)
	}
	return w.b
}

func TestObjectHandlesUsesAndroidStorageRoot(t *testing.T) {
	payload := encodeU32Array([]uint32{10, 20})
	data := encodeData(opGetObjectHandles, 0, payload)
	resp := encodeResponse(respOK, 0, nil)
	tr := &seqTransport{reads: [][]byte{append(append([]byte{}, data...), resp...)}}
	s := &session{t: tr}
	got, err := s.objectHandles(context.Background(), 0x00010001, handleRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("handles = %v", got)
	}
	if len(tr.writes) != 1 {
		t.Fatalf("writes = %d", len(tr.writes))
	}
	params := paramsFrom(tr.writes[0][containerHeaderSize:])
	if len(params) != 3 || params[0] != 0x00010001 || params[1] != 0 || params[2] != handleRoot {
		t.Fatalf("GetObjectHandles params = %v", params)
	}
	if handleRoot != 0xffffffff {
		t.Fatalf("storage root parent = 0x%x, Android expects 0xffffffff", handleRoot)
	}
}

func TestReceiveDataKeepsTrailingResponse(t *testing.T) {
	payload := encodeU32Array([]uint32{0x00010001})
	data := encodeData(opGetStorageIDs, 0, payload)
	resp := encodeResponse(respOK, 0, nil)
	tr := &seqTransport{reads: [][]byte{append(append([]byte{}, data...), resp...)}}
	s := &session{t: tr}
	got, err := s.receiveData(context.Background(), opGetStorageIDs, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := (&byteReader{b: got}).u32Array()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 0x00010001 {
		t.Fatalf("ids = %v", ids)
	}
	if len(s.pending) != 0 {
		t.Fatalf("pending leftover = %d", len(s.pending))
	}
}

func TestObjectHandlesRetriesPTPRoot(t *testing.T) {
	fail := encodeResponse(respInvalidParent, 0, nil)
	payload := encodeU32Array([]uint32{7})
	data := encodeData(opGetObjectHandles, 1, payload)
	ok := encodeResponse(respOK, 1, nil)
	tr := &seqTransport{reads: [][]byte{
		fail,
		append(append([]byte{}, data...), ok...),
	}}
	s := &session{t: tr}
	got, err := s.objectHandles(context.Background(), 0x00010001, handleRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("handles = %v", got)
	}
	if len(tr.writes) != 2 {
		t.Fatalf("writes = %d", len(tr.writes))
	}
	first := paramsFrom(tr.writes[0][containerHeaderSize:])
	second := paramsFrom(tr.writes[1][containerHeaderSize:])
	if first[2] != handleRoot || second[2] != handleAll {
		t.Fatalf("parents %v then %v", first, second)
	}
}

func TestGetPartialObjectRespectsSize(t *testing.T) {
	content := []byte("hello MTP!")
	data := encodeData(opGetPartialObject, 0, content)
	resp := encodeResponse(respOK, 0, nil)
	tr := &seqTransport{reads: [][]byte{append(append([]byte{}, data...), resp...)}}
	s := &session{t: tr}
	var buf bytes.Buffer
	n, err := s.getObjectTo(context.Background(), 42, int64(len(content)), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(content)) || buf.String() != string(content) {
		t.Fatalf("got %q n=%d", buf.String(), n)
	}
	params := paramsFrom(tr.writes[0][containerHeaderSize:])
	if len(params) != 3 || params[0] != 42 || params[1] != 0 || params[2] != uint32(len(content)) {
		t.Fatalf("GetPartialObject params = %v", params)
	}
}

func TestGetPartialObjectChunksThenStops(t *testing.T) {
	const size = partialChunk + 10
	part1 := bytes.Repeat([]byte{'a'}, partialChunk)
	part2 := bytes.Repeat([]byte{'b'}, 10)
	d0 := encodeData(opGetPartialObject, 0, part1)
	r0 := encodeResponse(respOK, 0, nil)
	d1 := encodeData(opGetPartialObject, 1, part2)
	r1 := encodeResponse(respOK, 1, nil)
	tr := &seqTransport{reads: [][]byte{
		append(append([]byte{}, d0...), r0...),
		append(append([]byte{}, d1...), r1...),
	}}
	s := &session{t: tr}
	var buf bytes.Buffer
	n, err := s.getObjectTo(context.Background(), 7, size, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != size {
		t.Fatalf("n = %d", n)
	}
	if len(tr.writes) != 2 {
		t.Fatalf("writes = %d", len(tr.writes))
	}
	p0 := paramsFrom(tr.writes[0][containerHeaderSize:])
	p1 := paramsFrom(tr.writes[1][containerHeaderSize:])
	if p0[1] != 0 || p0[2] != uint32(partialChunk) {
		t.Fatalf("first chunk params %v", p0)
	}
	if p1[1] != uint32(partialChunk) || p1[2] != 10 {
		t.Fatalf("second chunk params %v (must not read past EOF)", p1)
	}
}

func TestGetObjectFullKeepsTrailingResponse(t *testing.T) {
	payload := []byte("abc")
	data := encodeData(opGetObject, 1, payload)
	resp := encodeResponse(respOK, 1, nil)
	tr := &seqTransport{reads: [][]byte{
		encodeResponse(respOpNotSupported, 0, nil), // GetPartialObject rejected
		append(append([]byte{}, data...), resp...),
	}}
	s := &session{t: tr, tx: 0}
	// First command is GetPartialObject (tx 0). Force fallback by making
	// getObjectTo see OpNotSupported. getObjectTo starts tx at 0.
	var buf bytes.Buffer
	n, err := s.getObjectTo(context.Background(), 9, 3, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || buf.String() != "abc" {
		t.Fatalf("got %q n=%d", buf.String(), n)
	}
}

func TestParseObjectPropList(t *testing.T) {
	var w byteWriter
	w.u32(6)
	w.u32(10)
	w.u16(propObjectFileName)
	w.u16(dtString)
	w.mtpString("DCIM")
	w.u32(10)
	w.u16(propObjectFormat)
	w.u16(dtUint16)
	w.u16(fmtAssociation)
	w.u32(10)
	w.u16(propParentObject)
	w.u16(dtUint32)
	w.u32(0)
	w.u32(11)
	w.u16(propObjectFileName)
	w.u16(dtString)
	w.mtpString("note.txt")
	w.u32(11)
	w.u16(propObjectFormat)
	w.u16(dtUint16)
	w.u16(fmtUndefined)
	w.u32(11)
	w.u16(propObjectSize)
	w.u16(dtUint64)
	w.u64(4)
	objs, err := parseObjectPropList(w.b)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("objs = %d", len(objs))
	}
	byHandle := map[uint32]propObject{}
	for _, o := range objs {
		byHandle[o.handle] = o
	}
	dcim := byHandle[10]
	if dcim.name != "DCIM" || !dcim.hasFormat || dcim.format != fmtAssociation {
		t.Fatalf("dcim = %+v", dcim)
	}
	if !isPropChild(dcim, handleRoot, true) {
		t.Fatal("DCIM should be a storage-root child")
	}
	note := byHandle[11]
	if note.name != "note.txt" || note.size != 4 || note.format == fmtAssociation {
		t.Fatalf("note = %+v", note)
	}
}

func TestIsPropChildSkipsParent(t *testing.T) {
	parent := propObject{handle: 5, hasParent: true, parent: 1}
	if isPropChild(parent, 5, false) {
		t.Fatal("parent object should be skipped")
	}
	child := propObject{handle: 6, hasParent: true, parent: 5}
	if !isPropChild(child, 5, false) {
		t.Fatal("direct child should be kept")
	}
}
