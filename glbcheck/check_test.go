package glbcheck

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

// buildGLB 由 JSON 对象与 BIN 数据构造合法容器（自动 4 字节对齐填充）。
func buildGLB(t *testing.T, doc any, bin []byte) []byte {
	t.Helper()
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	for len(jsonBytes)%4 != 0 {
		jsonBytes = append(jsonBytes, 0x20)
	}
	var chunks []byte
	chunks = appendChunk(chunks, chunkTypeJSON, jsonBytes)
	if bin != nil {
		padded := append([]byte(nil), bin...)
		for len(padded)%4 != 0 {
			padded = append(padded, 0)
		}
		chunks = appendChunk(chunks, chunkTypeBIN, padded)
	}
	total := headerSize + len(chunks)
	out := make([]byte, 0, total)
	out = binary.LittleEndian.AppendUint32(out, glbMagic)
	out = binary.LittleEndian.AppendUint32(out, glbVersion)
	out = binary.LittleEndian.AppendUint32(out, uint32(total))
	return append(out, chunks...)
}

func appendChunk(dst []byte, typ uint32, body []byte) []byte {
	dst = binary.LittleEndian.AppendUint32(dst, uint32(len(body)))
	dst = binary.LittleEndian.AppendUint32(dst, typ)
	return append(dst, body...)
}

// mapDoc 便捷构造 JSON 文档。
func mapDoc(kv ...any) map[string]any {
	doc := map[string]any{"asset": map[string]any{"version": "2.0"}}
	for i := 0; i+1 < len(kv); i += 2 {
		doc[kv[i].(string)] = kv[i+1]
	}
	return doc
}

// checkCodes 返回结果中的全部错误码。
func checkCodes(res *Result) []string {
	var codes []string
	for _, issue := range res.Issues {
		codes = append(codes, issue.Code)
	}
	return codes
}

// requireCode 断言结果包含指定错误码。
func requireCode(t *testing.T, res *Result, code string) {
	t.Helper()
	for _, issue := range res.Issues {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("期望错误码 %s，实际 %v (issues=%v)", code, checkCodes(res), res.Issues)
}

// requireOK 断言预检通过。
func requireOK(t *testing.T, res *Result) {
	t.Helper()
	if !res.OK() {
		t.Fatalf("期望通过，实际 issues=%v", res.Issues)
	}
}

// simpleDoc 构造含单个 buffer/bufferView/accessor 的文档。
func simpleDoc(view map[string]any, accessor map[string]any) map[string]any {
	return mapDoc(
		"buffers", []any{map[string]any{"byteLength": 64}},
		"bufferViews", []any{view},
		"accessors", []any{accessor},
	)
}

func baseView() map[string]any {
	return map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 64}
}

func TestValidMinimal(t *testing.T) {
	data := buildGLB(t, mapDoc(), nil)
	requireOK(t, Check(data))
}

func TestValidInterleavedStride(t *testing.T) {
	// 交错布局：VEC3 float32，byteStride=16（含 4 字节间隔），count=3 → 2*16+12=44
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 44, "byteStride": 16}
	acc := map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"}
	doc := mapDoc(
		"buffers", []any{map[string]any{"byteLength": 44}},
		"bufferViews", []any{view},
		"accessors", []any{acc},
	)
	requireOK(t, Check(buildGLB(t, doc, make([]byte, 44))))
}

func TestStrideTooSmall(t *testing.T) {
	// VEC3 float32 完整布局步长 12，显式 byteStride=8 必须拒绝
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 64, "byteStride": 8}
	acc := map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"}
	res := Check(buildGLB(t, simpleDoc(view, acc), make([]byte, 64)))
	requireCode(t, res, "STRIDE_TOO_SMALL")
}

func TestStrideNotMultipleOf4(t *testing.T) {
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 64, "byteStride": 14}
	acc := map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"}
	res := Check(buildGLB(t, simpleDoc(view, acc), make([]byte, 64)))
	requireCode(t, res, "VIEW_STRIDE")
}

func TestStrideOutOfRange(t *testing.T) {
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 512, "byteStride": 256}
	acc := map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"}
	res := Check(buildGLB(t, simpleDoc(view, acc), make([]byte, 512)))
	requireCode(t, res, "VIEW_STRIDE")
}

func TestMatrixColumnPaddingExactFit(t *testing.T) {
	// MAT3 uint8：列 3 字节，列步长对齐到 4，完整步长 12，末元素实际占用 11。
	// count=2 → 12+11=23，视图恰好 23 字节应通过。
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 23}
	acc := map[string]any{"bufferView": 0, "componentType": 5121, "count": 2, "type": "MAT3"}
	doc := mapDoc(
		"buffers", []any{map[string]any{"byteLength": 23}},
		"bufferViews", []any{view},
		"accessors", []any{acc},
	)
	requireOK(t, Check(buildGLB(t, doc, make([]byte, 23))))
}

func TestMatrixColumnPaddingOneByteShort(t *testing.T) {
	// 同上布局，视图少 1 字节（22）必须拒绝。
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 22}
	acc := map[string]any{"bufferView": 0, "componentType": 5121, "count": 2, "type": "MAT3"}
	doc := mapDoc(
		"buffers", []any{map[string]any{"byteLength": 22}},
		"bufferViews", []any{view},
		"accessors", []any{acc},
	)
	res := Check(buildGLB(t, doc, make([]byte, 22)))
	requireCode(t, res, "DATA_RANGE")
}

func TestMatrixAbsoluteAlignment(t *testing.T) {
	// MAT2 uint16：绝对起点 2 未按 4 字节对齐，必须拒绝。
	view := map[string]any{"buffer": 0, "byteOffset": 2, "byteLength": 16}
	acc := map[string]any{"bufferView": 0, "componentType": 5123, "count": 2, "type": "MAT2"}
	doc := mapDoc(
		"buffers", []any{map[string]any{"byteLength": 32}},
		"bufferViews", []any{view},
		"accessors", []any{acc},
	)
	res := Check(buildGLB(t, doc, make([]byte, 32)))
	requireCode(t, res, "OFFSET_ALIGNMENT")
}

func TestExactEndBoundary(t *testing.T) {
	// VEC4 uint16 紧密布局：8 字节/元素，count=8 恰好占满 64 字节视图。
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 64}
	acc := map[string]any{"bufferView": 0, "componentType": 5123, "count": 8, "type": "VEC4"}
	doc := mapDoc(
		"buffers", []any{map[string]any{"byteLength": 64}},
		"bufferViews", []any{view},
		"accessors", []any{acc},
	)
	requireOK(t, Check(buildGLB(t, doc, make([]byte, 64))))
}

func TestOneByteOverflow(t *testing.T) {
	// count=8 需要 64 字节，视图只有 63 字节（少 1 字节）必须拒绝。
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 63}
	acc := map[string]any{"bufferView": 0, "componentType": 5123, "count": 8, "type": "VEC4"}
	doc := mapDoc(
		"buffers", []any{map[string]any{"byteLength": 63}},
		"bufferViews", []any{view},
		"accessors", []any{acc},
	)
	res := Check(buildGLB(t, doc, make([]byte, 63)))
	requireCode(t, res, "DATA_RANGE")
}

func TestAccessorOffsetAlignment(t *testing.T) {
	// uint32 分量，相对偏移 2 未按 4 对齐。
	view := map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 64}
	acc := map[string]any{"bufferView": 0, "byteOffset": 2, "componentType": 5125, "count": 4, "type": "SCALAR"}
	res := Check(buildGLB(t, simpleDoc(view, acc), make([]byte, 64)))
	requireCode(t, res, "OFFSET_ALIGNMENT")
}

func TestAccessorWithoutBufferView(t *testing.T) {
	// 无 bufferView 且 byteOffset 缺省：合法（全零底值）。
	acc := map[string]any{"componentType": 5126, "count": 4, "type": "VEC3"}
	doc := mapDoc("accessors", []any{acc})
	requireOK(t, Check(buildGLB(t, doc, nil)))
}

func TestAccessorWithoutBufferViewNonzeroOffset(t *testing.T) {
	acc := map[string]any{"byteOffset": 4, "componentType": 5126, "count": 4, "type": "VEC3"}
	doc := mapDoc("accessors", []any{acc})
	res := Check(buildGLB(t, doc, nil))
	requireCode(t, res, "OFFSET_ZERO_BASE")
}

func TestAccessorInvalidCount(t *testing.T) {
	acc := map[string]any{"componentType": 5126, "count": 0, "type": "VEC3"}
	doc := mapDoc("accessors", []any{acc})
	res := Check(buildGLB(t, doc, nil))
	requireCode(t, res, "ACCESSOR_COUNT")
}

func TestUnsupportedComponentType(t *testing.T) {
	acc := map[string]any{"componentType": 5124, "count": 1, "type": "SCALAR"}
	doc := mapDoc("accessors", []any{acc})
	res := Check(buildGLB(t, doc, nil))
	requireCode(t, res, "COMPONENT_TYPE")
}
