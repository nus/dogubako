package app

import (
	"context"
	"fmt"
	"time"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/adbfs"
	"github.com/nus/dogubako/internal/i18n"
)

const (
	adbListTimeout = 20 * time.Second
	adbCopyTimeout = time.Hour
)

type devicesResult struct {
	devices []adbfs.Device
	err     error
}

type listResult struct {
	path    string
	probe   bool
	entries []adbfs.Entry
	err     error
}

type copyResult struct {
	n      int
	dest   string
	err    error
	reload bool
}

// AndroidTreeRow is one visible line in the device file tree.
type AndroidTreeRow struct {
	Entry    adbfs.Entry
	Depth    int
	Expanded bool
}

// AndroidModel holds the Android file-manager tool state.
type AndroidModel struct {
	generation uint64
	client     adbfs.Client

	devices  []adbfs.Device
	serial   string
	root     string
	selected string

	expanded map[string]bool
	children map[string][]adbfs.Entry
	probe    []string
	loaded   bool

	status statusMsg

	pendingDevices <-chan devicesResult
	pendingList    <-chan listResult
	pendingCopy    <-chan copyResult
}

func (m *AndroidModel) Generation() uint64 { return m.generation }

func (m *AndroidModel) StatusText(lang i18n.Lang) string {
	if m.status.key == "" {
		return ""
	}
	return i18n.T(lang, m.status.key, m.status.args...)
}

func (m *AndroidModel) SetStatus(key i18n.Key, args ...any) {
	if m.status.key == key && fmt.Sprint(m.status.args...) == fmt.Sprint(args...) {
		return
	}
	m.status.key = key
	if len(args) == 0 {
		m.status.args = nil
	} else {
		m.status.args = append([]any(nil), args...)
	}
	m.generation++
}

func (m *AndroidModel) SetClient(c adbfs.Client) {
	m.client = c
}

func (m *AndroidModel) Client() adbfs.Client {
	if m.client == nil {
		m.client = adbfs.Default()
	}
	return m.client
}

func (m *AndroidModel) Busy() bool {
	return m.pendingDevices != nil || m.pendingList != nil || m.pendingCopy != nil
}

func (m *AndroidModel) Devices() []adbfs.Device { return m.devices }
func (m *AndroidModel) Serial() string          { return m.serial }
func (m *AndroidModel) Root() string {
	if m.root == "" {
		return "/"
	}
	return m.root
}
func (m *AndroidModel) Selected() string { return m.selected }

func (m *AndroidModel) HasSelection() bool {
	_, ok := m.lookup(m.selected)
	return ok
}

func (m *AndroidModel) CanGoUp() bool {
	return m.serial != "" && m.Root() != "/"
}

func (m *AndroidModel) PushDest() string {
	if e, ok := m.lookup(m.selected); ok {
		if e.IsDir {
			return e.Path
		}
		return adbfs.Parent(e.Path)
	}
	return m.Root()
}

func (m *AndroidModel) SelectedEntry() (adbfs.Entry, bool) {
	return m.lookup(m.selected)
}

func (m *AndroidModel) lookup(p string) (adbfs.Entry, bool) {
	if p == "" {
		return adbfs.Entry{}, false
	}
	p = adbfs.Clean(p)
	if p == m.Root() {
		return adbfs.Entry{Name: adbfs.Base(p), Path: p, IsDir: true}, true
	}
	parent := adbfs.Parent(p)
	for _, e := range m.children[parent] {
		if e.Path == p {
			return e, true
		}
	}
	return adbfs.Entry{}, false
}

func (m *AndroidModel) Rows() []AndroidTreeRow {
	var rows []AndroidTreeRow
	m.appendRows(m.Root(), 0, &rows)
	return rows
}

func (m *AndroidModel) appendRows(dir string, depth int, rows *[]AndroidTreeRow) {
	for _, e := range m.children[dir] {
		expanded := e.IsDir && m.expanded[e.Path]
		*rows = append(*rows, AndroidTreeRow{Entry: e, Depth: depth, Expanded: expanded})
		if expanded {
			m.appendRows(e.Path, depth+1, rows)
		}
	}
}

func (m *AndroidModel) EnsureLoaded() {
	if m.loaded || m.Busy() {
		return
	}
	m.RefreshDevices()
}

func (m *AndroidModel) Drain() {
	m.drainDevices()
	m.drainList()
	m.drainCopy()
}

func (m *AndroidModel) drainDevices() {
	if m.pendingDevices == nil {
		return
	}
	select {
	case res := <-m.pendingDevices:
		m.pendingDevices = nil
		m.applyDevices(res.devices, res.err)
		guigui.RequestRebuild()
	default:
	}
}

func (m *AndroidModel) drainList() {
	if m.pendingList == nil {
		return
	}
	select {
	case res := <-m.pendingList:
		m.pendingList = nil
		m.applyList(res)
		guigui.RequestRebuild()
	default:
	}
}

func (m *AndroidModel) drainCopy() {
	if m.pendingCopy == nil {
		return
	}
	select {
	case res := <-m.pendingCopy:
		m.pendingCopy = nil
		if res.err != nil {
			m.SetStatus(i18n.StatusAdbCopyFailed, res.err)
		} else {
			m.SetStatus(i18n.StatusAdbCopied, res.n, res.dest)
			if res.reload && m.serial != "" {
				m.startList(m.Root(), false)
			}
		}
		guigui.RequestRebuild()
	default:
	}
}

func (m *AndroidModel) applyDevices(devs []adbfs.Device, err error) {
	m.devices = devs
	m.generation++
	if err != nil {
		m.serial = ""
		m.children = nil
		m.SetStatus(i18n.StatusAdbConnectFailed, err)
		return
	}
	if len(devs) == 0 {
		m.serial = ""
		m.children = nil
		m.SetStatus(i18n.StatusAdbNoDevices)
		return
	}
	if m.device(m.serial).Serial == "" {
		m.serial = firstOnlineSerial(devs)
		if m.serial == "" {
			m.serial = devs[0].Serial
		}
	}
	d := m.device(m.serial)
	if !d.Online() {
		m.children = nil
		m.SetStatus(i18n.StatusAdbDeviceOffline, d.State)
		return
	}
	m.beginRootProbe()
}

func (m *AndroidModel) device(serial string) adbfs.Device {
	for _, d := range m.devices {
		if d.Serial == serial {
			return d
		}
	}
	return adbfs.Device{}
}

func firstOnlineSerial(devs []adbfs.Device) string {
	for _, d := range devs {
		if d.Online() {
			return d.Serial
		}
	}
	return ""
}

func (m *AndroidModel) beginRootProbe() {
	m.children = map[string][]adbfs.Entry{}
	m.expanded = map[string]bool{}
	m.selected = ""
	m.probe = append([]string(nil), adbfs.DefaultRoots...)
	m.tryNextRoot()
}

func (m *AndroidModel) tryNextRoot() {
	if len(m.probe) == 0 {
		m.root = "/"
		m.startList("/", false)
		return
	}
	m.root = m.probe[0]
	m.probe = m.probe[1:]
	m.startList(m.root, true)
}

func (m *AndroidModel) applyList(res listResult) {
	if res.err != nil {
		if res.probe && (len(m.probe) > 0 || res.path != "/") {
			m.tryNextRoot()
			return
		}
		if m.children == nil {
			m.children = map[string][]adbfs.Entry{}
		}
		m.children[res.path] = nil
		m.SetStatus(i18n.StatusAdbListFailed, res.err)
		return
	}
	if res.probe {
		m.probe = nil
		m.root = res.path
	}
	if m.children == nil {
		m.children = map[string][]adbfs.Entry{}
	}
	m.children[res.path] = res.entries
	m.generation++
	m.SetStatus(i18n.StatusAdbListed, res.path, len(res.entries))
}

func (m *AndroidModel) RefreshDevices() {
	if m.Busy() {
		return
	}
	m.loaded = true
	m.SetStatus(i18n.StatusAdbListing)
	ch := make(chan devicesResult, 1)
	m.pendingDevices = ch
	m.generation++
	client := m.Client()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), adbListTimeout)
		defer cancel()
		devs, err := client.Devices(ctx)
		ch <- devicesResult{devices: devs, err: err}
	}()
}

func (m *AndroidModel) Reload() {
	if m.Busy() || m.serial == "" {
		if !m.Busy() {
			m.RefreshDevices()
		}
		return
	}
	d := m.device(m.serial)
	if !d.Online() {
		m.RefreshDevices()
		return
	}
	m.children = map[string][]adbfs.Entry{}
	m.expanded = map[string]bool{}
	m.startList(m.Root(), false)
}

func (m *AndroidModel) SelectDevice(serial string) {
	if m.Busy() || serial == "" || serial == m.serial {
		return
	}
	m.serial = serial
	m.generation++
	d := m.device(serial)
	if !d.Online() {
		m.children = nil
		m.SetStatus(i18n.StatusAdbDeviceOffline, d.State)
		return
	}
	m.beginRootProbe()
}

func (m *AndroidModel) SelectPath(path string) {
	path = adbfs.Clean(path)
	if m.selected == path {
		return
	}
	m.selected = path
	m.generation++
}

func (m *AndroidModel) ToggleExpand(path string) {
	if m.Busy() {
		return
	}
	e, ok := m.lookup(path)
	if !ok || !e.IsDir {
		return
	}
	if m.expanded == nil {
		m.expanded = map[string]bool{}
	}
	if m.expanded[path] {
		m.expanded[path] = false
		m.generation++
		return
	}
	m.expanded[path] = true
	m.generation++
	if _, ok := m.children[path]; !ok {
		m.startList(path, false)
	}
}

func (m *AndroidModel) GoUp() {
	if m.Busy() || !m.CanGoUp() {
		return
	}
	m.root = adbfs.Parent(m.Root())
	m.selected = ""
	m.expanded = map[string]bool{}
	m.children = map[string][]adbfs.Entry{}
	m.startList(m.root, false)
}

func (m *AndroidModel) startList(path string, probe bool) {
	if m.pendingList != nil || m.serial == "" {
		return
	}
	path = adbfs.Clean(path)
	m.SetStatus(i18n.StatusAdbListing)
	ch := make(chan listResult, 1)
	m.pendingList = ch
	m.generation++
	client := m.Client()
	serial := m.serial
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), adbListTimeout)
		defer cancel()
		ents, err := client.List(ctx, serial, path)
		ch <- listResult{path: path, probe: probe, entries: ents, err: err}
	}()
}

func (m *AndroidModel) StartPull(local string) {
	if m.Busy() {
		return
	}
	e, ok := m.lookup(m.selected)
	if !ok {
		m.SetStatus(i18n.StatusAdbNoSelection)
		return
	}
	m.startCopy(true, e.Path, local)
}

func (m *AndroidModel) StartPush(local string) {
	if m.Busy() {
		return
	}
	if m.serial == "" || !m.device(m.serial).Online() {
		m.SetStatus(i18n.StatusAdbSelectOnline)
		return
	}
	m.startCopy(false, local, m.PushDest())
}

func (m *AndroidModel) startCopy(pull bool, src, dest string) {
	m.SetStatus(i18n.StatusAdbCopying)
	ch := make(chan copyResult, 1)
	m.pendingCopy = ch
	m.generation++
	client := m.Client()
	serial := m.serial
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), adbCopyTimeout)
		defer cancel()
		var n int
		var err error
		if pull {
			n, err = adbfs.Pull(ctx, client, serial, src, dest)
		} else {
			n, err = adbfs.Push(ctx, client, serial, src, dest)
		}
		ch <- copyResult{n: n, dest: dest, err: err, reload: !pull}
	}()
}

func formatFileSize(n int64, isDir bool) string {
	if isDir {
		return "—"
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	kb := float64(n) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1f KB", kb)
	}
	mb := kb / 1024
	if mb < 1024 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmt.Sprintf("%.1f GB", mb/1024)
}

func formatFileTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("2006-01-02 15:04")
}
