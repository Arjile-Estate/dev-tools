package config

type ConfigLoader interface {
	LoadConfig(path string) (*Config, error)
}
