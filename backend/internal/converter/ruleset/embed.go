package ruleset

import _ "embed"

//go:embed geosite-cn.srs
var GeositeCN []byte

//go:embed geoip-cn.srs
var GeoipCN []byte
