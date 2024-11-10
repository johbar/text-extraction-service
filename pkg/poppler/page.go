package puregopoppler

// Text returns the pages textual content
func (p *Page) Text() string {
	txtPtr := poppler_page_get_text(p.uintptr)
	return toStr(txtPtr)
}

// Close closes the page, freeing up resources allocated when the page was opened
func (p *Page) Close() {
	if p.uintptr != 0 {
		g_object_unref(p.uintptr)
		p.uintptr = 0
	}
	// do nothing if null pointer
}
