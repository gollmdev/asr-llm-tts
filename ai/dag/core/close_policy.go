package dag

type EventExpr interface {
	Match(received map[string]struct{}) bool
}

type eventTypeExpr struct {
	eventType string
}

func (e eventTypeExpr) Match(received map[string]struct{}) bool {
	_, ok := received[e.eventType]
	return ok
}

type allExpr struct {
	children []EventExpr
}

func (e allExpr) Match(received map[string]struct{}) bool {
	if len(e.children) == 0 {
		return true
	}
	for _, child := range e.children {
		if child == nil || !child.Match(received) {
			return false
		}
	}
	return true
}

type anyExpr struct {
	children []EventExpr
}

func (e anyExpr) Match(received map[string]struct{}) bool {
	if len(e.children) == 0 {
		return false
	}
	for _, child := range e.children {
		if child != nil && child.Match(received) {
			return true
		}
	}
	return false
}

func HasEvent(eventType string) EventExpr {
	return eventTypeExpr{eventType: eventType}
}

func All(exprs ...EventExpr) EventExpr {
	return allExpr{children: exprs}
}

func Any(exprs ...EventExpr) EventExpr {
	return anyExpr{children: exprs}
}

type NodeCloseContext struct {
	NodeID          string
	ReceivedEvents  map[string]struct{}
	ActiveUpstreams int
}

type NodeStartContext struct {
	NodeID         string
	ReceivedEvents map[string]struct{}
}

type NodeStartPolicy interface {
	CanStart(ctx NodeStartContext) bool
}

type NodeStartPolicyProvider interface {
	StartPolicy() NodeStartPolicy
}

type AggregateStartPolicy struct {
	Required EventExpr
}

func (p AggregateStartPolicy) CanStart(ctx NodeStartContext) bool {
	if p.Required == nil {
		return true
	}
	return p.Required.Match(ctx.ReceivedEvents)
}

type NodeClosePolicy interface {
	CanClose(ctx NodeCloseContext) bool
}

type NodeClosePolicyProvider interface {
	ClosePolicy() NodeClosePolicy
}

type AggregateClosePolicy struct {
	Required EventExpr
}

func (p AggregateClosePolicy) CanClose(ctx NodeCloseContext) bool {
	if ctx.ActiveUpstreams > 0 {
		return false
	}
	if p.Required == nil {
		return true
	}
	return p.Required.Match(ctx.ReceivedEvents)
}
