package queue

import (
	"reflect"
	"sync"
)

// ChannelBackend uses go channels to manage queues
// Suitable for multithreaded single process environment
type ChannelBackend struct {
	*sync.Mutex
	channels map[string]chan interface{}
	buffer   int
}

// NewChannelBackend creates new channels backend
func NewChannelBackend() *ChannelBackend {
	b := &ChannelBackend{Mutex: &sync.Mutex{}}
	return b.Channels(make(map[string]chan interface{})).Buffer(1000)
}

// Channels sets initial channels (queues). Key - queue name, value - go channel
func (b *ChannelBackend) Channels(channels map[string]chan interface{}) *ChannelBackend {
	b.channels = channels
	return b
}

// Buffer sets default buffer for new channels created by the backend. Default value is 1000.
func (b *ChannelBackend) Buffer(buffer int) *ChannelBackend {
	b.buffer = buffer
	return b
}

// Put adds value to the end of a queue.
func (b *ChannelBackend) Put(queueName string, value interface{}) error {
	b.getChannel(queueName) <- value
	return nil
}

// Get removes the first element from a queue and put it in the value pointed to by v
func (b *ChannelBackend) Get(queueName string, v interface{}) error {
	result := <-b.getChannel(queueName)
	value := reflect.ValueOf(v)
	value.Elem().Set(reflect.ValueOf(result))
	return nil
}

func (b *ChannelBackend) RemoveQueue(queueName string) error  {
	b.Lock()
	defer b.Unlock()
	delete(b.channels, queueName)
	return nil
}

func (b *ChannelBackend) getChannel(queueName string) chan interface{} {
	b.Lock()
	defer b.Unlock()
	result, ok := b.channels[queueName]

	if !ok {
		result = make(chan interface{}, b.buffer)
		b.channels[queueName] = result
	}

	return result
}
