// Package glbcheck 提供 GLB 资产入库前的整文件预检能力。
// 仅检查容器结构、字节布局与引用合法性，不检查材质、节点或几何质量。
package glbcheck

// Severity 表示预检问题的严重程度。
type Severity int

const (
	// SeverityError 表示必须拒绝入库的错误。
	SeverityError Severity = iota
)

// Issue 描述一条预检发现的问题。
type Issue struct {
	Severity Severity
	Code     string // 稳定的机器可读错误码，如 "container.badMagic"
	Path     string // 出错位置，如 "accessors[3].sparse.indices"
	Message  string // 人类可读描述
}

// Report 是一次预检的结果，仅保存在内存。
type Report struct {
	Issues []Issue
}

// OK 报告是否没有任何错误。
func (r *Report) OK() bool { return len(r.Issues) == 0 }

func (r *Report) errorf(code, path, format string, args ...any) {
	r.Issues = append(r.Issues, Issue{
		Severity: SeverityError,
		Code:     code,
		Path:     path,
		Message:  sprintf(format, args...),
	})
}
