package main

import (
	"os"
)

func newWrapper(opt options) (result *wrapper, err error) {
	srv, err := newServer(opt)
	if err != nil {
		return nil, err
	}
	proc, err := newProcess(opt)
	if err != nil {
		return nil, err
	}

	result = &wrapper{
		server:  srv,
		process: proc,
	}

	return result, nil
}

type wrapper struct {
	server  *server
	process *process
}

func (w *wrapper) run() (int, error) {
	go func() {
		if err := w.server.serve(); err != nil {
			w.server.logger.WithError(err).Fatal("failed to serve")
			os.Exit(27)
		}
	}()

	return w.process.wait()
}

func (w *wrapper) Close() (rErr error) {
	defer func() {
		if err := w.server.Close(); err != nil && rErr == nil {
			rErr = err
		}
	}()
	defer func() {
		if err := w.process.Close(); err != nil && rErr == nil {
			rErr = err
		}
	}()
	return nil
}
