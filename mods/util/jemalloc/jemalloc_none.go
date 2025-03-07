//go:build !(linux && amd64 && debug)

package jemalloc

func HeapStat(stat *Stat) {
}
