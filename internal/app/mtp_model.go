package app

import (
	"context"
	"fmt"
	"time"

	"github.com/guigui-gui/guigui"

	"github.com/nus/dogubako/internal/i18n"
	"github.com/nus/dogubako/internal/mtpfs"
)

const (
	mtpDeviceTimeout = 20 * time.Second
	mtpListTimeout   = 5 * time.Minute
	mtpCopyTimeout   = time.Hour
)

type mtpDevicesResult struct {
	devices []mtpfs.Device
	err     error
}

type mtpListResult struct {
	path    string
	entries []mtpfs.Entry
	loaded  int
	total   int
	err     error
	done    bool
}

type mtpCopyResult struct {
	n      int
	dest   string
	err    error
	reload bool
}

// MTPTreeRow is one visible line in the MTP file tree.
type MTPTreeRow struct {
	Entry    mtpfs.Entry
	Depth    int
	Expanded bool
}

// MTPModel holds the MTP file-manager tool state.
type MTPModel struct {
	generation uint64
	client     mtpfs.Client

	devices  []mtpfs.Device
	serial   string
	root     string
	selected string

	expanded map[string]bool
	children map[string][]mtpfs.Entry
	loaded   bool

	status statusMsg

	sortCol  int
	sortDesc bool

	pendingDevices <-chan mtpDevicesResult
	pendingList    <-chan mtpListResult
	pendingCopy    <-chan mtpCopyResult

	retryAlert *mtpAlert
}

func (m *MTPModel) Generation() uint64 { return m.generation }

func (m *MTPModel) StatusText(lang i18n.Lang) string {
	if m.status.key == "" {
		return ""
	}
	return i18n.T(lang, m.status.key, m.status.args...)
}

func (m *MTPModel) SetStatus(key i18n.Key, args ...any) {
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

func (m *MTPModel) SetClient(c mtpfs.Client) { m.client = c }

func (m *MTPModel) Client() mtpfs.Client {
	if m.client == nil {
		m.client = mtpfs.Default()
	}
	return m.client
}

func (m *MTPModel) Busy() bool {
	return m.pendingDevices != nil || m.pendingList != nil || m.pendingCopy != nil
}

func (m *MTPModel) Devices() []mtpfs.Device { return m.devices }
func (m *MTPModel) Serial() string          { return m.serial }
func (m *MTPModel) Root() string {
	if m.root == "" {
		return "/"
	}
	return m.root
}
func (m *MTPModel) Selected() string { return m.selected }

func (m *MTPModel) HasSelection() bool {
	_, ok := m.lookup(m.selected)
	return ok
}

func (m *MTPModel) CanPush() bool {
	return m.serial != "" && m.device(m.serial).Online() && m.PushDest() != "/"
}

func (m *MTPModel) CanGoUp() bool {
	return m.serial != "" && m.Root() != "/"
}

func (m *MTPModel) PushDest() string {
	if e, ok := m.lookup(m.selected); ok {
		if e.IsDir {
			return e.Path
		}
		return mtpfs.Parent(e.Path)
	}
	return m.Root()
}

func (m *MTPModel) SelectedEntry() (mtpfs.Entry, bool) {
	return m.lookup(m.selected)
}

func (m *MTPModel) lookup(p string) (mtpfs.Entry, bool) {
	if p == "" {
		return mtpfs.Entry{}, false
	}
	p = mtpfs.Clean(p)
	if p == m.Root() {
		return mtpfs.Entry{Name: mtpfs.Base(p), Path: p, IsDir: true}, true
	}
	parent := mtpfs.Parent(p)
	for _, e := range m.children[parent] {
		if e.Path == p {
			return e, true
		}
	}
	return mtpfs.Entry{}, false
}

func (m *MTPModel) Rows() []MTPTreeRow {
	var rows []MTPTreeRow
	m.appendRows(m.Root(), 0, &rows)
	return rows
}

func (m *MTPModel) SortCol() int {
	if m.sortCol == 0 {
		return androidSortName
	}
	return m.sortCol
}

func (m *MTPModel) SortDesc() bool { return m.sortDesc }

func (m *MTPModel) ToggleSort(col int) {
	if col != androidSortName && col != androidSortSize && col != androidSortMod {
		return
	}
	if m.SortCol() == col {
		m.sortDesc = !m.sortDesc
	} else {
		m.sortCol = col
		m.sortDesc = false
	}
	m.generation++
}

func (m *MTPModel) appendRows(dir string, depth int, rows *[]MTPTreeRow) {
	ents := append([]mtpfs.Entry(nil), m.children[dir]...)
	sortMTPEntries(ents, m.SortCol(), m.sortDesc)
	for _, e := range ents {
		expanded := e.IsDir && m.expanded[e.Path]
		*rows = append(*rows, MTPTreeRow{Entry: e, Depth: depth, Expanded: expanded})
		if expanded {
			m.appendRows(e.Path, depth+1, rows)
		}
	}
}

func (m *MTPModel) EnsureLoaded() {
	if m.loaded || m.Busy() {
		return
	}
	m.RefreshDevices()
}

func (m *MTPModel) Drain() {
	m.drainDevices()
	m.drainList()
	m.drainCopy()
}

func (m *MTPModel) TakeRetryAlert() (i18n.Key, []any, bool) {
	a := m.retryAlert
	m.retryAlert = nil
	if a == nil {
		return "", nil, false
	}
	return a.key, a.args, true
}

func (m *MTPModel) queueRetryAlert(key i18n.Key, err error) {
	if !mtpfs.IsRetryExhausted(err) {
		return
	}
	m.retryAlert = &mtpAlert{key: key, args: []any{err}}
}

type mtpAlert struct {
	key  i18n.Key
	args []any
}

func (m *MTPModel) drainDevices() {
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

func (m *MTPModel) drainList() {
	if m.pendingList == nil {
		return
	}
	select {
	case res := <-m.pendingList:
		if res.done {
			m.pendingList = nil
		}
		m.applyList(res)
		guigui.RequestRebuild()
	default:
	}
}

func (m *MTPModel) drainCopy() {
	if m.pendingCopy == nil {
		return
	}
	select {
	case res := <-m.pendingCopy:
		m.pendingCopy = nil
		if res.err != nil {
			m.SetStatus(i18n.StatusMTPCopyFailed, res.err)
			m.queueRetryAlert(i18n.StatusMTPCopyFailed, res.err)
		} else {
			m.SetStatus(i18n.StatusMTPCopied, res.n, res.dest)
			if res.reload && m.serial != "" {
				m.startList(m.Root())
			}
		}
		guigui.RequestRebuild()
	default:
	}
}

func (m *MTPModel) applyDevices(devs []mtpfs.Device, err error) {
	m.devices = devs
	m.generation++
	if err != nil {
		m.serial = ""
		m.children = nil
		m.SetStatus(i18n.StatusMTPConnectFailed, err)
		return
	}
	if len(devs) == 0 {
		m.serial = ""
		m.children = nil
		m.SetStatus(i18n.StatusMTPNoDevices)
		return
	}
	if m.device(m.serial).Serial == "" {
		m.serial = firstMTPOnlineSerial(devs)
		if m.serial == "" {
			m.serial = devs[0].Serial
		}
	}
	d := m.device(m.serial)
	if !d.Online() {
		m.children = nil
		m.SetStatus(i18n.StatusMTPDeviceOffline, d.State)
		return
	}
	m.beginRoot()
}

func (m *MTPModel) device(serial string) mtpfs.Device {
	for _, d := range m.devices {
		if d.Serial == serial {
			return d
		}
	}
	return mtpfs.Device{}
}

func firstMTPOnlineSerial(devs []mtpfs.Device) string {
	for _, d := range devs {
		if d.Online() {
			return d.Serial
		}
	}
	return ""
}

func (m *MTPModel) beginRoot() {
	m.children = map[string][]mtpfs.Entry{}
	m.expanded = map[string]bool{}
	m.selected = ""
	m.root = "/"
	m.startList("/")
}

func (m *MTPModel) applyList(res mtpListResult) {
	if m.children == nil {
		m.children = map[string][]mtpfs.Entry{}
	}
	if res.entries != nil {
		m.children[res.path] = res.entries
		m.generation++
	}
	if !res.done {
		if res.total > 0 {
			m.SetStatus(i18n.StatusMTPListingProgress, res.loaded, res.total)
		} else {
			m.SetStatus(i18n.StatusMTPListing)
		}
		return
	}
	if res.entries == nil {
		m.children[res.path] = []mtpfs.Entry{}
		m.generation++
	}
	if res.err != nil {
		m.SetStatus(i18n.StatusMTPListFailed, res.err)
		m.queueRetryAlert(i18n.StatusMTPListFailed, res.err)
		return
	}
	m.SetStatus(i18n.StatusMTPListed, res.path, len(m.children[res.path]))
}

func (m *MTPModel) RefreshDevices() {
	if m.Busy() {
		return
	}
	m.loaded = true
	m.SetStatus(i18n.StatusMTPListing)
	ch := make(chan mtpDevicesResult, 1)
	m.pendingDevices = ch
	m.generation++
	client := m.Client()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mtpDeviceTimeout)
		defer cancel()
		devs, err := client.Devices(ctx)
		ch <- mtpDevicesResult{devices: devs, err: err}
	}()
}

func (m *MTPModel) Reload() {
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
	m.children = map[string][]mtpfs.Entry{}
	m.expanded = map[string]bool{}
	m.startList(m.Root())
}

func (m *MTPModel) SelectDevice(serial string) {
	if m.Busy() || serial == "" || serial == m.serial {
		return
	}
	m.serial = serial
	m.generation++
	d := m.device(serial)
	if !d.Online() {
		m.children = nil
		m.SetStatus(i18n.StatusMTPDeviceOffline, d.State)
		return
	}
	m.beginRoot()
}

func (m *MTPModel) SelectPath(path string) {
	path = mtpfs.Clean(path)
	if m.selected == path {
		return
	}
	m.selected = path
	m.generation++
}

func (m *MTPModel) Copying() bool { return m.pendingCopy != nil }

func (m *MTPModel) ToggleExpand(path string) {
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
	if m.Busy() {
		return
	}
	m.expanded[path] = true
	m.generation++
	if _, ok := m.children[path]; !ok {
		m.startList(path)
	}
}

func (m *MTPModel) GoUp() {
	if m.Busy() || !m.CanGoUp() {
		return
	}
	m.root = mtpfs.Parent(m.Root())
	m.selected = ""
	m.expanded = map[string]bool{}
	m.children = map[string][]mtpfs.Entry{}
	m.startList(m.root)
}

func (m *MTPModel) startList(path string) {
	if m.pendingList != nil || m.serial == "" {
		return
	}
	path = mtpfs.Clean(path)
	m.SetStatus(i18n.StatusMTPListing)
	ch := make(chan mtpListResult, 8)
	m.pendingList = ch
	m.generation++
	client := m.Client()
	serial := m.serial
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mtpListTimeout)
		defer cancel()
		ctx = mtpfs.WithListProgress(ctx, func(loaded, total int, ents []mtpfs.Entry) {
			cp := append([]mtpfs.Entry(nil), ents...)
			select {
			case ch <- mtpListResult{path: path, entries: cp, loaded: loaded, total: total}:
			default:
			}
		})
		ents, err := client.List(ctx, serial, path)
		ch <- mtpListResult{path: path, entries: ents, loaded: len(ents), total: len(ents), err: err, done: true}
	}()
}

func (m *MTPModel) StartPull(local string) {
	if m.Busy() {
		return
	}
	e, ok := m.lookup(m.selected)
	if !ok {
		m.SetStatus(i18n.StatusMTPNoSelection)
		return
	}
	m.startCopy(true, e.Path, local)
}

func (m *MTPModel) StartPush(local string) {
	if m.Busy() {
		return
	}
	if m.serial == "" || !m.device(m.serial).Online() {
		m.SetStatus(i18n.StatusMTPSelectOnline)
		return
	}
	m.startCopy(false, local, m.PushDest())
}

func (m *MTPModel) startCopy(pull bool, src, dest string) {
	m.SetStatus(i18n.StatusMTPCopying)
	ch := make(chan mtpCopyResult, 1)
	m.pendingCopy = ch
	m.generation++
	client := m.Client()
	serial := m.serial
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mtpCopyTimeout)
		defer cancel()
		var n int
		var err error
		if pull {
			n, err = mtpfs.Pull(ctx, client, serial, src, dest)
		} else {
			n, err = mtpfs.Push(ctx, client, serial, src, dest)
		}
		ch <- mtpCopyResult{n: n, dest: dest, err: err, reload: !pull}
	}()
}
