package sandbox

// FileAccessPolicy defines file access permissions using glob patterns.
type FileAccessPolicy struct {
	ReadGlobs  []string // globs for allowed reads; empty = allow all
	WriteGlobs []string // globs for allowed writes; empty = allow all
	DenyGlobs  []string // globs that are always denied (takes precedence)
}
