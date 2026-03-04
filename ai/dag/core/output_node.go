package dag

import (
	"context"
	"log"
)

type OutputNode struct {
	id     string
	output chan<- Event // 注意：写入外部 channel
}

func NewOutputNode(id string, out chan<- Event) *OutputNode {
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
	ctx context.Context,
	in <-chan Event,
	out chan<- Event,
) error {
	for {
		select {
		case ev, ok := <-in:
			if !ok {
				log.Println("output close!")
				close(n.output)
				return nil
			}
			// 直接推给 Session
			n.output <- ev

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
