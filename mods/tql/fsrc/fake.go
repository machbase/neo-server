package fsrc

type FakeSource interface {
	get() int
}

var _ FakeSource = &oscilatorSource{}

type oscilatorSource struct {
}

func (fs *oscilatorSource) get() int {
	return 0
}
