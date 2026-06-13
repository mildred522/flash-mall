package observability

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeromicro/go-zero/core/logx"
)

func StartDiagnostics(metricsAddr, pprofAddr string) {
	if metricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		go func() {
			if err := http.ListenAndServe(metricsAddr, mux); err != nil {
				logx.Errorf("metrics server stopped: addr=%s err=%v", metricsAddr, err)
			}
		}()
	}
	if pprofAddr != "" {
		go func() {
			if err := http.ListenAndServe(pprofAddr, nil); err != nil {
				logx.Errorf("pprof server stopped: addr=%s err=%v", pprofAddr, err)
			}
		}()
	}
}
