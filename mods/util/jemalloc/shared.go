package jemalloc

const JEMALLOC_STRING = "jemalloc"

var Enabled bool = false

type Stat struct {
	Active   int64
	Resident int64
}
