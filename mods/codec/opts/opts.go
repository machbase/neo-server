package opts

type Option func(enc any)

type CanSetHttpHeader interface {
	SetHttpHeader(key, value string)
}

func HttpHeader(k string, v string) Option {
	return func(enc any) {
		if e, ok := enc.(CanSetHttpHeader); ok {
			e.SetHttpHeader(k, v)
		}
	}
}
