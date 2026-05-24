
package clustering

import (
	"testing"

	"gameclustering.com/internal/core"
)

func TestSubscriptionRegistry(t *testing.T) {
	sub1 := core.Subscription{NodeId: "p01", Tag: "presence", Topic: "message", Endpoint: "192.168.1.11:7001"}
	sub2 := core.Subscription{NodeId: "p01", Tag: "presence", Topic: "message", Endpoint: "192.168.1.11:7001"}
	sub3 := core.Subscription{NodeId: "p01", Tag: "tournament", Topic: "score", Endpoint: "192.168.1.11:7001"}
	reg := SubscriptionRegistry{topicEnds: make(map[core.TopicKey]map[string]core.Subscription)}
	reg.add(sub1)
	reg.add(sub2)
	reg.add(sub3)

	if reg.size() != 2 {
		t.Errorf("should be 2 %d", reg.size())
	}
	reg.del(sub3)
	reg.del(sub1)
	reg.del(sub2)
	if reg.size() != 0 {
		t.Errorf("should be 0 %d", reg.size())
	}
}
