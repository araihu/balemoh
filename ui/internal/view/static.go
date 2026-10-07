package view

import _ "embed"

//go:embed static/balemoh.css
var balemohCSS []byte

func BalemohCSS() []byte { return balemohCSS }

//go:embed static/social-v1.png
var socialPreview []byte

func SocialPreview() []byte { return socialPreview }

//go:embed static/host-live.js
var hostLiveJS []byte

func HostLiveJS() []byte { return hostLiveJS }
