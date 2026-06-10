package policy

import (
	"mime"
	"path"
	"regexp"
	"strings"
)

// CompiledRule is a Rule with its matchers precompiled.
type CompiledRule struct {
	Rule
	contentRe *regexp.Regexp
}

// Compile precompiles every rule's matchers. Policy must already be validated.
func (p *Policy) Compile() ([]CompiledRule, error) {
	out := make([]CompiledRule, 0, len(p.Rules))
	for _, r := range p.Rules {
		cr := CompiledRule{Rule: r}
		if r.Match.ContentMatches != "" {
			re, err := regexp.Compile("(?i)" + r.Match.ContentMatches)
			if err != nil {
				return nil, err
			}
			cr.contentRe = re
		}
		out = append(out, cr)
	}
	return out, nil
}

// ContentFunc lazily provides a bounded prefix of a file's content (the caller
// decides the cap). It is only called when a rule actually needs content
// (cheapest-first).
type ContentFunc func(relPath string) ([]byte, error)

// Matches reports whether the file at relPath satisfies every set condition.
func (cr *CompiledRule) Matches(relPath string, content ContentFunc) (bool, error) {
	base := path.Base(relPath)
	ext := strings.TrimPrefix(path.Ext(base), ".")

	if len(cr.Match.Ext) > 0 && !containsFold(cr.Match.Ext, ext) {
		return false, nil
	}
	if len(cr.Match.Mime) > 0 {
		mt := mime.TypeByExtension("." + ext)
		if mt == "" || !mimeMatch(cr.Match.Mime, mt) {
			return false, nil
		}
	}
	if cr.Match.Path != "" {
		ok, _ := path.Match(cr.Match.Path, relPath)
		if !ok {
			// Also try matching just the basename so "*.pdf" works at any depth.
			if ok2, _ := path.Match(cr.Match.Path, base); !ok2 {
				return false, nil
			}
		}
	}
	if cr.contentRe != nil {
		if content == nil {
			return false, nil
		}
		buf, err := content(relPath)
		if err != nil {
			return false, err
		}
		if !cr.contentRe.Match(buf) {
			return false, nil
		}
	}
	return true, nil
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// mimeMatch supports exact ("application/pdf") and prefix-wildcard
// ("image/*") patterns against a detected mime type.
func mimeMatch(patterns []string, mt string) bool {
	// strip parameters: "text/plain; charset=utf-8" → "text/plain"
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = strings.TrimSpace(mt[:i])
	}
	for _, p := range patterns {
		if strings.HasSuffix(p, "/*") {
			if strings.HasPrefix(mt, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if strings.EqualFold(p, mt) {
			return true
		}
	}
	return false
}
