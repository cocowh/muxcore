package router

import (
    "github.com/cocowh/muxcore/pkg/logger"
)

// RadixNode 前缀树节点
type RadixNode struct {
    prefix     string
    children   map[string]*RadixNode
    isLeaf     bool
    nodeIDs    []string
}

// RadixTree 实现前缀树路由
type RadixTree struct {
    root *RadixNode
}

// NewRadixTree 创建前缀树
func NewRadixTree() *RadixTree {
    return &RadixTree{
        root: &RadixNode{
            children: make(map[string]*RadixNode),
        },
    }
}

// Insert 插入路由规则
func (t *RadixTree) Insert(path string, nodeIDs []string) {
    if path == "" {
        return
    }

    current := t.root
    for {
        // 查找最长公共前缀
        var commonPrefixLen int
        minLen := len(current.prefix)
        if len(path) < minLen {
            minLen = len(path)
        }

        for i := 0; i < minLen; i++ {
            if current.prefix[i] != path[i] {
                break
            }
            commonPrefixLen = i + 1
        }

        // 如果没有公共前缀
        if commonPrefixLen == 0 {
            // 创建新节点
            child := &RadixNode{
                prefix:   path,
                children: make(map[string]*RadixNode),
                isLeaf:   true,
                nodeIDs:  nodeIDs,
            }
            current.children[string(path[0])] = child
            logger.Debugf("Inserted new radix node with prefix: %s", path)
            return
        }

        // 如果当前节点的前缀是path的前缀
        if commonPrefixLen == len(current.prefix) {
            // 继续处理剩余路径
            path = path[commonPrefixLen:]
            if path == "" {
                // 更新当前节点
                current.isLeaf = true
                current.nodeIDs = nodeIDs
                logger.Debugf("Updated radix node with prefix: %s", current.prefix)
                return
            }

            // 查找下一个子节点
            nextChar := string(path[0])
            if child, exists := current.children[nextChar]; exists {
                current = child
            } else {
                // 创建新节点
                child := &RadixNode{
                    prefix:   path,
                    children: make(map[string]*RadixNode),
                    isLeaf:   true,
                    nodeIDs:  nodeIDs,
                }
                current.children[nextChar] = child
                logger.Debugf("Inserted new radix node with prefix: %s", path)
                return
            }
        } else {
            // 需要分裂当前节点
            splitNode := &RadixNode{
                prefix:   current.prefix[:commonPrefixLen],
                children: make(map[string]*RadixNode),
                isLeaf:   false,
            }

            // 调整原节点
            current.prefix = current.prefix[commonPrefixLen:]
            splitNode.children[string(current.prefix[0])] = current

            // 创建新节点
            newNode := &RadixNode{
                prefix:   path[commonPrefixLen:],
                children: make(map[string]*RadixNode),
                isLeaf:   true,
                nodeIDs:  nodeIDs,
            }
            splitNode.children[string(newNode.prefix[0])] = newNode

            // 更新父节点
            parentChar := string(splitNode.prefix[0])
            if parent, exists := t.findParent(t.root, current); exists {
                parent.children[parentChar] = splitNode
            } else {
                // 如果当前节点是根节点的直接子节点
                t.root.children[parentChar] = splitNode
            }

            logger.Debugf("Split radix node and inserted new node with prefix: %s", path)
            return
        }
    }
}

// findParent 查找父节点（辅助函数）
func (t *RadixTree) findParent(root *RadixNode, node *RadixNode) (*RadixNode, bool) {
    if root == nil {
        return nil, false
    }

    for _, child := range root.children {
        if child == node {
            return root, true
        }

        if parent, found := t.findParent(child, node); found {
            return parent, true
        }
    }

    return nil, false
}

// Search 查找匹配的路由
func (t *RadixTree) Search(path string) []string {
    if path == "" {
        return nil
    }

    current := t.root
    var bestMatch []string

    for {
        // 查找最长公共前缀
        var commonPrefixLen int
        minLen := len(current.prefix)
        if len(path) < minLen {
            minLen = len(path)
        }

        for i := 0; i < minLen; i++ {
            if current.prefix[i] != path[i] {
                break
            }
            commonPrefixLen = i + 1
        }

        // 如果没有公共前缀
        if commonPrefixLen == 0 {
            return bestMatch
        }

        // 如果当前节点的前缀是path的前缀
        if commonPrefixLen == len(current.prefix) {
            // 记录匹配
            if current.isLeaf {
                bestMatch = current.nodeIDs
            }

            // 继续处理剩余路径
            path = path[commonPrefixLen:]
            if path == "" {
                return bestMatch
            }

            // 查找下一个子节点
            nextChar := string(path[0])
            if child, exists := current.children[nextChar]; exists {
                current = child
            } else {
                return bestMatch
            }
        } else {
            // 部分匹配
            if commonPrefixLen == len(path) && current.prefix[:commonPrefixLen] == path {
                return bestMatch
            }
            return nil
        }
    }
}

// Delete 删除路由规则
func (t *RadixTree) Delete(path string) bool {
    // 实现删除逻辑...
    return false
}

// GetAllPaths 获取所有路径（用于调试）
func (t *RadixTree) GetAllPaths() []string {
    var paths []string
    t.collectPaths(t.root, "", &paths)
    return paths
}

// collectPaths 收集所有路径（辅助函数）
func (t *RadixTree) collectPaths(node *RadixNode, currentPath string, paths *[]string) {
    if node == nil {
        return
    }

    newPath := currentPath + node.prefix
    if node.isLeaf {
        *paths = append(*paths, newPath)
    }

    for _, child := range node.children {
        t.collectPaths(child, newPath, paths)
    }
}