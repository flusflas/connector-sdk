// Copyright (c) OpenFaaS Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package types

import (
	"sync"
)

type MatchTopicFunc func(topicReceived, topicFunction string) bool

func defaultMatchTopic(topicReceived, topicFunction string) bool {
	return topicReceived == topicFunction
}

func NewTopicMap(matchFunc MatchTopicFunc) *TopicMap {
	lookup := make(map[string][]string)

	if matchFunc == nil {
		matchFunc = defaultMatchTopic
	}

	return &TopicMap{
		lookup:    &lookup,
		lock:      sync.RWMutex{},
		matchFunc: matchFunc,
	}
}

type TopicMap struct {
	lookup    *map[string][]string
	lock      sync.RWMutex
	matchFunc MatchTopicFunc
}

func (t *TopicMap) Match(topicName string) []string {
	t.lock.RLock()
	defer t.lock.RUnlock()
	var values []string

	matchFunc := t.matchFunc
	if matchFunc == nil {
		matchFunc = defaultMatchTopic
	}

	for key, functions := range *t.lookup {
		if matchFunc(topicName, key) {
			values = append(values, functions...)
		}
	}

	return values
}

func (t *TopicMap) Sync(updated *map[string][]string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.lookup = updated
}

func (t *TopicMap) Topics() []string {
	t.lock.RLock()
	defer t.lock.RUnlock()

	topics := make([]string, 0, len(*t.lookup))
	for topic := range *t.lookup {
		topics = append(topics, topic)
	}

	return topics
}
