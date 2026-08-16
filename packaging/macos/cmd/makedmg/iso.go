package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
)

const sectorSize = 2048

type node struct {
	name       string
	src        string
	isDir      bool
	isSymlink  bool
	linkTarget string
	children   []*node
	modTime    time.Time
	size       int64

	isoID    string
	jolietID string

	isoExtent    uint32
	isoDataLen   uint32
	jolietExtent uint32
	jolietLen    uint32
	dataExtent   uint32
	dirNo        int
	parent       *node
	inode        uint32
}

func writeImage(outPath, srcDir, volName string, now time.Time) error {
	root, err := walkTree(srcDir, now)
	if err != nil {
		return err
	}
	assignISONames(root)
	dirs := collectDirs(root)
	for i, d := range dirs {
		d.dirNo = i + 1
	}

	// Layout: 16 zero sectors, PVD, Joliet SVD, terminator, then path tables,
	// ISO directories, Joliet directories, file payloads.
	next := uint32(19)

	isoLPath, isoLSize := buildPathTable(dirs, false, true)
	isoMPath, _ := buildPathTable(dirs, false, false)
	jolLPath, jolLSize := buildPathTable(dirs, true, true)
	jolMPath, _ := buildPathTable(dirs, true, false)

	isoLLoc := next
	next += sectorsFor(len(isoLPath))
	isoMLoc := next
	next += sectorsFor(len(isoMPath))
	jolLLoc := next
	next += sectorsFor(len(jolLPath))
	jolMLoc := next
	next += sectorsFor(len(jolMPath))

	// Directory records need child extents. Assign ISO dir extents first
	// using a size estimate, then rebuild records once extents are known.
	// Two-pass: compute packed sizes with placeholder extents, assign,
	// then rebuild with real extents (size does not depend on extent values).
	assignDirExtents(dirs, &next, false)
	assignDirExtents(dirs, &next, true)
	assignFileExtents(root, &next)

	isoDirs := make([][]byte, len(dirs))
	jolDirs := make([][]byte, len(dirs))
	for i, d := range dirs {
		isoDirs[i] = packDirRecords(dirRecords(d, false))
		jolDirs[i] = packDirRecords(dirRecords(d, true))
		if uint32(len(isoDirs[i])) != d.isoDataLen {
			return fmt.Errorf("iso dir size changed for %s: %d vs %d", d.name, len(isoDirs[i]), d.isoDataLen)
		}
		if uint32(len(jolDirs[i])) != d.jolietLen {
			return fmt.Errorf("joliet dir size changed for %s: %d vs %d", d.name, len(jolDirs[i]), d.jolietLen)
		}
	}

	totalSectors := next
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zero := make([]byte, sectorSize)
	for i := 0; i < 16; i++ {
		if _, err := f.Write(zero); err != nil {
			return err
		}
	}

	pvd := makeVolumeDescriptor(1, volName, false, totalSectors, uint32(isoLSize), isoLLoc, isoMLoc, root, now)
	svd := makeVolumeDescriptor(2, volName, true, totalSectors, uint32(jolLSize), jolLLoc, jolMLoc, root, now)
	term := make([]byte, sectorSize)
	term[0] = 255
	copy(term[1:6], "CD001")
	term[6] = 1

	if _, err := f.Write(pvd); err != nil {
		return err
	}
	if _, err := f.Write(svd); err != nil {
		return err
	}
	if _, err := f.Write(term); err != nil {
		return err
	}

	if err := writePadded(f, isoLPath); err != nil {
		return err
	}
	if err := writePadded(f, isoMPath); err != nil {
		return err
	}
	if err := writePadded(f, jolLPath); err != nil {
		return err
	}
	if err := writePadded(f, jolMPath); err != nil {
		return err
	}

	for _, data := range isoDirs {
		if err := writePadded(f, data); err != nil {
			return err
		}
	}
	for _, data := range jolDirs {
		if err := writePadded(f, data); err != nil {
			return err
		}
	}
	if err := writeFilePayloads(f, root); err != nil {
		return err
	}

	// ISO 9660 images are already a complete volume. macOS accepts them as
	// .dmg because DiskImageMounter sniffs CD001 at sector 16.
	return f.Close()
}

func walkTree(srcDir string, now time.Time) (*node, error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		return nil, err
	}
	root := &node{
		name:    ".",
		src:     srcDir,
		isDir:   true,
		modTime: info.ModTime(),
		inode:   1,
	}
	if root.modTime.IsZero() {
		root.modTime = now
	}
	var nextIno uint32 = 2
	err = walkChildren(root, srcDir, now, &nextIno)
	return root, err
}

func walkChildren(parent *node, dir string, now time.Time, nextIno *uint32) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, e := range entries {
		name := e.Name()
		if name == "." || name == ".." {
			continue
		}
		full := filepath.Join(dir, name)
		n := &node{name: name, parent: parent, inode: *nextIno}
		*nextIno++
		fi, err := os.Lstat(full)
		if err != nil {
			return err
		}
		n.modTime = fi.ModTime()
		if n.modTime.IsZero() {
			n.modTime = now
		}
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(full)
			if err != nil {
				return err
			}
			n.isSymlink = true
			n.linkTarget = target
		case fi.IsDir():
			n.isDir = true
			n.src = full
			if err := walkChildren(n, full, now, nextIno); err != nil {
				return err
			}
		default:
			n.src = full
			n.size = fi.Size()
		}
		parent.children = append(parent.children, n)
	}
	return nil
}

func collectDirs(root *node) []*node {
	var out []*node
	var walk func(*node)
	walk = func(n *node) {
		if !n.isDir {
			return
		}
		out = append(out, n)
		kids := append([]*node(nil), n.children...)
		sort.Slice(kids, func(i, j int) bool {
			return kids[i].name < kids[j].name
		})
		for _, c := range kids {
			walk(c)
		}
	}
	walk(root)
	return out
}

func assignISONames(n *node) {
	used := map[string]int{}
	for _, c := range n.children {
		c.isoID = uniqueISOName(c, used)
		if c.isDir {
			c.jolietID = c.name
		} else {
			c.jolietID = c.name + ";1"
		}
		if c.isDir {
			assignISONames(c)
		}
	}
}

func uniqueISOName(n *node, used map[string]int) string {
	base := isoSanitize(n.name)
	if !n.isDir {
		if !strings.Contains(base, ".") {
			base += "."
		}
		base += ";1"
	}
	if len(base) > 31 {
		base = trimISO(base, 31)
	}
	candidate := base
	for used[candidate] > 0 {
		used[base]++
		candidate = numberedISO(base, used[base])
	}
	used[candidate] = 1
	return candidate
}

func isoSanitize(name string) string {
	name = strings.ToUpper(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		s = "FILE"
	}
	return s
}

func trimISO(id string, max int) string {
	if len(id) <= max {
		return id
	}
	if strings.HasSuffix(id, ";1") {
		keep := max - 2
		if keep < 1 {
			keep = 1
		}
		return id[:keep] + ";1"
	}
	return id[:max]
}

func numberedISO(base string, n int) string {
	suffix := fmt.Sprintf("_%d", n)
	if strings.HasSuffix(base, ";1") {
		stem := strings.TrimSuffix(base, ";1")
		keep := 31 - 2 - len(suffix)
		if keep < 1 {
			keep = 1
		}
		if len(stem) > keep {
			stem = stem[:keep]
		}
		return stem + suffix + ";1"
	}
	keep := 31 - len(suffix)
	if keep < 1 {
		keep = 1
	}
	if len(base) > keep {
		base = base[:keep]
	}
	return base + suffix
}

func assignDirExtents(dirs []*node, next *uint32, joliet bool) {
	for _, d := range dirs {
		data := packDirRecords(dirRecords(d, joliet))
		if joliet {
			d.jolietExtent = *next
			d.jolietLen = uint32(len(data))
		} else {
			d.isoExtent = *next
			d.isoDataLen = uint32(len(data))
		}
		*next += sectorsFor(len(data))
	}
}

func assignFileExtents(n *node, next *uint32) {
	if n.isDir {
		for _, c := range n.children {
			assignFileExtents(c, next)
		}
		return
	}
	if n.isSymlink {
		n.size = 0
		n.dataExtent = 0
		return
	}
	n.dataExtent = *next
	*next += sectorsFor(int(n.size))
}

func dirRecords(d *node, joliet bool) [][]byte {
	recs := [][]byte{
		makeDirRecord("\x00", d, d, true, joliet, true),
		makeDirRecord("\x01", d.parentOrSelf(), d, true, joliet, false),
	}
	kids := append([]*node(nil), d.children...)
	sort.Slice(kids, func(i, j int) bool {
		if joliet {
			return jolietSortKey(kids[i].jolietID) < jolietSortKey(kids[j].jolietID)
		}
		return kids[i].isoID < kids[j].isoID
	})
	for _, c := range kids {
		id := c.isoID
		if joliet {
			id = c.jolietID
		}
		recs = append(recs, makeDirRecord(id, c, d, c.isDir, joliet, false))
	}
	return recs
}

func (n *node) parentOrSelf() *node {
	if n.parent == nil {
		return n
	}
	return n.parent
}

func jolietSortKey(s string) string {
	return strings.ToUpper(s)
}

func makeDirRecord(id string, target, container *node, isDir, joliet, isRootDot bool) []byte {
	var idBytes []byte
	if joliet && id != "\x00" && id != "\x01" {
		idBytes = encodeUTF16BE(id)
	} else {
		idBytes = []byte(id)
	}

	extent, dataLen := target.isoExtent, target.isoDataLen
	if !target.isDir {
		extent, dataLen = target.dataExtent, uint32(target.size)
	} else if joliet {
		extent, dataLen = target.jolietExtent, target.jolietLen
	}

	su := rockRidge(target, container, isDir, isRootDot, id)
	recLen := dirRecordLen(len(idBytes), len(su))
	rec := make([]byte, recLen)
	rec[0] = byte(recLen)
	putBoth32(rec[2:10], extent)
	putBoth32(rec[10:18], dataLen)
	putDirTime(rec[18:25], target.modTime)
	if isDir {
		rec[25] = 2
	}
	putBoth16(rec[28:32], 1)
	rec[32] = byte(len(idBytes))
	copy(rec[33:], idBytes)
	idEnd := 33 + len(idBytes)
	if len(idBytes)%2 == 0 {
		idEnd++
	}
	copy(rec[idEnd:], su)
	return rec
}

func dirRecordLen(idLen, suLen int) int {
	n := 33 + idLen
	if idLen%2 == 0 {
		n++
	}
	n += suLen
	if n%2 == 1 {
		n++
	}
	return n
}

func rockRidge(target, container *node, isDir, isRootDot bool, rawID string) []byte {
	var su []byte
	if isRootDot && container.parent == nil && rawID == "\x00" {
		su = append(su, 'S', 'P', 7, 1, 0xBE, 0xEF, 0)
	}
	mode := uint32(0o100644)
	nlink := uint32(1)
	if isDir {
		mode = 0o040755
		nlink = 2
	} else if target.isSymlink {
		mode = 0o120755
	} else if target.name == "dogubako" || strings.HasSuffix(target.src, string(filepath.Separator)+"dogubako") {
		mode = 0o100755
	} else if target.src != "" {
		if fi, err := os.Lstat(target.src); err == nil && fi.Mode()&0o111 != 0 {
			mode = 0o100755
		}
	}
	px := make([]byte, 44)
	px[0], px[1], px[2], px[3] = 'P', 'X', 44, 1
	putBoth32(px[4:12], mode)
	putBoth32(px[12:20], nlink)
	putBoth32(px[20:28], 0)
	putBoth32(px[28:36], 0)
	putBoth32(px[36:44], target.inode)
	su = append(su, px...)

	if rawID != "\x00" && rawID != "\x01" {
		nm := target.name
		nmRec := []byte{'N', 'M', byte(5 + len(nm)), 1, 0}
		nmRec = append(nmRec, nm...)
		if len(nmRec)%2 == 1 {
			// NM length includes header; no extra pad here — SUSP entries
			// themselves are not required to be even, the dir record is.
		}
		su = append(su, nmRec...)
	}
	if target.isSymlink {
		su = append(su, symlinkSL(target.linkTarget)...)
	}
	return su
}

func symlinkSL(target string) []byte {
	// SL: flags + components. 0x08 = root, then each path element.
	var comps []byte
	if path.IsAbs(target) || strings.HasPrefix(target, "/") {
		comps = append(comps, 0x08, 0)
		target = strings.TrimPrefix(target, "/")
	}
	if target == "" {
		// root only
	} else {
		parts := strings.Split(target, "/")
		for i, p := range parts {
			if p == "" {
				continue
			}
			flag := byte(0)
			if i < len(parts)-1 {
				flag = 0x01 // continue
			}
			comps = append(comps, flag, byte(len(p)))
			comps = append(comps, p...)
		}
	}
	rec := []byte{'S', 'L', byte(5 + len(comps)), 1, 0}
	rec = append(rec, comps...)
	return rec
}

func packDirRecords(recs [][]byte) []byte {
	var out []byte
	used := 0
	for _, rec := range recs {
		if used > 0 && used+len(rec) > sectorSize {
			out = append(out, make([]byte, sectorSize-used)...)
			used = 0
		}
		out = append(out, rec...)
		used += len(rec)
		if used == sectorSize {
			used = 0
		}
	}
	return out
}

func buildPathTable(dirs []*node, joliet, little bool) ([]byte, int) {
	var out []byte
	for _, d := range dirs {
		var id []byte
		if d.parent == nil {
			id = []byte{0}
		} else if joliet {
			id = encodeUTF16BE(d.name)
		} else {
			id = []byte(d.isoID)
		}
		parentNo := uint16(1)
		if d.parent != nil {
			parentNo = uint16(d.parent.dirNo)
		}
		extent := d.isoExtent
		if joliet {
			extent = d.jolietExtent
		}
		entry := make([]byte, 8+len(id)+len(id)%2)
		entry[0] = byte(len(id))
		if little {
			binary.LittleEndian.PutUint32(entry[2:6], extent)
			binary.LittleEndian.PutUint16(entry[6:8], parentNo)
		} else {
			binary.BigEndian.PutUint32(entry[2:6], extent)
			binary.BigEndian.PutUint16(entry[6:8], parentNo)
		}
		copy(entry[8:], id)
		out = append(out, entry...)
	}
	return out, len(out)
}

func makeVolumeDescriptor(typ byte, volName string, joliet bool, total, pathSize, pathL, pathM uint32, root *node, now time.Time) []byte {
	vd := make([]byte, sectorSize)
	vd[0] = typ
	copy(vd[1:6], "CD001")
	vd[6] = 1
	sys := padA("APPLE COMPUTER", 32)
	var vol []byte
	if joliet {
		copy(vd[88:91], []byte{0x25, 0x2F, 0x45})
		vol = padUTF16(volName, 16)
		copy(vd[8:40], padUTF16("APPLE COMPUTER", 16))
	} else {
		copy(vd[8:40], sys)
		// PVD only allows A–Z / 0–9 / _. Japanese names live in the Joliet SVD.
		vol = padD(isoVolumeID(volName), 32)
	}
	copy(vd[40:72], vol)
	putBoth32(vd[80:88], total)
	putBoth16(vd[120:124], 1)
	putBoth16(vd[124:128], 1)
	putBoth16(vd[128:132], sectorSize)
	putBoth32(vd[132:140], pathSize)
	if joliet {
		binary.LittleEndian.PutUint32(vd[140:144], pathL)
		binary.BigEndian.PutUint32(vd[148:152], pathM)
	} else {
		binary.LittleEndian.PutUint32(vd[140:144], pathL)
		binary.BigEndian.PutUint32(vd[148:152], pathM)
	}
	rootRec := makeDirRecord("\x00", root, root, true, joliet, true)
	if len(rootRec) > 34 {
		// PVD root record is a 34-byte directory record without SUSP.
		rootRec = makePVDRootRecord(root, joliet)
	}
	copy(vd[156:190], rootRec[:34])
	space32 := bytesRepeat(0x20, 128)
	copy(vd[190:318], space32)
	copy(vd[318:446], space32)
	copy(vd[446:574], space32)
	copy(vd[574:702], padA("DOGUBAKO", 128))
	copy(vd[702:738], bytesRepeat(0x20, 37))
	copy(vd[738:774], bytesRepeat(0x20, 37))
	copy(vd[774:810], bytesRepeat(0x20, 37))
	copy(vd[810:827], formatLongDate(now))
	copy(vd[827:844], formatLongDate(now))
	copy(vd[844:861], bytesRepeat('0', 16))
	vd[860] = 0
	copy(vd[861:878], bytesRepeat('0', 16))
	vd[877] = 0
	vd[878] = 1
	return vd
}

func makePVDRootRecord(root *node, joliet bool) []byte {
	rec := make([]byte, 34)
	rec[0] = 34
	extent, dataLen := root.isoExtent, root.isoDataLen
	if joliet {
		extent, dataLen = root.jolietExtent, root.jolietLen
	}
	putBoth32(rec[2:10], extent)
	putBoth32(rec[10:18], dataLen)
	putDirTime(rec[18:25], root.modTime)
	rec[25] = 2
	putBoth16(rec[28:32], 1)
	rec[32] = 1
	rec[33] = 0
	return rec
}

func writeFilePayloads(w io.Writer, n *node) error {
	if n.isDir {
		for _, c := range n.children {
			if err := writeFilePayloads(w, c); err != nil {
				return err
			}
		}
		return nil
	}
	if n.isSymlink || n.src == "" {
		return nil
	}
	f, err := os.Open(n.src)
	if err != nil {
		return err
	}
	defer f.Close()
	copied, err := io.Copy(w, f)
	if err != nil {
		return err
	}
	pad := sectorPad(int(copied))
	if pad == 0 {
		return nil
	}
	_, err = w.Write(make([]byte, pad))
	return err
}

func writePadded(w io.Writer, data []byte) error {
	if _, err := w.Write(data); err != nil {
		return err
	}
	if pad := sectorPad(len(data)); pad > 0 {
		_, err := w.Write(make([]byte, pad))
		return err
	}
	return nil
}

func sectorsFor(n int) uint32 {
	if n == 0 {
		return 1
	}
	return uint32((n + sectorSize - 1) / sectorSize)
}

func sectorPad(n int) int {
	r := n % sectorSize
	if r == 0 {
		return 0
	}
	return sectorSize - r
}

func putBoth16(b []byte, v uint16) {
	binary.LittleEndian.PutUint16(b[0:2], v)
	binary.BigEndian.PutUint16(b[2:4], v)
}

func putBoth32(b []byte, v uint32) {
	binary.LittleEndian.PutUint32(b[0:4], v)
	binary.BigEndian.PutUint32(b[4:8], v)
}

func putDirTime(b []byte, t time.Time) {
	t = t.UTC()
	b[0] = byte(t.Year() - 1900)
	b[1] = byte(t.Month())
	b[2] = byte(t.Day())
	b[3] = byte(t.Hour())
	b[4] = byte(t.Minute())
	b[5] = byte(t.Second())
	b[6] = 0
}

func formatLongDate(t time.Time) []byte {
	t = t.UTC()
	s := t.Format("20060102150405") + "00"
	b := make([]byte, 17)
	copy(b, s)
	return b
}

func encodeUTF16BE(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, len(u)*2)
	for i, r := range u {
		binary.BigEndian.PutUint16(out[i*2:], r)
	}
	return out
}

func padA(s string, n int) []byte {
	s = strings.ToUpper(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return padSpaces(b.String(), n)
}

func isoVolumeID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "DOGUBAKO"
	}
	return b.String()
}

func padD(s string, n int) []byte {
	s = strings.ToUpper(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := padSpaces(b.String(), n)
	return out
}

func padUTF16(s string, units int) []byte {
	u := utf16.Encode([]rune(s))
	if len(u) > units {
		u = u[:units]
	}
	out := make([]byte, units*2)
	for i := 0; i < units; i++ {
		if i < len(u) {
			binary.BigEndian.PutUint16(out[i*2:], u[i])
		} else {
			binary.BigEndian.PutUint16(out[i*2:], 0x0020)
		}
	}
	return out
}

func padSpaces(s string, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	copy(b, s)
	return b
}

func bytesRepeat(c byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return b
}