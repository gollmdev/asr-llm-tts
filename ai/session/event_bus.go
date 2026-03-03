package session

import (
	"context"
	"log"
	"sync"

	"github.com/gollmdev/asr-llm-tts/ai/event"
	"golang.org/x/sync/errgroup"
)

// const (
// 	EventLLMChunk EventType = "llm.chunk"
// 	EventLLMDone  EventType = "llm.done"
// 	EventTTSChunk EventType = "tts.chunk"
// 	EventError    EventType = "error"
// )

type RingBuffer struct {
	mu     sync.RWMutex
	size   int
	buffer []event.Event
	index  int64
}

// type Bus struct {
// 	mu     sync.RWMutex
// 	ring   *RingBuffer
// 	subs   map[EventType]map[chan Event]struct{}
// 	closed bool
// }

type Subscriber struct {
	// eventType EventType
	Ch     chan event.Event
	Ctx    context.Context
	cancel context.CancelFunc
	G      *errgroup.Group
	// Closed    chan struct{}
}

type Bus struct {
	mu        sync.RWMutex
	ring      *RingBuffer
	subs      map[event.EventType]map[*Subscriber]struct{} // 多播 / 广播 一个 EventType 可以有多个 Subscriber
	SubSize   chan int                                     // 用于标记没有订阅者的事件类型
	closeOnce sync.Once

	closed bool
}

// type Bus struct {
// 	mu     sync.RWMutex
// 	ring   *RingBuffer
// 	subs   map[EventType]*Subscriber
// 	closed bool
// }

// type Bus struct {
// 	mu     sync.RWMutex
// 	ring   *RingBuffer
// 	subs   map[EventType][]*Subscriber
// 	closed bool
// }

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		size:   size,
		buffer: make([]event.Event, size),
	}
}

func NewBus(bufferSize int) *Bus {
	return &Bus{
		subs:    make(map[event.EventType]map[*Subscriber]struct{}),
		ring:    NewRingBuffer(bufferSize),
		SubSize: make(chan int),
	}
}
func (r *RingBuffer) Replay(fromID int64) []event.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []event.Event

	for i := fromID; i < r.index; i++ {
		if i < r.index-int64(r.size) {
			continue
		}
		result = append(result, r.buffer[i%int64(r.size)])
	}

	return result
}

func (b *Bus) Subscribe(ctx context.Context, types ...event.EventType) (*Subscriber, func()) {
	// ch := make(chan Event, 16)
	ctx, cancel := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	sub := &Subscriber{
		// eventType: eventType,
		Ctx:    ctx,
		cancel: cancel,
		Ch:     make(chan event.Event, 16),
		G:      g,
		// Closed:    make(chan struct{}),
	}

	// if b.closed {
	// 	// close(ch)
	// 	b.mu.Unlock()
	// 	return nil, nil
	// }

	// if _, ok := b.subs[t]; !ok {
	// 	b.subs[t] = make(map[chan Event]struct{})
	// }
	// b.subs[t][ch] = struct{}{}
	b.mu.Lock()

	for _, t := range types {
		if b.subs[t] == nil {
			b.subs[t] = make(map[*Subscriber]struct{})
		}
		b.subs[t][sub] = struct{}{}
	}
	b.mu.Unlock()

	// ctx 结束时自动取消订阅
	// go func() {
	// 	select {
	// 	case <-ctx.Done():
	// 		b.unsubscribe(t, ch)
	// 	}
	// }()
	unsub := func() {
		b.mu.Lock()
		// if subs, ok := b.subs[t]; ok {
		// 	if _, ok := subs[ch]; ok {
		// 		delete(subs, ch)
		// 		close(ch)
		// 	}
		// }
		for _, t := range types {
			if subs, ok := b.subs[t]; ok {
				if _, ok := subs[sub]; ok {
					sub.cancel() // 取消订阅者的 context，触发自动取消逻辑
					// 死锁风险：取消逻辑中需要 sub.G.Wait() 等待 goroutine 结束，cancel本身就在等待 goroutine 结束，导致死锁
					// if err := sub.G.Wait(); err != nil {
					// 	log.Printf("Error waiting for subscriber goroutines to finish: %v", err)
					// }
					delete(subs, sub)
					if len(subs) == 0 {
						delete(b.subs, t)
						// close(b.ZeroSubscribers) // 关闭 zeroSubscribers 通知没有订阅者的事件类型
						// close(sub.Ch)
					}
					// log.Println("len(subs):", len(subs))
				}
			}
		}
		log.Printf(" %s unsubscribe ch!", types)
		if len(b.subs) == 0 {
			// log.Printf("No subscribers left, all events will be ignored.")
			// 关闭 subSize 通知没有订阅者的事件类型
			log.Println("No subscribers left, all events will be ignored.")
			b.closeOnce.Do(func() {
				close(b.SubSize)
			})

		} else {
			// log.Printf("Remaining subscribers: %d", len(b.subs))
			b.SubSize <- len(b.subs) // 发送信号通知没有订阅者的事件类型

		}

		b.mu.Unlock()
	}

	history := b.ring.Replay(0)
	if len(history) != 0 {
		log.Printf("%s Replay history events: %v", types, history)
	}
	go func() {
		for _, e := range history {
			sub.Ch <- e
		}
	}()

	return sub, unsub
}

func (b *Bus) Unsubscribe(t event.EventType, sub *Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subs[t]; ok {
		// if _, exists := subs[sub]; exists {
		// 	delete(subs, sub)
		// }
		sub.cancel() // 取消订阅者的 context，触发自动取消逻辑
		delete(subs, sub)
		if len(subs) == 0 {
			delete(b.subs, t)
			// close(sub.Ch)
		}
	}
}

func (r *RingBuffer) Add(event event.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// event := Event{
	// 	ID:      r.index,
	// 	Content: content,
	// 	Time:    time.Now(),
	// }
	//
	r.buffer[r.index%int64(r.size)] = event
	r.index++

	// return event
}

func (b *Bus) Publish(e event.Event) {
	if e.Type == event.EventLLMChunk {
		b.ring.Add(e)
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	subs := b.subs[e.Type]
	for sub := range subs {
		sub.Ch <- e
		// select {
		// case ch <- e:
		// default:
		// 	log.Printf(">>>>>>>>>> %s: slow consumer, save buffer", e.Type)
		// }
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true

	for _, subs := range b.subs {
		for sub := range subs {
			close(sub.Ch)
		}
	}
	b.subs = nil
}

// func (b *Bus) Subscribe(t EventType) <-chan Event {
// 	ch := make(chan Event, 16)

// 	b.mu.Lock()
// 	b.subs[t] = append(b.subs[t], ch)
// 	b.mu.Unlock()

// 	return ch
// }

// func (b *Bus) Publish(e Event) {
// 	b.mu.RLock()
// 	defer b.mu.RUnlock()

// 	for _, ch := range b.subs[e.Type] {
// 		select {
// 		case ch <- e:
// 		default:
// 			// 慢消费者直接丢（或打点）
// 		}
// 	}
// }
