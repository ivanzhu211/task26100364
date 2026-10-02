package glbcheck

import "testing"

// 交错步长：两个 VEC3 float32 访问器共用一个 stride=24 的视图。
func TestInterleavedStride(t *testing.T) {
	doc := baseDoc(48)
	withViews(doc, view(0, 48, i64(24)))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5126, "count": 2, "type": "VEC3"},
		map[string]any{"bufferView": 0, "byteOffset": 12, "componentType": 5126, "count": 2, "type": "VEC3"},
	)
	expectOK(t, CheckBytes(makeGLB(t, doc, make([]byte, 48))))
}

// 显式 stride 小于紧密布局步长必须拒绝（VEC3 float32 紧密步长为 12）。
func TestStrideTooSmall(t *testing.T) {
	doc := baseDoc(48)
	withViews(doc, view(0, 48, i64(8)))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5126, "count": 2, "type": "VEC3"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 48))), "accessor.strideTooSmall")
}

// 非法 stride 值：非 4 倍数、小于 4、大于 252。
func TestInvalidStrideValues(t *testing.T) {
	for _, stride := range []int64{2, 6, 254, 256} {
		doc := baseDoc(64)
		withViews(doc, view(0, 64, i64(stride)))
		withAccessors(doc,
			map[string]any{"bufferView": 0, "componentType": 5126, "count": 2, "type": "VEC3"},
		)
		expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 64))), "bufferView.badStride")
	}
}

// MAT3 float32：列步长 12 已对齐，紧密元素 36 字节，恰好结束应通过。
func TestMat3Float32ExactFit(t *testing.T) {
	doc := baseDoc(72)
	withViews(doc, view(0, 72, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5126, "count": 2, "type": "MAT3"},
	)
	expectOK(t, CheckBytes(makeGLB(t, doc, make([]byte, 72))))
}

// MAT3 int8：列 3 字节填充到 4，紧密步长 12；最后元素末列填充可省略，
// 2 个元素实际占用 12+11=23 字节。恰好 23 通过，22 拒绝。
func TestMat3Int8ColumnPadding(t *testing.T) {
	doc := baseDoc(24)
	withViews(doc, view(0, 23, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5120, "count": 2, "type": "MAT3"},
	)
	expectOK(t, CheckBytes(makeGLB(t, doc, make([]byte, 24))))

	doc = baseDoc(24)
	withViews(doc, view(0, 22, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5120, "count": 2, "type": "MAT3"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 24))), "accessor.outOfView")
}

// 标量恰好结束与少 1 字节。
func TestExactEndAndOneByteShort(t *testing.T) {
	// 3 个 uint16 SCALAR = 6 字节，恰好结束。
	doc := baseDoc(8)
	withViews(doc, view(0, 6, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5123, "count": 3, "type": "SCALAR"},
	)
	expectOK(t, CheckBytes(makeGLB(t, doc, make([]byte, 8))))

	// 视图只有 5 字节，少 1 字节必须拒绝。
	doc = baseDoc(8)
	withViews(doc, view(0, 5, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5123, "count": 3, "type": "SCALAR"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 8))), "accessor.outOfView")
}

// 带 stride 时末尾元素按实际占用计算：stride=8 的 uint16 SCALAR，
// 3 个元素占用 8+8+2=18 字节，恰好 18 通过，17 拒绝。
func TestStrideExactEndAndOneByteShort(t *testing.T) {
	doc := baseDoc(20)
	withViews(doc, view(0, 18, i64(8)))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5123, "count": 3, "type": "SCALAR"},
	)
	expectOK(t, CheckBytes(makeGLB(t, doc, make([]byte, 20))))

	doc = baseDoc(20)
	withViews(doc, view(0, 17, i64(8)))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5123, "count": 3, "type": "SCALAR"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 20))), "accessor.outOfView")
}

// 相对视图偏移未按分量大小对齐。
func TestRelativeOffsetMisaligned(t *testing.T) {
	doc := baseDoc(16)
	withViews(doc, view(0, 16, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "byteOffset": 1, "componentType": 5123, "count": 2, "type": "SCALAR"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 16))), "accessor.offsetMisaligned")
}

// 绝对偏移未按分量大小对齐：视图偏移 1 + 访问器偏移 1 = 2，对 uint16 合法；
// 视图偏移 1 + 访问器偏移 2 = 3，非法。相对偏移 2 本身合法，只应报绝对对齐。
func TestAbsoluteOffsetMisaligned(t *testing.T) {
	doc := baseDoc(16)
	withViews(doc, view(1, 15, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "byteOffset": 2, "componentType": 5123, "count": 2, "type": "SCALAR"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 16))), "accessor.absOffsetMisaligned")
}

// MAT3 int8 元素绝对起点必须 4 字节对齐（列起点对齐）。
func TestMatrixColumnAlignment(t *testing.T) {
	doc := baseDoc(32)
	withViews(doc, view(2, 23, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5120, "count": 2, "type": "MAT3"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 32))), "accessor.columnMisaligned")

	// 视图偏移 4：元素起点 4，对齐，通过。
	doc = baseDoc(32)
	withViews(doc, view(4, 23, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5120, "count": 2, "type": "MAT3"},
	)
	expectOK(t, CheckBytes(makeGLB(t, doc, make([]byte, 32))))
}

// 无 bufferView 的访问器：byteOffset 缺省/为 0 合法，非 0 拒绝。
func TestAccessorWithoutBufferView(t *testing.T) {
	doc := baseDoc(4)
	withAccessors(doc,
		map[string]any{"componentType": 5126, "count": 4, "type": "SCALAR"},
	)
	expectOK(t, CheckBytes(makeGLB(t, doc, make([]byte, 4))))

	doc = baseDoc(4)
	withAccessors(doc,
		map[string]any{"componentType": 5126, "count": 4, "type": "SCALAR", "byteOffset": 4},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 4))), "accessor.zeroBaseOffset")
}

// count 必须为正整数。
func TestAccessorCountPositive(t *testing.T) {
	doc := baseDoc(4)
	withViews(doc, view(0, 4, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5126, "count": 0, "type": "SCALAR"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 4))), "accessor.badCount")
}

// 非法 componentType 与 type。
func TestBadComponentTypeAndShape(t *testing.T) {
	doc := baseDoc(4)
	withViews(doc, view(0, 4, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5124, "count": 1, "type": "SCALAR"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 4))), "accessor.badComponentType")

	doc = baseDoc(4)
	withViews(doc, view(0, 4, nil))
	withAccessors(doc,
		map[string]any{"bufferView": 0, "componentType": 5126, "count": 1, "type": "VEC5"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 4))), "accessor.badType")
}

// bufferView 越界：以声明的 buffer 长度为边界。
func TestBufferViewOutOfRange(t *testing.T) {
	doc := baseDoc(8)
	withViews(doc, view(4, 8, nil)) // 4+8=12 > 8
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 8))), "bufferView.outOfRange")
}

// 访问器引用不存在的 bufferView。
func TestAccessorBadBufferViewRef(t *testing.T) {
	doc := baseDoc(4)
	withAccessors(doc,
		map[string]any{"bufferView": 7, "componentType": 5126, "count": 1, "type": "SCALAR"},
	)
	expectCode(t, CheckBytes(makeGLB(t, doc, make([]byte, 4))), "accessor.badBufferView")
}
