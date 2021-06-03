package server

import (
	"context"
	"log"
	"net/http"

	"github.com/sirupsen/logrus"
)

func NewServer(listenAddr, fileName, sourceType string, parameters map[string]string) error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()
	errorCh := make(chan error)
	srv := &http.Server{
		Addr: listenAddr,
	}
	fp, err := LaunchFileProvisioner(ctx, errorCh, fileName, sourceType, parameters)
	if err != nil {
		return err
	}
	srv.Handler = NewRouter(fp)

	go func() {
		err := <-errorCh
		cancelFunc()
		if err != nil {
			log.Fatalf("Failed to get file %v from source type %v: %v", fileName, sourceType, err)
			return
		}
		srv.Close()
		logrus.Info("Closed file provisioning server")
	}()

	logrus.Infof("Starting file provision server at %v", listenAddr)

	return srv.ListenAndServe()
}
