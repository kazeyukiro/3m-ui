package ruleset

import _ "embed"

//go:embed cnsite.srs
var CNSite []byte

//go:embed cnip.srs
var CNIP []byte

//go:embed gfw.srs
var GFW []byte
