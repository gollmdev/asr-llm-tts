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