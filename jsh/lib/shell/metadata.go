package shell

// MetadataProvider is the foundation discovery hook for runtime introspection.
// It allows consumers to query the current runtime profile and registered
// metadata without coupling to Shell or Repl internals.
//
// Intended uses include \help, \globals, and \profile slash commands in the
// human REPL and structured inspection in the agent REPL.
type MetadataProvider interface {
	Metadata() RuntimeMetadata
}

// MetadataProviderFunc is a function adapter that implements MetadataProvider.
// It lets callers register a discovery hook without defining a named type.
type MetadataProviderFunc func() RuntimeMetadata

// Metadata implements MetadataProvider.
func (f MetadataProviderFunc) Metadata() RuntimeMetadata { return f() }
