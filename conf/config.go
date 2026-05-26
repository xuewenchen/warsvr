package conf

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Services ServicesConfig `yaml:"services"`
}

// ServicesConfig maps backend names to their server node lists.
// Keys are service names like "gateway", "chatsvr", etc.
type ServicesConfig map[string][]ServerNode

type ServerNode struct {
	ID         string `yaml:"id"`
	TCPListen  string `yaml:"tcp_listen"`
	WSListen   string `yaml:"ws_listen"`
	Listen     string `yaml:"listen"`
	PublicAddr string `yaml:"public_addr"`
}

func ParseHostPort(addr string) (string, int) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		panic("invalid address: " + addr)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		panic("invalid port: " + addr)
	}
	return parts[0], port
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
