package tql

type NodeContext struct {
	// Key  any
	node *Node
}

// tql function: context()
func (node *Node) GetContext() *NodeContext {
	return &NodeContext{
		node: node,
	}
}

// tql function: key()
func (node *Node) GetRecordKey() any {
	if inflight := node.Inflight(); inflight != nil {
		return inflight.key
	}
	return nil
}

// tql function: value()
func (node *Node) GetRecordValue() any {
	if inflight := node.Inflight(); inflight != nil {
		return inflight.value
	}
	return nil
}

// tql function: payload()
func (node *Node) GetRequestPayload() any {
	return node.task.inputReader
}

// tql function: param()
func (node *Node) GetRequestParam(name string) any {
	vals := node.task.params[name]
	if len(vals) == 1 {
		return vals[0]
	} else if len(vals) > 0 {
		return vals
	}
	return nil
}
