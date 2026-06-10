package policy

import (
	"fmt"
	"path"
	"strings"
	"time"
)

// TemplateInput carries the metadata-derived values templates may reference.
type TemplateInput struct {
	Path    string    // KB-relative path of the file
	ModTime time.Time // file modification time
}

// Expand substitutes {var} placeholders. Validation already guaranteed every
// variable is known; this only fills values.
func Expand(tmpl string, in TemplateInput) string {
	if tmpl == "" {
		return ""
	}
	base := path.Base(in.Path)
	ext := strings.TrimPrefix(path.Ext(base), ".")
	name := strings.TrimSuffix(base, path.Ext(base))

	r := strings.NewReplacer(
		"{year}", fmt.Sprintf("%04d", in.ModTime.Year()),
		"{month}", fmt.Sprintf("%02d", int(in.ModTime.Month())),
		"{day}", fmt.Sprintf("%02d", in.ModTime.Day()),
		"{name}", name,
		"{ext}", ext,
	)
	return r.Replace(tmpl)
}

// Target computes the file's destination path for a rule: Into decides the
// directory (empty = stay), Rename decides the filename (empty = keep).
// The result is cleaned and guaranteed KB-root-relative.
func (cr *CompiledRule) Target(relPath string, mod time.Time) string {
	in := TemplateInput{Path: relPath, ModTime: mod}

	dir := path.Dir(relPath)
	if cr.Into != "" {
		dir = path.Clean(Expand(cr.Into, in))
	}
	base := path.Base(relPath)
	if cr.Rename != "" {
		base = Expand(cr.Rename, in)
	}
	if dir == "." || dir == "/" {
		return path.Clean(base)
	}
	return path.Clean(dir + "/" + base)
}
