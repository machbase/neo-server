package tagql

import "errors"

type Vec []any

func NewVec(e ...any) Vec {
	return Vec(e)
}

func (vec Vec) Length() int {
	return len(vec)
}

func (vec Vec) At(idx int) any {
	if idx < 0 || idx >= len(vec) {
		return nil
	}
	return vec[idx]
}

func (vec Vec) Append(e any) Vec {
	return append(vec, e)
}

func (vec Vec) Remove(idx int) (Vec, error) {
	if idx < 0 || idx >= len(vec) {
		return nil, errors.New("out of index")
	}
	h := []any{}
	t := []any{}
	if idx > 0 {
		h = vec[0:idx]
	}
	if idx < len(vec)-1 {
		t = vec[idx+1:]
	}
	return append(h, t...), nil
}

func (vec Vec) InsertAt(idx int, v any) (Vec, error) {
	if idx < 0 || idx >= len(vec) {
		return nil, errors.New("out of index")
	}
	return append(vec[0:idx], append([]any{v}, vec[idx:]...)...), nil
}
