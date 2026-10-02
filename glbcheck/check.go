// Package glbcheck 提供 GLB 资产入库前的整文件预检能力。
// 仅检查容器结构、字节布局与引用合法性，不检查材质、节点或几何质量。
package glbcheck

import (
	"fmt"
	"os"
)

// Severity 表示诊断级别。
type Severity int

const (
	SeverityError Severity = iota // 错误：资产必须被拒绝
)

// Issue 描述一条预检诊断。
type Issue struct {
	Severity Severity
	Code     string // 稳定的机器可读错误码
	Message  string // 人类可读描述
	Path     string // 出错位置，如 "accessors[3].sparse.indices"
}

// Result 是一次预检的内存结果，不持久化。
type Result struct {
	Issues []Issue
}

// OK 报告预检是否通过（无任何错误）。
func (r *Result) OK() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return false
		}
	}
	return true
}

func (r *Result) errorf(path, code, format string, args ...any) {
	r.Issues = append(r.Issues, Issue{
		Severity: SeverityError,
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Path:     path,
	})
}

// Check 对内存中的 GLB 字节执行整文件预检。
func Check(data []byte) *Result {
	res := &Result{}
	c := parseContainer(data, res)
	if c == nil {
		return res
	}
	root, ok := parseDocument(c.json, res)
	if !ok {
		return res
	}
	validateDocument(root, c.bin, res)
	return res
}

// CheckFile 读取本地文件并执行整文件预检。
func CheckFile(path string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("glbcheck: 读取文件失败: %w", err)
	}
	return Check(data), nil
}
