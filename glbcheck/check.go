package glbcheck

import "os"

// CheckFile 预检本地目录中的 GLB 文件，结果仅保存在内存。
func CheckFile(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return CheckBytes(data), nil
}

// CheckBytes 预检内存中的 GLB 字节流。
func CheckBytes(data []byte) *Report {
	rep := &Report{}
	checkAll(data, rep)
	return rep
}

func checkAll(data []byte, rep *Report) {
	c := parseContainer(data, rep)
	if c == nil {
		return
	}
	g := parseGLTF(c.jsonChunk, rep)
	if g == nil {
		return
	}
	declaredLen, ok := checkBuffers(g, c.binChunk, rep)
	if !ok {
		return
	}
	viewsValid := checkBufferViews(g, declaredLen, rep)
	checkAccessors(g, c.binChunk, viewsValid, rep)
}
