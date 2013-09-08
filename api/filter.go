package api

import (
	"github.com/wakarimasenco/streamingchan/fourchan"
	"strings"
)

type Filter struct {
	Board   string
	Thread  int
	Comment string
	Name    string
	Trip    string
	Email   string
}

func (f *Filter) Passes(post fourchan.Post) bool {
	if f.Board != "" && post.Board != f.Board {
		return false
	}
	if f.Thread != 0 && (post.No != f.Thread && post.Resto != f.Thread) {
		return false
	}
	if f.Comment != "" && !strings.Contains(strings.ToLower(post.Com), strings.ToLower(f.Comment)) {
		return false
	}
	if f.Name != "" && !strings.Contains(strings.ToLower(post.Name), strings.ToLower(f.Name)) {
		return false
	}
	if f.Trip != "" && !strings.Contains(strings.ToLower(post.Trip), strings.ToLower(f.Trip)) {
		return false
	}
	if f.Email != "" && !strings.Contains(strings.ToLower(post.Email), strings.ToLower(f.Email)) {
		return false
	}
	return true
}
