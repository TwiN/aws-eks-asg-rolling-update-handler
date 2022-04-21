package metrics

import (
	"bytes"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricServer(t *testing.T) {

	Server.NodeGroups.WithLabelValues().Set(5)
	Server.Errors.Add(2)
	Server.ScaledUpNodes.WithLabelValues("nodeg-1").Inc()
	Server.ScaledUpNodes.WithLabelValues("nodeg-2").Inc()
	Server.ScaledDownNodes.WithLabelValues("nodeg-1").Inc()
	Server.ScaledDownNodes.WithLabelValues("nodeg-2").Inc()
	Server.OutdatedNodes.WithLabelValues("nodeg-1").Set(1)
	Server.OutdatedNodes.WithLabelValues("nodeg-2").Set(1)
	Server.UpdatedNodes.WithLabelValues("nodeg-1").Set(1)
	Server.UpdatedNodes.WithLabelValues("nodeg-2").Set(1)
	Server.DrainedNodes.WithLabelValues("nodeg-1").Inc()
	Server.DrainedNodes.WithLabelValues("nodeg-2").Inc()

	err := testutil.GatherAndCompare(prometheus.Gatherers{Server.registry}, bytes.NewBufferString(`
# HELP rolling_update_handler_drained_nodes_total The total number of drained nodes
# TYPE rolling_update_handler_drained_nodes_total counter
rolling_update_handler_drained_nodes_total{node_group="nodeg-1"} 1
rolling_update_handler_drained_nodes_total{node_group="nodeg-2"} 1
# HELP rolling_update_handler_errors The total errors
# TYPE rolling_update_handler_errors counter
rolling_update_handler_errors 2
 # HELP rolling_update_handler_node_groups The total number of node groups managed
# TYPE rolling_update_handler_node_groups gauge
rolling_update_handler_node_groups 5
# HELP rolling_update_handler_outdated_nodes The number of outdated nodes
# TYPE rolling_update_handler_outdated_nodes gauge
rolling_update_handler_outdated_nodes{node_group="nodeg-1"} 1
rolling_update_handler_outdated_nodes{node_group="nodeg-2"} 1
# HELP rolling_update_handler_scaled_down_nodes The total number of nodes scaled down
# TYPE rolling_update_handler_scaled_down_nodes counter
rolling_update_handler_scaled_down_nodes{node_group="nodeg-1"} 1
rolling_update_handler_scaled_down_nodes{node_group="nodeg-2"} 1
# HELP rolling_update_handler_scaled_up_nodes The total number of nodes scaled up
# TYPE rolling_update_handler_scaled_up_nodes counter
rolling_update_handler_scaled_up_nodes{node_group="nodeg-1"} 1
rolling_update_handler_scaled_up_nodes{node_group="nodeg-2"} 1
# HELP rolling_update_handler_updated_nodes The number of updated nodes
# TYPE rolling_update_handler_updated_nodes gauge
rolling_update_handler_updated_nodes{node_group="nodeg-1"} 1
rolling_update_handler_updated_nodes{node_group="nodeg-2"} 1
`))

	if err != nil {
		t.Errorf("Expected no errors but got: %v", err)
	}
}
