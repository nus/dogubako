package mtpfs

import "fmt"

type propObject struct {
	handle    uint32
	format    uint16
	size      uint64
	name      string
	modDate   string
	parent    uint32
	hasFormat bool
	hasSize   bool
	hasParent bool
}

func (r *byteReader) skipMTPValue(dt uint16) error {
	if dt == dtString {
		_, err := r.mtpString()
		return err
	}
	if dt&0x4000 != 0 {
		n, err := r.u32()
		if err != nil {
			return err
		}
		elem := dt &^ 0x4000
		for i := 0; i < int(n); i++ {
			if err := r.skipMTPValue(elem); err != nil {
				return err
			}
		}
		return nil
	}
	switch dt {
	case 0x0001, 0x0002:
		return r.skip(1)
	case 0x0003, dtUint16:
		return r.skip(2)
	case 0x0005, dtUint32:
		return r.skip(4)
	case 0x0007, dtUint64:
		return r.skip(8)
	case 0x0009, 0x000a:
		return r.skip(16)
	default:
		return fmt.Errorf("unknown MTP datatype 0x%04x", dt)
	}
}

func parseObjectPropList(b []byte) ([]propObject, error) {
	r := byteReader{b: b}
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	byHandle := map[uint32]*propObject{}
	order := make([]uint32, 0, n)
	for i := uint32(0); i < n; i++ {
		handle, err := r.u32()
		if err != nil {
			return nil, err
		}
		code, err := r.u16()
		if err != nil {
			return nil, err
		}
		dt, err := r.u16()
		if err != nil {
			return nil, err
		}
		obj := byHandle[handle]
		if obj == nil {
			obj = &propObject{handle: handle}
			byHandle[handle] = obj
			order = append(order, handle)
		}
		if err := applyPropValue(obj, code, dt, &r); err != nil {
			return nil, err
		}
	}
	out := make([]propObject, 0, len(order))
	for _, h := range order {
		out = append(out, *byHandle[h])
	}
	return out, nil
}

func applyPropValue(obj *propObject, code, dt uint16, r *byteReader) error {
	switch code {
	case propObjectFormat:
		v, err := readUintProp(r, dt)
		if err != nil {
			return err
		}
		obj.format = uint16(v)
		obj.hasFormat = true
		return nil
	case propObjectSize:
		v, err := readUintProp(r, dt)
		if err != nil {
			return err
		}
		obj.size = v
		obj.hasSize = true
		return nil
	case propParentObject:
		v, err := readUintProp(r, dt)
		if err != nil {
			return err
		}
		obj.parent = uint32(v)
		obj.hasParent = true
		return nil
	case propObjectFileName:
		s, err := readStringProp(r, dt)
		if err != nil {
			return err
		}
		obj.name = s
		return nil
	case propDateModified:
		s, err := readStringProp(r, dt)
		if err != nil {
			return err
		}
		obj.modDate = s
		return nil
	default:
		return r.skipMTPValue(dt)
	}
}

func readUintProp(r *byteReader, dt uint16) (uint64, error) {
	switch dt {
	case 0x0001, 0x0002:
		v, err := r.u8()
		return uint64(v), err
	case 0x0003, dtUint16:
		v, err := r.u16()
		return uint64(v), err
	case 0x0005, dtUint32:
		v, err := r.u32()
		return uint64(v), err
	case 0x0007, dtUint64:
		return r.u64()
	default:
		if err := r.skipMTPValue(dt); err != nil {
			return 0, err
		}
		return 0, nil
	}
}

func readStringProp(r *byteReader, dt uint16) (string, error) {
	if dt != dtString {
		if err := r.skipMTPValue(dt); err != nil {
			return "", err
		}
		return "", nil
	}
	return r.mtpString()
}

func mergePropObjects(dst map[uint32]*propObject, src []propObject) {
	for i := range src {
		p := src[i]
		e := dst[p.handle]
		if e == nil {
			cp := p
			dst[p.handle] = &cp
			continue
		}
		if p.hasFormat {
			e.format = p.format
			e.hasFormat = true
		}
		if p.hasSize {
			e.size = p.size
			e.hasSize = true
		}
		if p.hasParent {
			e.parent = p.parent
			e.hasParent = true
		}
		if p.name != "" {
			e.name = p.name
		}
		if p.modDate != "" {
			e.modDate = p.modDate
		}
	}
}

func propMapSlice(m map[uint32]*propObject) []propObject {
	out := make([]propObject, 0, len(m))
	for _, p := range m {
		out = append(out, *p)
	}
	return out
}

func isPropChild(p propObject, parentHandle uint32, storage bool) bool {
	if parentHandle != 0 && parentHandle != handleRoot && p.handle == parentHandle {
		return false
	}
	if p.hasParent {
		if storage {
			return p.parent == 0 || p.parent == handleRoot
		}
		return p.parent == parentHandle
	}
	return p.handle != parentHandle
}
