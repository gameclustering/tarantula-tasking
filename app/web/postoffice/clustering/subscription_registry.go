package clustering

import (
	"sort"
	"strings"

	"gameclustering.com/internal/core"
)

type sub func(sub core.Subscription)

type SubscriptionRegistry struct {
	topicEnds map[core.TopicKey]map[string]core.Subscription
	cPools    map[core.TopicKey]*core.RpcConnPool
	roundIdx  map[string]int // round-robin counter per "topic#tag"
	auth      core.Authenticator
	caCert    []byte
}

func (s *SubscriptionRegistry) add(sub core.Subscription) {
	subs, exists := s.topicEnds[sub.TopicKey()]
	if !exists {
		subs = make(map[string]core.Subscription)
		s.topicEnds[sub.TopicKey()] = subs
		cp := core.RpcConnPool{Target: sub.Endpoint, Tag: sub.Tag, NodeId: sub.NodeId, Auth: s.auth, CACert: s.caCert}
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

// pick returns the next subscriber for the given task name and tag using
// round-robin. Tag scopes the pool to a specific environment; empty tag
// matches any subscriber. Each call advances the counter independently,
// so reserve and confirm phases can land on different nodes.
func (s *SubscriptionRegistry) pick(name, tag string) *core.Subscription {
	type entry struct {
		ep     string
		cp     *core.RpcConnPool
		nodeId string
	}
	entries := make([]entry, 0)
	for k, subMap := range s.topicEnds {
		if k.Topic != name {
			continue
		}
		for _, sub := range subMap {
			if tag == "" || sub.Tag == tag {
				entries = append(entries, entry{ep: k.Endpoint, cp: s.cPools[k], nodeId: sub.NodeId})
				break // one entry per endpoint
			}
		}
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ep < entries[j].ep })
	key := name + "#" + tag
	idx := s.roundIdx[key] % len(entries)
	s.roundIdx[key]++
	e := entries[idx]
	result := core.Subscription{Topic: name, Endpoint: e.ep, CPool: e.cp, NodeId: e.nodeId}
	return &result
}

// pickByNodeId returns the subscriber with the given NodeId, used to route
// AskFinish to the exact node that handled AskReserve.
func (s *SubscriptionRegistry) pickByNodeId(name, nodeId string) *core.Subscription {
	for k, subMap := range s.topicEnds {
		if k.Topic != name {
			continue
		}
		for _, sub := range subMap {
			if sub.NodeId == nodeId {
				cp := s.cPools[k]
				result := core.Subscription{Topic: name, Endpoint: k.Endpoint, CPool: cp, NodeId: sub.NodeId}
				return &result
			}
		}
	}
	return nil
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
