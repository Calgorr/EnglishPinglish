package config

import "github.com/spf13/viper"

type Config struct {
	Server ServerConfig `yaml:"server"`
	Redis  RedisConfig  `yaml:"redis"`
	Ninja  NinjaConfig  `yaml:"ninja"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type RedisConfig struct {
	Addr string `yaml:"addr"`
	TTL  int    `yaml:"ttl"`
}

type NinjaConfig struct {
	NinjaAPIKey        string `yaml:"ninja_api_key"`
	NinjaDictionaryURL string `yaml:"ninja_dictionary_url"`
	NinjaRandomURL     string `yaml:"ninja_random_url"`
}

func LoadConfig(filename string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(filename)
	v.SetConfigType("yml")
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}

func ParseConfig(v *viper.Viper) (*Config, error) {
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
