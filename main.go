package main

import (
	"flag"
	stdlog "log"
	"net/http"

	"github.com/Giantmen/prophet/config"
	"github.com/Giantmen/prophet/controller"
	"github.com/Giantmen/prophet/log"

	"github.com/BurntSushi/toml"
)

var (
	cfgPath = flag.String("config", "config.toml", "config file path")
)

func initLog(cfg *config.Config) {
	log.SetLevelByString(cfg.LogLevel)
	if !cfg.Debug {
		log.SetHighlighting(false)
		err := log.SetOutputByName(cfg.LogPath)
		if err != nil {
			log.Fatal(err)
		}
		log.SetRotateByDay()
	}
}

func main() {
	flag.Parse()
	var cfg config.Config
	_, err := toml.DecodeFile(*cfgPath, &cfg)
	if err != nil {
		stdlog.Fatal("DecodeConfigFile error: ", err)
	}
	log.Info("config:", cfg)
	initLog(&cfg)

	done := make(chan struct{})
	http.HandleFunc("/prophet/stop", func(w http.ResponseWriter, r *http.Request) {
		done <- struct{}{}
	})

	go func() {
		stdlog.Fatal(http.ListenAndServe(cfg.Listen, nil))
	}()

	ctr, err := controller.NewController(&cfg, done)
	if err != nil {
		log.Fatal(err)
	}
	ctr.Run()
}
