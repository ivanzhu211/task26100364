package glbcheck

import (
	"encoding/binary"
	"fmt"
)

// 本文件负责语义校验：buffer、bufferView、accessor 及 sparse 引用。

// validator 持有校验过程的上下文。
type validator struct {
	res  *Result
	root *gltfRoot
	bin  []byte // 内嵌 BIN 块内容，可为 nil
}

// validateDocument 校验 JSON 文档与 BIN 数据的一致性。
func validateDocument(root *gltfRoot, bin []byte, res *Result) {
	v := &validator{res: res, root: root, bin: bin}
	v.checkBuffers()
	v.checkBufferViews()
	v.checkAccessors()
}

// declaredBufferLength 返回声明的 buffer 长度；无 buffer 时返回 0。
func (v *validator) declaredBufferLength() int64 {
	if len(v.root.Buffers) == 0 {
		return 0
	}
	return v.root.Buffers[0].ByteLength
}

func (v *validator) checkBuffers() {
	buffers := v.root.Buffers
	if len(buffers) > 1 {
		v.res.errorf("buffers", "BUFFER_COUNT", "最多允许一个内嵌 buffer，实际 %d 个", len(buffers))
	}
	for i, buf := range buffers {
		path := fmt.Sprintf("buffers[%d]", i)
		if buf.URI != "" {
			v.res.errorf(path+".uri", "URI_UNSUPPORTED", "不支持外部 URI buffer: %s", buf.URI)
		}
		if buf.ByteLength <= 0 {
			v.res.errorf(path+".byteLength", "BUFFER_LENGTH", "byteLength 必须为正整数，实际 %d", buf.ByteLength)
		}
	}
	if len(buffers) == 0 {
		if v.bin != nil {
			v.res.errorf("chunks", "BIN_ORPHAN", "存在 BIN 块但未声明 buffer")
		}
		return
	}
	declared := buffers[0].ByteLength
	if v.bin == nil {
		v.res.errorf("buffers[0]", "BIN_MISSING", "声明了 buffer 但缺少 BIN 块")
		return
	}
	if declared <= 0 {
		return
	}
	diff := int64(len(v.bin)) - declared
	switch {
	case diff < 0:
		v.res.errorf("buffers[0].byteLength", "BIN_LENGTH",
			"BIN 块长度 %d 小于 buffer.byteLength %d", len(v.bin), declared)
	case diff > 3:
		v.res.errorf("buffers[0].byteLength", "BIN_PADDING",
			"BIN 块长度 %d 超出 buffer.byteLength %d 达 %d 字节，仅允许 0 至 3 字节零填充",
			len(v.bin), declared, diff)
	default:
		for i := declared; i < int64(len(v.bin)); i++ {
			if v.bin[i] != 0 {
				v.res.errorf("buffers[0].byteLength", "BIN_PADDING",
					"BIN 块尾部填充字节（偏移 %d）非零: 0x%02X", i, v.bin[i])
				break
			}
		}
	}
}

func (v *validator) checkBufferViews() {
	declared := v.declaredBufferLength()
	for i, view := range v.root.BufferViews {
		path := fmt.Sprintf("bufferViews[%d]", i)
		if view.Buffer < 0 || view.Buffer >= len(v.root.Buffers) {
			v.res.errorf(path+".buffer", "VIEW_BUFFER", "buffer 索引 %d 越界（共 %d 个 buffer）",
				view.Buffer, len(v.root.Buffers))
			continue
		}
		if view.ByteOffset < 0 {
			v.res.errorf(path+".byteOffset", "VIEW_OFFSET", "byteOffset 不能为负: %d", view.ByteOffset)
		}
		if view.ByteLength <= 0 {
			v.res.errorf(path+".byteLength", "VIEW_LENGTH", "byteLength 必须为正整数，实际 %d", view.ByteLength)
		}
		if view.ByteOffset >= 0 && view.ByteLength > 0 && view.ByteOffset+view.ByteLength > declared {
			v.res.errorf(path, "VIEW_RANGE",
				"视图范围 [%d, %d) 超出声明的 buffer 长度 %d",
				view.ByteOffset, view.ByteOffset+view.ByteLength, declared)
		}
		if view.ByteStride != 0 {
			if view.ByteStride < 4 || view.ByteStride > 252 || view.ByteStride%4 != 0 {
				v.res.errorf(path+".byteStride", "VIEW_STRIDE",
					"byteStride 须为 4 至 252 之间的 4 倍数，实际 %d", view.ByteStride)
			}
		}
	}
}

// viewByIndex 返回合法索引的 bufferView，非法时记录错误并返回 nil。
func (v *validator) viewByIndex(index int, path string) *gltfBufferView {
	if index < 0 || index >= len(v.root.BufferViews) {
		v.res.errorf(path, "VIEW_INDEX", "bufferView 索引 %d 越界（共 %d 个 bufferView）",
			index, len(v.root.BufferViews))
		return nil
	}
	return &v.root.BufferViews[index]
}

// checkDataRange 校验一段数据在视图内的偏移、对齐与占用范围。
// absBase 为视图相对 buffer 的偏移，rel 为数据相对视图的偏移。
func (v *validator) checkDataRange(view *gltfBufferView, rel int64, li layoutInfo, count int64, stride int, path string) {
	if rel < 0 {
		v.res.errorf(path+".byteOffset", "OFFSET_NEGATIVE", "byteOffset 不能为负: %d", rel)
		return
	}
	if rel%int64(li.comp.size) != 0 {
		v.res.errorf(path+".byteOffset", "OFFSET_ALIGNMENT",
			"相对视图偏移 %d 未按分量大小 %d 对齐", rel, li.comp.size)
	}
	abs := view.ByteOffset + rel
	if abs%int64(li.comp.size) != 0 {
		v.res.errorf(path+".byteOffset", "OFFSET_ALIGNMENT",
			"绝对偏移 %d 未按分量大小 %d 对齐", abs, li.comp.size)
	}
	if li.shape.isMatrix && abs%4 != 0 {
		v.res.errorf(path+".byteOffset", "OFFSET_ALIGNMENT",
			"矩阵列绝对起点 %d 未按 4 字节对齐", abs)
	}
	occupied := li.occupiedBytes(count, stride)
	if rel+occupied > view.ByteLength {
		v.res.errorf(path, "DATA_RANGE",
			"数据占用 [%d, %d) 超出视图长度 %d（count=%d, stride=%d）",
			rel, rel+occupied, view.ByteLength, count, stride)
	}
}

func (v *validator) checkAccessors() {
	for i, acc := range v.root.Accessors {
		path := fmt.Sprintf("accessors[%d]", i)
		li, ok := deriveLayout(acc.ComponentType, acc.Type)
		if _, compOK := componentTypes[acc.ComponentType]; !compOK {
			v.res.errorf(path+".componentType", "COMPONENT_TYPE", "不支持的 componentType: %d", acc.ComponentType)
		}
		if _, shapeOK := shapes[acc.Type]; !shapeOK {
			v.res.errorf(path+".type", "ACCESSOR_TYPE", "不支持的 type: %q", acc.Type)
		}
		if !ok {
			continue
		}
		if acc.Count <= 0 {
			v.res.errorf(path+".count", "ACCESSOR_COUNT", "count 必须为正整数，实际 %d", acc.Count)
		}
		if acc.BufferView == nil {
			// 无 bufferView：以全零为底值，byteOffset 只能缺省或为 0。
			if acc.ByteOffset != 0 {
				v.res.errorf(path+".byteOffset", "OFFSET_ZERO_BASE",
					"无 bufferView 的访问器 byteOffset 只能缺省或为 0，实际 %d", acc.ByteOffset)
			}
		} else {
			view := v.viewByIndex(*acc.BufferView, path+".bufferView")
			if view != nil {
				stride := li.fullStride
				if view.ByteStride != 0 {
					stride = view.ByteStride
					if view.ByteStride >= 4 && view.ByteStride <= 252 &&
						view.ByteStride%4 == 0 && view.ByteStride < li.fullStride {
						v.res.errorf(path+".bufferView", "STRIDE_TOO_SMALL",
							"byteStride %d 小于完整布局步长 %d", view.ByteStride, li.fullStride)
					}
				}
				if acc.Count > 0 {
					v.checkDataRange(view, acc.ByteOffset, li, acc.Count, stride, path)
				}
			}
		}
		if acc.Sparse != nil {
			v.checkSparse(acc, li, path+".sparse")
		}
	}
}

func (v *validator) checkSparse(acc gltfAccessor, li layoutInfo, path string) {
	sp := acc.Sparse
	if sp.Count <= 0 {
		v.res.errorf(path+".count", "SPARSE_COUNT", "sparse.count 必须为正整数，实际 %d", sp.Count)
	} else if acc.Count > 0 && sp.Count > acc.Count {
		v.res.errorf(path+".count", "SPARSE_COUNT",
			"sparse.count %d 超过访问器 count %d", sp.Count, acc.Count)
	}

	// indices
	indicesPath := path + ".indices"
	comp, compOK := componentTypes[sp.Indices.ComponentType]
	if !compOK || (sp.Indices.ComponentType != 5121 && sp.Indices.ComponentType != 5123 && sp.Indices.ComponentType != 5125) {
		v.res.errorf(indicesPath+".componentType", "SPARSE_INDEX_TYPE",
			"sparse indices 仅支持 5121/5123/5125，实际 %d", sp.Indices.ComponentType)
	} else {
		indexLI := layoutInfo{comp: comp, shape: shapes["SCALAR"], colSize: comp.size, colStride: comp.size, fullStride: comp.size, lastSize: comp.size}
		view := v.viewByIndex(sp.Indices.BufferView, indicesPath+".bufferView")
		if view != nil {
			if view.ByteStride != 0 {
				v.res.errorf(indicesPath+".bufferView", "SPARSE_STRIDE", "sparse indices 的视图不得声明 byteStride")
			}
			if sp.Count > 0 {
				v.checkDataRange(view, sp.Indices.ByteOffset, indexLI, sp.Count, comp.size, indicesPath)
				v.checkSparseIndices(view, sp, comp, acc.Count, indicesPath)
			}
		}
	}

	// values
	valuesPath := path + ".values"
	view := v.viewByIndex(sp.Values.BufferView, valuesPath+".bufferView")
	if view != nil {
		if view.ByteStride != 0 {
			v.res.errorf(valuesPath+".bufferView", "SPARSE_STRIDE", "sparse values 的视图不得声明 byteStride")
		}
		if sp.Count > 0 {
			v.checkDataRange(view, sp.Values.ByteOffset, li, sp.Count, li.fullStride, valuesPath)
		}
	}
}

// checkSparseIndices 读取稀疏索引数据，校验严格递增且小于 count。
func (v *validator) checkSparseIndices(view *gltfBufferView, sp *gltfSparse, comp componentInfo, accessorCount int64, path string) {
	if v.bin == nil {
		return
	}
	start := view.ByteOffset + sp.Indices.ByteOffset
	end := start + sp.Count*int64(comp.size)
	if start < 0 || end > int64(len(v.bin)) {
		return // 范围错误已在 checkDataRange 报告
	}
	data := v.bin[start:end]
	prev := int64(-1)
	for i := int64(0); i < sp.Count; i++ {
		var value int64
		switch comp.size {
		case 1:
			value = int64(data[i])
		case 2:
			value = int64(binary.LittleEndian.Uint16(data[i*2:]))
		case 4:
			value = int64(binary.LittleEndian.Uint32(data[i*4:]))
		}
		if accessorCount > 0 && value >= accessorCount {
			v.res.errorf(path, "SPARSE_INDEX_RANGE",
				"稀疏索引 #%d 的值 %d 不小于访问器 count %d", i, value, accessorCount)
		}
		if value <= prev {
			v.res.errorf(path, "SPARSE_INDEX_ORDER",
				"稀疏索引 #%d 的值 %d 未严格递增（前一个为 %d）", i, value, prev)
		}
		prev = value
	}
}
