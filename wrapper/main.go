package main

import (
	"os"
	"os/signal"

	"github.com/echocat/slf4g"
	_ "github.com/echocat/slf4g/native"
	_ "github.com/echocat/slf4g/sdk/bridge/hook"
)

func main() {
	var opts options
	if err := opts.readFromDefaultFile(); err != nil {
		log.WithError(err).
			Fatal()
		os.Exit(21)
	}
	if err := opts.ensureSecretsFromDefaultFile(); err != nil {
		log.WithError(err).
			Fatal()
		os.Exit(21)
	}

	w, err := newWrapper(opts)
	if err != nil {
		log.WithError(err).Fatal("cannot initialize wrapper")
		os.Exit(22)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs)

	go func() {
		for sig := range sigs {
			w.process.signal(sig)
		}
	}()

	if ec, err := w.run(); err != nil {
		log.WithError(err).Fatal("wrapper execution failed")
		os.Exit(26)
	} else {
		os.Exit(ec)
	}
}
