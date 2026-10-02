package glbcheck

import "encoding/json"

// gltf 是预检所需的 glTF 2.0 JSON 子集。
type gltf struct {
	Asset           asset        `json:"asset"`
	Buffers         []buffer     `json:"buffers"`
	BufferViews     []bufferView `json:"bufferViews"`
	Accessors       []accessor   `json:"accessors"`
	ExtensionsUsed  []string     `json:"extensionsUsed"`
	ExtensionsRequd []string     `json:"extensionsRequired"`
}

type asset struct {
	Version string `json:"version"`
}

type buffer struct {
	ByteLength int64  `json:"byteLength"`
	URI        string `json:"uri"`
}

type bufferView struct {
	Buffer     int    `json:"buffer"`
	ByteOffset int64  `json:"byteOffset"`
	ByteLength int64  `json:"byteLength"`
	ByteStride *int64 `json:"byteStride"`
}

type accessor struct {
	BufferView    *int    `json:"bufferView"`
	ByteOffset    int64   `json:"byteOffset"`
	ComponentType int64   `json:"componentType"`
	Count         int64   `json:"count"`
	Type          string  `json:"type"`
	Sparse        *sparse `json:"sparse"`
}

type sparse struct {
	Count   int64         `json:"count"`
	Indices sparseIndices `json:"indices"`
	Values  sparseValues  `json:"values"`
}

type sparseIndices struct {
	BufferView    int   `json:"bufferView"`
	ByteOffset    int64 `json:"byteOffset"`
	ComponentType int64 `json:"componentType"`
}

type sparseValues struct {
	BufferView int   `json:"bufferView"`
	ByteOffset int64 `json:"byteOffset"`
}

// parseGLTF 解析并做 JSON 层面的基本校验。
func parseGLTF(jsonChunk []byte, rep *Report) *gltf {
	var g gltf
	if err := json.Unmarshal(jsonChunk, &g); err != nil {
		rep.errorf("gltf.invalidJSON", "", "JSON 块解析失败: %v", err)
		return nil
	}
	if g.Asset.Version != "2.0" {
		rep.errorf("gltf.unsupportedVersion", "asset.version", "glTF 版本 %q 不受支持，仅支持 2.0", g.Asset.Version)
		return nil
	}
	if len(g.ExtensionsUsed) > 0 || len(g.ExtensionsRequd) > 0 {
		rep.errorf("gltf.extensionsUnsupported", "extensionsUsed", "不支持任何扩展: used=%v required=%v", g.ExtensionsUsed, g.ExtensionsRequd)
		return nil
	}
	return &g
}
