package logic

import (
	"context"
	"net"
	"testing"

	"flash-mall/app/order/api/internal/svc"
)

func TestSystemHealthCheckTCPFallsBackForDockerHost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener addr: %v", err)
	}

	l := NewSystemHealthLogic(context.Background(), &svc.ServiceContext{})
	ok, detail := l.checkTCP(net.JoinHostPort("host.docker.internal", port))
	if !ok {
		t.Fatalf("expected docker host fallback to connect, detail=%s", detail)
	}

	<-done
}
