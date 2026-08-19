package view

import _ "embed"

//go:embed static/balemoh.css
var balemohCSS []byte

func BalemohCSS() []byte { return balemohCSS }
