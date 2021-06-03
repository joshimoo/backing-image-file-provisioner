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

	// TODO: the producer should be responsible for creation and closing of the error channel
	// you can use a buffered error channel of size[1] if you want to treat errors as terminal
	// creation example: https://github.com/longhorn/sparse-tools/blob/336a5f7c1ce4c9e3e5e17b554067d486a318c9fd/sparse/client.go#L79
	// usage example + discussion: https://github.com/longhorn/sparse-tools/pull/76/commits/336a5f7c1ce4c9e3e5e17b554067d486a318c9fd#r639390423
	errorCh := make(chan error)
	srv := &http.Server{
		Addr: listenAddr,
	}
	fp, err := LaunchFileProvisioner(ctx, errorCh, fileName, sourceType, parameters)
	if err != nil {
		return err
	}
	srv.Handler = NewRouter(fp)

	// TODO: replace this with a select on errc, ctx.Done once NewServer gets passed a ctx
	// The initial context should be created at the beginning of the program, so you can hook up a signal handler
	go func() {
		// ensure cleanup
		defer func() {
			cancelFunc()
			srv.Close()
			logrus.Info("Closed file provisioning server")
		}()

		err := <-errorCh
		if err != nil {
			log.Fatalf("Failed to get file %v from source type %v: %v", fileName, sourceType, err)
			return
		}
	}()

	logrus.Infof("Starting file provision server at %v", listenAddr)

	return srv.ListenAndServe()
}
