package glbcheck

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

// makeGLB 由 JSON 对象与 BIN 负载构造合法的 GLB 字节流。
// declaredLen 为 buffers[0].byteLength 的声明值；bin 为 BIN 块原始负载（函数内不做 4 字节补齐，
// 调用方需自行保证块长度对齐，以便构造非法容器用例）。
func makeGLB(t *testing.T, doc map[string]any, bin []byte) []byte {
	t.Helper()
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	for len(js)%4 != 0 {
		js = append(js, 0x20)
	}
	var chunks []byte
	chunks = appendChunk(chunks, chunkTypeJSON, js)
	if bin != nil {
		chunks = appendChunk(chunks, chunkTypeBIN, bin)
	}
	total := glbHeaderLen + len(chunks)
	out := make([]byte, 0, total)
	var hdr [glbHeaderLen]byte
	binary.LittleEndian.PutUint32(hdr[0:4], glbMagic)
	binary.LittleEndian.PutUint32(hdr[4:8], glbVersion2)
	binary.LittleEndian.PutUint32(hdr[8:12], uint32(total))
	out = append(out, hdr[:]...)
	out = append(out, chunks...)
	return out
}

func appendChunk(dst []byte, typ uint32, body []byte) []byte {
	var hdr [chunkHeaderLen]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(body)))
	binary.LittleEndian.PutUint32(hdr[4:8], typ)
	dst = append(dst, hdr[:]...)
	return append(dst, body...)
}

// baseDoc 返回一个含单个内嵌 buffer 的基础 glTF 文档。
func baseDoc(declaredLen int64) map[string]any {
	return map[string]any{
		"asset":   map[string]any{"version": "2.0"},
		"buffers": []any{map[string]any{"byteLength": declaredLen}},
	}
}

// withViews 给文档设置 bufferViews。
func withViews(doc map[string]any, views ...map[string]any) map[string]any {
	arr := make([]any, len(views))
	for i, v := range views {
		arr[i] = v
	}
	doc["bufferViews"] = arr
	return doc
}

// withAccessors 给文档设置 accessors。
func withAccessors(doc map[string]any, accs ...map[string]any) map[string]any {
	arr := make([]any, len(accs))
	for i, a := range accs {
		arr[i] = a
	}
	doc["accessors"] = arr
	return doc
}

// view 构造一个 bufferView 描述；stride 为 nil 表示不声明 byteStride。
func view(offset, length int64, stride *int64) map[string]any {
	m := map[string]any{"buffer": 0, "byteOffset": offset, "byteLength": length}
	if stride != nil {
		m["byteStride"] = *stride
	}
	return m
}

func i64(v int64) *int64 { return &v }

// hasCode 判断报告是否包含指定错误码。
func hasCode(rep *Report, code string) bool {
	for _, iss := range rep.Issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}

// expectOK 断言预检通过。
func expectOK(t *testing.T, rep *Report) {
	t.Helper()
	if !rep.OK() {
		t.Fatalf("期望预检通过，得到问题: %+v", rep.Issues)
	}
}

// expectCode 断言预检失败且包含指定错误码。
func expectCode(t *testing.T, rep *Report, code string) {
	t.Helper()
	if rep.OK() {
		t.Fatalf("期望预检失败(%s)，但没有报告任何问题", code)
	}
	if !hasCode(rep, code) {
		t.Fatalf("期望错误码 %s，得到: %+v", code, rep.Issues)
	}
}
