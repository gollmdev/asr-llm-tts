package node

import (
	"errors"
	"log"

	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
)

type OutputNode struct {
	// id     string
	// output chan<- *Event // 注意：写入外部 channel
}

//	func NewOutputNode(id string, out chan<- *Event) *OutputNode {
//		return &OutputNode{
//			id:     id,
//			output: out,
//		}
//	}
//
//	func (n *OutputNode) ClosePolicy() dag.NodeClosePolicy {
//		return dag.AggregateClosePolicy{
//			Required: dag.All(
//				dag.Any(dag.HasEvent("llm_chunk")),
//			),
//		}
//	}
func (n *OutputNode) Mode() dag.NodeMode {
	return dag.ModeLazy
}
func (n *OutputNode) ID() string { return "output" }

func (n *OutputNode) Run(
	// ctx context.Context,
	rt dag.NodeRuntime,
	// in <-chan dag.Event,
	// out chan<- dag.Event,
) error {
	ctx := rt.Context()
	for {
		select {
		case ev, ok := <-rt.Input():
			if !ok {
				log.Println("output close!")
				// close(n.output)
				return nil
			}
			// 直接推给 Session
			// rt.Output().Emit(ev)
			rt.RuntimeContext().Output.Emit(ev)

		case <-ctx.Done():
			// close(n.output)
			// return nil
			return errors.New("output node context canceled!")
			// return nil
		}
	}
}
