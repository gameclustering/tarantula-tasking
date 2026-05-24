package clustering

import (
	"strings"

	"gameclustering.com/internal/core"
)

type sub func(sub core.Subscription)

type SubscriptionRegistry struct {
	topicEnds map[core.TopicKey]map[string]core.Subscription
	cPools    map[core.TopicKey]*core.RpcConnPool
	auth core.Authenticator
}

func (s *SubscriptionRegistry) add(sub core.Subscription) {
	subs, exists := s.topicEnds[sub.TopicKey()]
	if !exists {
		subs = make(map[string]core.Subscription)
		s.topicEnds[sub.TopicKey()] = subs
		cp := core.RpcConnPool{Target: sub.Endpoint, Tag: sub.Tag, NodeId: sub.NodeId,Auth: s.auth}
		cp.Start()
		s.cPools[sub.TopicKey()] = &cp
	}
	_, exists = subs[sub.Key()]
	if exists {
		return
	}
	subs[sub.Key()] = sub
}

func (s *SubscriptionRegistry) del(sub core.Subscription) {
	subs, exists := s.topicEnds[sub.TopicKey()]
	if !exists {
		return
	}
	delete(subs, sub.Key())
	if len(subs) > 0 {
		return
	}
	s.cPools[sub.TopicKey()].Release()
	delete(s.cPools, sub.TopicKey())
	delete(s.topicEnds, sub.TopicKey())
}

func (s *SubscriptionRegistry) size() int {
	return len(s.topicEnds)
}

func (s *SubscriptionRegistry) topic(req TopicRequest) []core.Subscription {
	subs := make([]core.Subscription, 0)
	for k := range s.topicEnds {
		if req.Name == k.Topic {
			cp := s.cPools[k]
			sub := core.Subscription{Topic: k.Topic, Endpoint: k.Endpoint, CPool: cp}
			subs = append(subs, sub)
		}
	}
	return subs
}

func (s *SubscriptionRegistry) list(prefixed bool) []core.Subscription {
	subs := make([]core.Subscription, 0)
	if prefixed {
		for k, s := range s.topicEnds {
			if strings.HasPrefix(k.Topic, TRANS_SUB_PREFIX) {
				for _, v := range s {
					sub := core.Subscription{NodeId: v.NodeId, Tag: v.Tag, Topic: v.Topic, Endpoint: k.Endpoint}
					subs = append(subs, sub)
				}
			}
		}
		return subs
	}
	for k, s := range s.topicEnds {
		if !strings.HasPrefix(k.Topic, TRANS_SUB_PREFIX) {
			for _, v := range s {
				sub := core.Subscription{NodeId: v.NodeId, Tag: v.Tag, Topic: v.Topic, Endpoint: k.Endpoint}
				subs = append(subs, sub)
			}
		}
	}
	return subs
}

func (s *SubscriptionRegistry) lookup(sub sub) {
	for _, v := range s.topicEnds {
		for _, b := range v {
			sub(b)
		}
	}
}
