package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

type CheckFunc func(ctx context.Context) error

type Server struct {
	checkFn CheckFunc
	srv     *http.Server
}

func New(ctx context.Context, checkFn CheckFunc) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := checkFn(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	return &Server{
		checkFn: checkFn,
		srv: &http.Server{
			Handler: mux,
		},
	}
}

func SetupHealthCheck(ctx context.Context, addr string, checkFn CheckFunc) error {
	srv := New(ctx, checkFn)

	ready := make(chan error, 1)

	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			ready <- err
			return
		}
		ready <- nil
		srv.srv.Serve(ln)
	}()

	return <-ready
}
