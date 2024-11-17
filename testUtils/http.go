package testUtils

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"testing"
)

func RunTestHttp(t *testing.T, f func(addr string), https bool) {
	httpServerExitDone := &sync.WaitGroup{}

	httpServerExitDone.Add(1)
	srv, addr := startHttpServer(t, httpServerExitDone, https)

	f(addr)

	if err := srv.Shutdown(context.TODO()); err != nil {
		t.Fatal(err)
	}
	httpServerExitDone.Wait()
}

func GetUrl(https bool, addr string, namespace string, site string) string {
	pre := "http"
	if https {
		pre += "s"
	}

	return fmt.Sprintf("%s://%s/%s/%s", pre, addr, namespace, site)
}

func RegisterHttpHandler(namespace string, site string, handler func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(fmt.Sprintf("/%s/%s", namespace, site), handler)
}

func startHttpServer(t *testing.T, wg *sync.WaitGroup, https bool) (*http.Server, string) {
	srv := &http.Server{}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()

	go func() {
		defer wg.Done() // let main know we are done cleaning up
		defer listener.Close()

		// always returns error. ErrServerClosed on graceful close
		if https {
			err = srv.ServeTLS(listener, "./tests/assets/localhost.crt", "./tests/assets/localhost.key")
		} else {
			err = srv.Serve(listener)
		}
		if err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	return srv, addr
}
