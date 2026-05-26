package conf

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Services ServicesConfig `yaml:"services"`
}

type ServicesConfig struct {
	Gateway []ServerNode `yaml:"gateway"`
	ChatSvr []ServerNode `yaml:"chatsvr"`
}

type ServerNode struct {
	ID         string `yaml:"id"`
	TCPListen  string `yaml:"tcp_listen"`
	WSListen   string `yaml:"ws_listen"`
	Listen     string `yaml:"listen"`
	PublicAddr string `yaml:"public_addr"`
}

var GlobalConfig *Config

func Load(path string) error {
	GlobalConfig = &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, GlobalConfig); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	return nil
}
