package config

// MihomoConfig represents the full structure of Mihomo configuration
type MihomoConfig struct {
	Mode               string                   `yaml:"mode,omitempty"`
	Port               int                      `yaml:"port,omitempty"`
	SocksPort          int                      `yaml:"socks-port,omitempty"`
	MixedPort          int                      `yaml:"mixed-port,omitempty"`
	AllowLan           bool                     `yaml:"allow-lan,omitempty"`
	LogLevel           string                   `yaml:"log-level,omitempty"`
	IPv6               bool                     `yaml:"ipv6,omitempty"`
	ExternalController string                   `yaml:"external-controller,omitempty"`
	Secret             string                   `yaml:"secret,omitempty"`
	DNS                map[string]interface{}   `yaml:"dns,omitempty"`
	Listeners          []map[string]interface{} `yaml:"listeners,omitempty"`
	Proxies            []map[string]interface{} `yaml:"proxies,omitempty"`
	ProxyGroups        []map[string]interface{} `yaml:"proxy-groups,omitempty"`
	Rules              []string                 `yaml:"rules,omitempty"`
	// GeodataLoader selects how the core reads GEO rule data. Left empty by
	// default so the key never appears in generated YAML and every existing
	// installation keeps whatever behaviour its core version ships with.
	// 'memconservative' is the loader built for memory-constrained devices —
	// unlike the 'standard' loader it does not lean on mmap-backed data, which
	// on a swapless small-RAM host is exactly what pushes the kernel into OOM.
	// See CoreLowMemory() for when this is applied.
	GeodataLoader string `yaml:"geodata-loader,omitempty"`
}
