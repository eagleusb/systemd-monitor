package main

type message struct {
	buf []byte
	len int // minimum length
}

func (m *message) writeBytes(p []byte) {
	m.buf = append(m.buf, p...)
}

func (m *message) writeEmail(name, email string) {
	if name == "" {
		m.write(email)
	} else {
		m.write(name)
		m.write(" <")
		m.write(email)
		m.writeByte('>')
	}
}

func (m *message) write(s string) {
	m.buf = append(m.buf, s...)
}

func (m *message) writeByte(b byte) {
	m.buf = append(m.buf, b)
}

func (m *message) initialized() {
	m.len = len(m.buf)
}

func (m *message) reset() {
	m.buf = m.buf[:m.len]
}
