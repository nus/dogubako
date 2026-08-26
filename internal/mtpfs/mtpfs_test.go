package mtpfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nus/dogubako/internal/usbhost"
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

func TestCopyReportsProgress(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fs := NewMem(Device{Serial: "dev1", State: "online", Model: "Pixel"})
	fs.PutDir("/Internal/DCIM", now)
	fs.PutFile("/Internal/DCIM/a.txt", []byte("a"), now)
	fs.PutFile("/Internal/DCIM/b.txt", []byte("b"), now)

	var pull [][2]int
	ctx := WithCopyProgress(context.Background(), func(copied, total int, copiedBytes, totalBytes int64) {
		pull = append(pull, [2]int{copied, total})
	})
	dir := t.TempDir()
	n, err := Pull(ctx, fs, "dev1", "/Internal/DCIM", dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("pulled %d", n)
	}
	if len(pull) < 3 || pull[0] != [2]int{0, 2} || pull[len(pull)-1] != [2]int{2, 2} {
		t.Fatalf("pull progress = %v", pull)
	}

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "y.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	var push [][2]int
	ctx = WithCopyProgress(context.Background(), func(copied, total int, copiedBytes, totalBytes int64) {
		push = append(push, [2]int{copied, total})
	})
	n, err = Push(ctx, fs, "dev1", src, "/Internal")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("pushed %d", n)
	}
	if len(push) < 3 || push[0] != [2]int{0, 2} || push[len(push)-1] != [2]int{2, 2} {
		t.Fatalf("push progress = %v", push)
	}
}

func TestCopyReportsSingleFileByteProgress(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	data := make([]byte, 2*partialChunk)
	for i := range data {
		data[i] = byte(i)
	}
	fs := NewMem(Device{Serial: "dev1", State: "online", Model: "Pixel"})
	fs.PutDir("/Internal", now)
	fs.PutFile("/Internal/big.bin", data, now)

	var pcts []int
	ctx := WithCopyProgress(context.Background(), func(copied, total int, copiedBytes, totalBytes int64) {
		pcts = append(pcts, listPercent64(copiedBytes, totalBytes))
	})
	dir := t.TempDir()
	n, err := Pull(ctx, fs, "dev1", "/Internal/big.bin", dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pulled %d", n)
	}
	if len(pcts) < 3 || pcts[0] != 0 || pcts[1] != 50 || pcts[len(pcts)-1] != 100 {
		t.Fatalf("pull percents = %v", pcts)
	}

	src := filepath.Join(t.TempDir(), "out.bin")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	pcts = nil
	ctx = WithCopyProgress(context.Background(), func(copied, total int, copiedBytes, totalBytes int64) {
		pcts = append(pcts, listPercent64(copiedBytes, totalBytes))
	})
	n, err = Push(ctx, fs, "dev1", src, "/Internal")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pushed %d", n)
	}
	if len(pcts) < 3 || pcts[0] != 0 || pcts[1] != 50 || pcts[len(pcts)-1] != 100 {
		t.Fatalf("push percents = %v", pcts)
	}
}

func TestPullCancelRemovesIncompleteFile(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	data := make([]byte, 2*partialChunk)
	fs := NewMem(Device{Serial: "dev1", State: "online", Model: "Pixel"})
	fs.PutDir("/Internal", now)
	fs.PutFile("/Internal/big.bin", data, now)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = WithCopyProgress(ctx, func(copied, total int, copiedBytes, totalBytes int64) {
		if copiedBytes > 0 {
			cancel()
		}
	})
	dir := t.TempDir()
	_, err := Pull(ctx, fs, "dev1", "/Internal/big.bin", dir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	dest := filepath.Join(dir, "big.bin")
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("incomplete file should be removed, stat = %v", statErr)
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

func (t *seqTransport) WriteStreamProgress(header []byte, r io.Reader, size int64, timeout time.Duration, wrote func(int64)) error {
	return fmt.Errorf("WriteStreamProgress not implemented")
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
	n, err := s.getPartialTo(context.Background(), 42, int64(len(content)), &buf)
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
	n, err := s.getPartialTo(context.Background(), 7, size, &buf)
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

func TestGetObjectToStreamsFullObject(t *testing.T) {
	payload := []byte("abc")
	data := encodeData(opGetObject, 0, payload)
	resp := encodeResponse(respOK, 0, nil)
	tr := &seqTransport{reads: [][]byte{
		append(append([]byte{}, data...), resp...),
	}}
	s := &session{t: tr}
	var buf bytes.Buffer
	n, err := s.getObjectTo(context.Background(), 9, 3, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || buf.String() != "abc" {
		t.Fatalf("got %q n=%d", buf.String(), n)
	}
	h, err := decodeHeader(tr.writes[0])
	if err != nil {
		t.Fatal(err)
	}
	if h.code != opGetObject {
		t.Fatalf("op = 0x%04x want GetObject", h.code)
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

func TestCloseSkipsCommandWhenBroken(t *testing.T) {
	tr := &seqTransport{}
	s := &session{t: tr, broken: true}
	s.close()
	if len(tr.writes) != 0 {
		t.Fatalf("CloseSession sent on broken session: %d writes", len(tr.writes))
	}
}

func TestSessionBrokenOnUSBReadError(t *testing.T) {
	tr := &seqTransport{}
	s := &session{t: tr}
	_, err := s.storageIDs(context.Background())
	if err == nil {
		t.Fatal("expected USB error")
	}
	if !s.broken {
		t.Fatal("session should be marked broken after a transport error")
	}
}

func TestStaleResponseMarksBroken(t *testing.T) {
	tr := &seqTransport{reads: [][]byte{encodeResponse(respOK, 99, nil)}}
	s := &session{t: tr}
	_, err := s.runCommand(context.Background(), opCloseSession, nil)
	if err == nil {
		t.Fatal("expected transaction mismatch")
	}
	if !s.broken {
		t.Fatal("stale response should break the session")
	}
	if isAnyResponse(err) {
		t.Fatalf("mismatch should not look like an MTP response: %v", err)
	}
}

func TestInvalidContainerTypeMarksBroken(t *testing.T) {
	garbage := make([]byte, 16)
	binary.LittleEndian.PutUint32(garbage[0:4], 16)
	binary.LittleEndian.PutUint16(garbage[4:6], 99)
	tr := &seqTransport{reads: [][]byte{garbage}}
	s := &session{t: tr}
	_, err := s.runCommand(context.Background(), opGetStorageIDs, nil)
	if err == nil {
		t.Fatal("expected invalid container")
	}
	if !s.broken {
		t.Fatal("invalid container should break the session")
	}
}

func TestGetObjectUnknownLengthKeepsTrailingResponse(t *testing.T) {
	payload := []byte("abc")
	data := encodeData(opGetObject, 0, payload)
	binary.LittleEndian.PutUint32(data[0:4], lengthUnknown)
	resp := encodeResponse(respOK, 0, nil)
	tr := &seqTransport{reads: [][]byte{append(append([]byte{}, data...), resp...)}}
	s := &session{t: tr}
	var buf bytes.Buffer
	n, err := s.getObjectFullTo(context.Background(), 9, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || buf.String() != "abc" {
		t.Fatalf("got %q n=%d (response must not be written into the file)", buf.String(), n)
	}
	if len(s.pending) != 0 {
		t.Fatalf("pending leftover = %d", len(s.pending))
	}
	if s.broken {
		t.Fatal("successful GetObject should leave the session healthy")
	}
}

func TestGetObjectToZeroSizeStillDownloads(t *testing.T) {
	fail := encodeResponse(respOpNotSupported, 0, nil)
	data := encodeData(opGetObject, 1, []byte("hi"))
	ok := encodeResponse(respOK, 1, nil)
	tr := &seqTransport{reads: [][]byte{
		fail,
		append(append([]byte{}, data...), ok...),
	}}
	s := &session{t: tr}
	var buf bytes.Buffer
	n, err := s.getObjectTo(context.Background(), 5, 0, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || buf.String() != "hi" {
		t.Fatalf("got %q n=%d", buf.String(), n)
	}
}

func TestGetObjectToUsesFullObjectForKnownSize(t *testing.T) {
	data := encodeData(opGetObject, 0, []byte("xyz"))
	ok := encodeResponse(respOK, 0, nil)
	tr := &seqTransport{reads: [][]byte{
		append(append([]byte{}, data...), ok...),
	}}
	s := &session{t: tr}
	var buf bytes.Buffer
	n, err := s.getObjectTo(context.Background(), 9, 3, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || buf.String() != "xyz" {
		t.Fatalf("got %q n=%d", buf.String(), n)
	}
	h, err := decodeHeader(tr.writes[0])
	if err != nil {
		t.Fatal(err)
	}
	if h.code != opGetObject {
		t.Fatalf("op = 0x%04x want GetObject", h.code)
	}
}

func TestEntriesFromInfosStopsOnTransportError(t *testing.T) {
	info := encodeObjectInfo(1, 0, fmtUndefined, 4, "a.txt", 0)
	d0 := encodeData(opGetObjectInfo, 0, info)
	r0 := encodeResponse(respOK, 0, nil)
	tr := &seqTransport{reads: [][]byte{append(append([]byte{}, d0...), r0...)}}
	s := &session{t: tr}
	d := &openDev{sess: s, nodes: map[string]node{}}
	_, err := d.entriesFromInfos(context.Background(), "/Internal", node{storageID: 1}, []uint32{1, 2})
	if err == nil {
		t.Fatal("expected error after USB read failed")
	}
	if !s.broken {
		t.Fatal("transport error during GetObjectInfo should break the session")
	}
}

func encodeStorageInfo(desc, volume string) []byte {
	var w byteWriter
	w.u16(1)
	w.u16(2)
	w.u16(0)
	w.u64(0)
	w.u64(0)
	w.u32(0)
	w.mtpString(desc)
	w.mtpString(volume)
	return w.b
}

type scriptedMTP struct {
	info usbhost.Info

	mu          sync.Mutex
	failStorage int
	lastOp      uint16
	lastTx      uint32
	queue       [][]byte
	closed      int
}

func (c *scriptedMTP) Info() usbhost.Info { return c.info }
func (c *scriptedMTP) MaxPacketOut() int  { return 512 }

func (c *scriptedMTP) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed++
	return nil
}

func (c *scriptedMTP) WriteStream(header []byte, r io.Reader, size int64, timeout time.Duration) error {
	return fmt.Errorf("WriteStream not implemented")
}

func (c *scriptedMTP) WriteStreamProgress(header []byte, r io.Reader, size int64, timeout time.Duration, wrote func(int64)) error {
	return fmt.Errorf("WriteStreamProgress not implemented")
}

func (c *scriptedMTP) Write(p []byte, timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	h, err := decodeHeader(p)
	if err != nil {
		return err
	}
	c.lastOp = h.code
	c.lastTx = h.transactionID
	if h.code == opGetStorageIDs && c.failStorage > 0 {
		c.failStorage--
		c.queue = nil
		return nil
	}
	c.queue = c.reply(h)
	return nil
}

func (c *scriptedMTP) Read(max int, timeout time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastOp == opGetStorageIDs && len(c.queue) == 0 {
		return nil, fmt.Errorf("usb timeout")
	}
	if len(c.queue) == 0 {
		return nil, fmt.Errorf("no MTP reply queued for 0x%04x", c.lastOp)
	}
	b := c.queue[0]
	c.queue = c.queue[1:]
	return b, nil
}

func (c *scriptedMTP) reply(h containerHeader) [][]byte {
	tx := h.transactionID
	switch h.code {
	case opOpenSession, opCloseSession:
		return [][]byte{encodeResponse(respOK, tx, nil)}
	case opGetDeviceInfo:
		return [][]byte{append(encodeData(h.code, tx, nil), encodeResponse(respOK, tx, nil)...)}
	case opGetStorageIDs:
		payload := encodeU32Array([]uint32{0x00010001})
		return [][]byte{append(encodeData(h.code, tx, payload), encodeResponse(respOK, tx, nil)...)}
	case opGetStorageInfo:
		payload := encodeStorageInfo("Internal", "vol0")
		return [][]byte{append(encodeData(h.code, tx, payload), encodeResponse(respOK, tx, nil)...)}
	default:
		return [][]byte{encodeResponse(respOpNotSupported, tx, nil)}
	}
}

func TestLiveRetriesListAfterBrokenSession(t *testing.T) {
	info := usbhost.Info{ID: "dev", Product: "Pixel"}
	var opens int
	remainingFails := 1
	c := &live{
		sessions: map[string]*openDev{},
		listFn: func() ([]usbhost.Info, error) {
			return []usbhost.Info{info}, nil
		},
		openFn: func(id string) (usbhost.Conn, error) {
			opens++
			fail := 0
			if remainingFails > 0 {
				fail = 1
				remainingFails--
			}
			return &scriptedMTP{info: info, failStorage: fail}, nil
		},
	}
	ents, err := c.List(context.Background(), "dev", "/")
	if err != nil {
		t.Fatalf("list after retry: %v", err)
	}
	if len(ents) != 1 || ents[0].Name != "Internal" {
		t.Fatalf("entries = %+v", ents)
	}
	if opens != 2 {
		t.Fatalf("opens = %d, want 2 (reconnect after the first USB timeout)", opens)
	}
}

func TestLiveListAfterFailedListUsesFreshSession(t *testing.T) {
	info := usbhost.Info{ID: "dev", Product: "Pixel"}
	var opens int
	remainingFails := 2
	c := &live{
		sessions: map[string]*openDev{},
		listFn: func() ([]usbhost.Info, error) {
			return []usbhost.Info{info}, nil
		},
		openFn: func(id string) (usbhost.Conn, error) {
			opens++
			fail := 0
			if remainingFails > 0 {
				fail = 1
				remainingFails--
			}
			return &scriptedMTP{info: info, failStorage: fail}, nil
		},
	}
	if _, err := c.List(context.Background(), "dev", "/"); err == nil {
		t.Fatal("expected first list to fail after retry")
	} else if !IsRetryExhausted(err) {
		t.Fatalf("first list error should be retry-exhausted: %v", err)
	}
	ents, err := c.List(context.Background(), "dev", "/")
	if err != nil {
		t.Fatalf("second list should work on a fresh session: %v", err)
	}
	if len(ents) != 1 || ents[0].Name != "Internal" {
		t.Fatalf("entries = %+v", ents)
	}
	if opens < 3 {
		t.Fatalf("opens = %d, want at least 3", opens)
	}
}
