package queue

import (
	"context"
	"sync"
	"time"

	"github.com/jolestar/go-commons-pool"
	"github.com/kr/beanstalk"
)

var poolTubes = sync.Pool{
	New: func() interface{} {
		return &beanstalk.Tube{}
	},
}

var poolTubeSets = sync.Pool{
	New: func() interface{} {
		return &beanstalk.TubeSet{}
	},
}

// BeanstalkBackend provides beanstalk-based backend to manage queues.
// Suitable for multi-host, multi-process and multithreaded environment
type BeanstalkBackend struct {
	pool        *pool.ObjectPool
	codec       Codec
	poolContext context.Context
}

// NewBeanstalkBackend creates new BeanstalkBackend
func NewBeanstalkBackend(addr string) (*BeanstalkBackend, error) {
	ctx := context.Background()
	b := &BeanstalkBackend{pool: getBeanstalkPool(ctx, addr), poolContext: ctx}
	return b.Codec(NewGOBCodec()), nil
}

// Codec sets codec to encode/decode objects in queues. GOBCodec is default.
func (b *BeanstalkBackend) Codec(c Codec) *BeanstalkBackend {
	b.codec = c
	return b
}

// Put adds value to the end of a queue.
func (b *BeanstalkBackend) Put(queueName string, value interface{}) error {
	conn, err := b.getConn()

	if err != nil {
		return err
	}

	defer b.pool.ReturnObject(b.poolContext, conn)

	data, err := b.codec.Marshal(value)

	if err != nil {
		return err
	}

	tube := poolTubes.Get().(*beanstalk.Tube)
	defer poolTubes.Put(tube)
	tube.Conn = conn
	tube.Name = queueName

	_, err = tube.Put(data, 1, 0, 0)
	return err
}

// Get removes the first element from a queue and put it in the value pointed to by value
func (b *BeanstalkBackend) Get(queueName string, value interface{}) error {
	conn, err := b.getConn()

	if err != nil {
		return err
	}

	defer b.pool.ReturnObject(b.poolContext, conn)

	tube := poolTubeSets.Get().(*beanstalk.TubeSet)
	defer poolTubeSets.Put(tube)
	tube.Conn = conn
	tube.Name = map[string]bool{queueName: true}
	for {
		id, data, err := tube.Reserve(time.Minute)

		if err == nil {
			err = b.codec.Unmarshal(data, value)

			if err != nil {
				return err
			}

			err = conn.Delete(id)

			if err != nil {
				return err
			}

			return nil

		} else if err != beanstalk.ErrTimeout {
			return err
		}
	}
}

func (b *BeanstalkBackend) RemoveQueue(queueName string) error {
	return nil
}

func (b *BeanstalkBackend) getConn() (*beanstalk.Conn, error) {
	o, err := b.pool.BorrowObject(b.poolContext)

	if err != nil {
		return nil, err
	}

	return o.(*beanstalk.Conn), nil
}

func getBeanstalkPool(ctx context.Context, addr string) *pool.ObjectPool {
	config := pool.NewDefaultPoolConfig()
	config.MaxTotal = -1
	// config.SoftMinEvictableIdleTimeMillis = 1000 * 60
	// config.TimeBetweenEvictionRunsMillis = 1000 * 60
	return pool.NewObjectPool(
		ctx,
		pool.NewPooledObjectFactory(
			func(ctx context.Context) (interface{}, error) {
				return beanstalk.Dial("tcp", addr)
			},
			func(ctx context.Context, object *pool.PooledObject) error {
				c := object.Object.(*beanstalk.Conn)
				return c.Close()
			},
			nil,
			nil,
			nil,
		),
		config,
	)
}
