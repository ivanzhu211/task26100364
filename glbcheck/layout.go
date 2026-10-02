package glbcheck

// 本文件负责字节布局规则：分量/形状元数据、步长与对齐计算。

// componentInfo 描述一种分量类型的元数据。
type componentInfo struct {
	size int    // 字节数
	name string // 人类可读名称
}

// componentTypes 映射 componentType 到元数据（小端读取）。
var componentTypes = map[int]componentInfo{
	5120: {size: 1, name: "int8"},
	5121: {size: 1, name: "uint8"},
	5122: {size: 2, name: "int16"},
	5123: {size: 2, name: "uint16"},
	5125: {size: 4, name: "uint32"},
	5126: {size: 4, name: "float32"},
}

// shapeInfo 描述一种形状（type）的元数据。
type shapeInfo struct {
	cols     int // 列数（标量/向量为 1）
	rows     int // 每列分量数
	isMatrix bool
}

// shapes 映射 type 字符串到元数据。
var shapes = map[string]shapeInfo{
	"SCALAR": {cols: 1, rows: 1},
	"VEC2":   {cols: 1, rows: 2},
	"VEC3":   {cols: 1, rows: 3},
	"VEC4":   {cols: 1, rows: 4},
	"MAT2":   {cols: 2, rows: 2, isMatrix: true},
	"MAT3":   {cols: 3, rows: 3, isMatrix: true},
	"MAT4":   {cols: 4, rows: 4, isMatrix: true},
}

// layoutInfo 是某个 accessor 的派生布局参数。
type layoutInfo struct {
	comp       componentInfo
	shape      shapeInfo
	colSize    int // 单列字节数（分量大小 * 行数）
	colStride  int // 列步长（矩阵按 4 字节对齐，非标量等于 colSize）
	fullStride int // 完整布局步长：相邻元素起点间距（含元素内部全部列填充）
	lastSize   int // 末元素实际占用：末列之后可省略填充
}

// align4 向上对齐到 4 字节。
func align4(n int) int {
	return (n + 3) &^ 3
}

// deriveLayout 计算分量类型与形状组合的布局参数。
func deriveLayout(componentType int, typ string) (layoutInfo, bool) {
	comp, ok := componentTypes[componentType]
	if !ok {
		return layoutInfo{}, false
	}
	shape, ok := shapes[typ]
	if !ok {
		return layoutInfo{}, false
	}
	li := layoutInfo{comp: comp, shape: shape}
	li.colSize = comp.size * shape.rows
	if shape.isMatrix {
		li.colStride = align4(li.colSize)
	} else {
		li.colStride = li.colSize
	}
	li.fullStride = li.colStride * shape.cols
	li.lastSize = li.colStride*(shape.cols-1) + li.colSize
	return li, true
}

// occupiedBytes 计算 count 个元素在给定步长下的最小占用字节数。
// 元素之间保留完整列填充，仅最后元素的末列之后可省略填充。
func (li layoutInfo) occupiedBytes(count int64, stride int) int64 {
	if count <= 0 {
		return 0
	}
	return (count-1)*int64(stride) + int64(li.lastSize)
}
