// func (e *Engine) Start(ctx context.Context) error {
// 	g, ctx := errgroup.WithContext(ctx)

// 	// 启动所有节点
// 	for id, node := range e.dag.Nodes {
// 		id := id
// 		node := node

// 		g.Go(func() error {
// 			return node.Run(ctx, e.nodeInput[id], e.bus)
// 		})
// 	}

// 	// 启动 dispatcher
// 	g.Go(func() error {
// 		for {
// 			select {
// 			case ev := <-e.bus:
// 				e.dispatch(ev)
// 			case <-ctx.Done():
// 				return ctx.Err()
// 			}
// 		}
// 	})

// 	return g.Wait()
// }

// func (e *Engine) dispatch(ev Event) {
// 	for _, edge := range e.dag.Edges {
// 		// if ev.Type == "final_output" {
// 		// 	e.Output <- ev
// 		// 	return
// 		// }
// 		if edge.FromNode == ev.From && edge.OnEvent == ev.Type {
// 			e.nodeInput[edge.ToNode] <- ev
// 		}
// 	}
// }


```
Node
  ↓
Emit
  ↓
middleware chain
  ↓
bus
  ↓
dispatch
  ↓
nodeInput
  ↓
Node
```

你的 Engine 其实已经接近：

Actor Runtime

未来可以演化成：

LLM Workflow Runtime

支持：

DAG

streaming

memory

tool

checkpoint

这一套是 LangGraph / Temporal 类系统的核心。



在 workflow / DAG runtime 里通常有三层：
```
Process Context   (程序级)
        ↓
Session Context   (会话级)
        ↓
Engine Context    (一次执行)
        ↓
Node Context      (节点运行)
```
```
ctx
 └─ RuntimeContext
      └─ Engine
           └─ nodeRuntime
```