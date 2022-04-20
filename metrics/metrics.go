package metrics

import (
	"net/http"
	"reflect"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	namespace = "rolling_update_handler"
	Server    *metricServer
)

type metricServer struct {
	registry *prometheus.Registry

	NodeGroups      *prometheus.GaugeVec
	OutdatedNodes   *prometheus.GaugeVec
	UpdatedNodes    *prometheus.GaugeVec
	ScaledUpNodes   *prometheus.CounterVec
	ScaledDownNodes *prometheus.CounterVec
	DrainedNodes    *prometheus.CounterVec
	Errors          prometheus.Counter
}

func init() {
	Server = newMetricServer()
}

func newMetricServer() *metricServer {
	m := &metricServer{
		registry: prometheus.NewPedanticRegistry(),
		NodeGroups: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "node_groups",
			Help:      "The total number of node groups managed"},
			[]string{}),
		OutdatedNodes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "outdated_nodes",
			Help:      "The number of outdated nodes",
		}, []string{"node_group"}),
		UpdatedNodes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "updated_nodes",
			Help:      "The number of updated nodes",
		}, []string{"node_group"}),
		ScaledUpNodes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scaled_up_nodes",
			Help:      "The total number of nodes scaled up",
		}, []string{"node_group"}),
		ScaledDownNodes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scaled_down_nodes",
			Help:      "The total number of nodes scaled down",
		}, []string{"node_group"}),
		DrainedNodes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "drained_nodes_total",
			Help:      "The total number of drained nodes",
		}, []string{"node_group"}),
		Errors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "errors",
			Help:      "The total errors",
		}),
	}

	m.register()

	return m
}

func (m *metricServer) register() {
	v := reflect.ValueOf(*m)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).CanInterface() {
			if metric, ok := v.Field(i).Interface().(prometheus.Collector); ok {
				m.registry.MustRegister(metric)
			}
		}
	}
}

func (m *metricServer) Listen(port int) error {
	gatherers := prometheus.Gatherers{prometheus.DefaultGatherer, m.registry}
	http.Handle("/metrics", promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{}))
	return http.ListenAndServe(":"+strconv.Itoa(port), nil)
}
