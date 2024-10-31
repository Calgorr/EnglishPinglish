package main

import "github.com/Calgorr/EnglishPinglish/config"

func main() {
	viper, err := config.LoadConfig("./config/config.yml")
	if err != nil {
		panic(err)
	}
	cfg, err := config.ParseConfig(viper)
	if err != nil {
		panic(err)
	}
	server := internal.NewRegisterService(cfg)
	if err = server.Start(); err != nil {
		panic(err)
	}
}
