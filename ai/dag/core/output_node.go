package dag

import (
	"errors"
	"log"
)

type OutputNode struct {
	id     string
	output chan<- *Event // 注意：写入外部 channel
}

func NewOutputNode(id string, out chan<- *Event) *OutputNode {
	return &OutputNode{
		id:     id,
		output: out,
	}
}
func (n *OutputNode) Mode() NodeMode {
	return ModeLazy
}
func (n *OutputNode) ID() string { return n.id }

func (n *OutputNode) Run(
	// ctx context.Context,
	rt NodeRuntime,
	// in <-chan Event,
	// out chan<- Event,
) error {
	ctx := rt.Context()
	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				log.Println("output close!")
				close(n.output)
				return nil
			}
			// 直接推给 Session
			n.output <- ev

		case <-ctx.Done():
			close(n.output)
			// return nil
			return errors.New("output node context canceled!")
			// return nil
		}
	}
}
