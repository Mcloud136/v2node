package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/conf"
	"github.com/wyx2685/v2node/core"
)

type Node struct {
	controllers []*Controller
	NodeInfos   []*panel.NodeInfo
}

type nodeInitResult struct {
	index      int
	controller *Controller
	nodeInfo   *panel.NodeInfo
	err        error
}

func New(nodes []conf.NodeConfig) (*Node, error) {
	n := &Node{
		controllers: make([]*Controller, len(nodes)),
		NodeInfos:   make([]*panel.NodeInfo, len(nodes)),
	}

	if len(nodes) == 0 {
		return n, nil
	}

	resultCh := make(chan nodeInitResult, len(nodes))
	var wg sync.WaitGroup

	for i, nodeConfig := range nodes {
		wg.Add(1)
		go func(index int, config conf.NodeConfig) {
			defer wg.Done()
			p, err := panel.New(&config)
			if err != nil {
				resultCh <- nodeInitResult{index: index, err: err}
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			info, err := p.GetNodeInfo(ctx)
			if err != nil {
				resultCh <- nodeInitResult{index: index, err: err}
				return
			}
			controller := NewController(p, &config, info)
			resultCh <- nodeInitResult{index: index, controller: controller, nodeInfo: info}
		}(i, nodeConfig)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		if result.err != nil {
			return nil, result.err
		}
		n.controllers[result.index] = result.controller
		n.NodeInfos[result.index] = result.nodeInfo
	}

	return n, nil
}

func (n *Node) Start(nodes []conf.NodeConfig, core *core.V2Core) error {
	// 并行启动所有 controller，减少多节点启动时间
	type startResult struct {
		index int
		err   error
	}
	results := make(chan startResult, len(nodes))
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := n.controllers[idx].Start(core)
			results <- startResult{index: idx, err: err}
		}(i)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	for r := range results {
		if r.err != nil {
			return fmt.Errorf("start node controller [%s-%d] error: %w",
				nodes[r.index].APIHost,
				nodes[r.index].NodeID,
				r.err)
		}
	}
	return nil
}

func (n *Node) Close() error {
	// 关闭所有 controller，即使某个失败也继续关闭其余的
	var errs []error
	for i, c := range n.controllers {
		if err := c.Close(); err != nil {
			log.Errorf("close controller [%d] failed: %v", i, err)
			errs = append(errs, err)
		}
	}
	n.controllers = nil
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
