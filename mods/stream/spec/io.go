package spec

type OutputStream interface {
	Write([]byte) (int, error)
	Flush() error
	Close() error
}

type InputStream interface {
	Read(p []byte) (n int, err error)
	Close() error
}
