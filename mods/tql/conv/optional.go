package conv

type OptionInt struct {
	empty bool
	Value int
}

func EmptyInt() OptionInt {
	return OptionInt{empty: true}
}

func (o OptionInt) Empty() bool {
	return o.empty
}

func (o OptionInt) NonEmpty() bool {
	return !o.empty
}

func (o OptionInt) Else(v int) int {
	if o.empty {
		return v
	}
	return o.Value
}
