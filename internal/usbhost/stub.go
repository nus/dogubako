//go:build !darwin

package usbhost

func list() ([]Info, error) {
	return nil, ErrUnsupported
}

func open(string) (Conn, error) {
	return nil, ErrUnsupported
}
