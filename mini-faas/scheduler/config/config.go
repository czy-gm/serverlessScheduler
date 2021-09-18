package config

const (
	DefaultPort = 10450
)

var Global = &Config{}

type Config struct {
	Region    string
	StackName string
	HostName  string
}
