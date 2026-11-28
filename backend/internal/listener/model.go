package listener

// MihomoConfig represents the root structure of a generated Mihomo configuration
type MihomoConfig struct {
	Listeners []map[string]interface{} `yaml:"listeners"`
}
