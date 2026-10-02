package glbcheck

import (
	"encoding/json"
	"fmt"
)

// 本文件定义预检所需的 glTF 2.0 JSON 子集类型。
// 仅解析与字节布局、引用相关的字段，其余字段忽略。

// gltfRoot 是 glTF JSON 文档的根。
type gltfRoot struct {
	ExtensionsUsed     []string         `json:"extensionsUsed"`
	ExtensionsRequired []string         `json:"extensionsRequired"`
	Buffers            []gltfBuffer     `json:"buffers"`
	BufferViews        []gltfBufferView `json:"bufferViews"`
	Accessors          []gltfAccessor   `json:"accessors"`
	Images             []gltfImage      `json:"images"`
}

type gltfBuffer struct {
	ByteLength int64  `json:"byteLength"`
	URI        string `json:"uri"`
}

type gltfBufferView struct {
	Buffer     int   `json:"buffer"`
	ByteOffset int64 `json:"byteOffset"`
	ByteLength int64 `json:"byteLength"`
	ByteStride int   `json:"byteStride"`
	Target     int   `json:"target"`
}

type gltfAccessor struct {
	BufferView    *int        `json:"bufferView"`
	ByteOffset    int64       `json:"byteOffset"`
	ComponentType int         `json:"componentType"`
	Count         int64       `json:"count"`
	Type          string      `json:"type"`
	Sparse        *gltfSparse `json:"sparse"`
}

type gltfSparse struct {
	Count   int64             `json:"count"`
	Indices gltfSparseIndices `json:"indices"`
	Values  gltfSparseValues  `json:"values"`
}

type gltfSparseIndices struct {
	BufferView    int   `json:"bufferView"`
	ByteOffset    int64 `json:"byteOffset"`
	ComponentType int   `json:"componentType"`
}

type gltfSparseValues struct {
	BufferView int   `json:"bufferView"`
	ByteOffset int64 `json:"byteOffset"`
}

type gltfImage struct {
	URI string `json:"uri"`
}

// parseDocument 解析 JSON 块并做文档级检查（扩展、外部 URI 引用）。
func parseDocument(data []byte, res *Result) (*gltfRoot, bool) {
	var root gltfRoot
	if err := json.Unmarshal(data, &root); err != nil {
		res.errorf("json", "JSON_INVALID", "JSON 块解析失败: %v", err)
		return nil, false
	}
	if len(root.ExtensionsUsed) > 0 || len(root.ExtensionsRequired) > 0 {
		res.errorf("extensions", "EXTENSION_UNSUPPORTED",
			"不支持扩展: used=%v required=%v", root.ExtensionsUsed, root.ExtensionsRequired)
	}
	for i, img := range root.Images {
		if img.URI != "" {
			res.errorf(fmt.Sprintf("images[%d]", i), "URI_UNSUPPORTED",
				"不支持外部 URI 引用: %s", img.URI)
		}
	}
	return &root, true
}
