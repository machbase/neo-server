package shell

import "github.com/dop251/goja"

// RuntimeMetadata is the shared discovery container for shell and repl
// runtimes. Later phases can expose this through user-facing inspection UX.
type RuntimeMetadata struct {
	Runtime string
	Profile string
	Values  map[string]any
}

// RuntimeProfile describes prompt/banner/runtime identity metadata for a
// session without mixing Shell semantics with Repl semantics.
type RuntimeProfile struct {
	Name        string
	Description string
	Banner      func() string
	Startup     func(rt *goja.Runtime) error
	Metadata    map[string]any
}

func (p RuntimeProfile) ResolveBanner() string {
	if p.Banner == nil {
		return ""
	}
	return p.Banner()
}

func (p RuntimeProfile) RunStartup(rt *goja.Runtime) error {
	if p.Startup == nil {
		return nil
	}
	return p.Startup(rt)
}

func (p RuntimeProfile) RuntimeMetadata(runtimeName string) RuntimeMetadata {
	values := make(map[string]any, len(p.Metadata))
	for key, value := range p.Metadata {
		values[key] = value
	}
	return RuntimeMetadata{
		Runtime: runtimeName,
		Profile: p.Name,
		Values:  values,
	}
}
