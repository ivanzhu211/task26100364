# glbcheck

GLB 资产预检库，供资产入库和本地预览诊断调用。源文件存在本地目录，结果仅保存在内存。

仅做整文件预检：容器结构、字节布局与引用合法性。不检查材质、节点或几何质量。

## 能力范围

- GLB 2.0 容器：魔数、版本、总长度、JSON 首块 + 可选单个 BIN 块、块长度与 4 字节对齐。
- buffer：最多一个内嵌 buffer，拒绝外部 URI、压缩与扩展；BIN 只允许比 `buffer.byteLength` 多 0~3 字节零填充；所有 bufferView 以声明的 buffer 长度为边界。
- bufferView / accessor / sparse 全量引用检查：
  - 分量类型 5120~5123、5125、5126（int8/uint8/int16/uint16/uint32/float32），小端读取。
  - 形状 SCALAR、VEC2~VEC4、MAT2~MAT4；矩阵按列存放，列绝对起点 4 字节对齐，仅最后元素末列后可省略填充。
  - `byteStride` 缺省为紧密布局步长；显式值须为 4~252 的 4 倍数且不小于完整布局步长。
  - 访问器相对/绝对偏移按分量大小对齐，最后实际占用字节不得越过视图。
  - 无 bufferView 的访问器以全零为底值，`byteOffset` 只能缺省或为 0。
  - sparse 数量不超过 count；indices 仅支持 5121/5123/5125，索引严格递增且小于 count；indices/values 视图不得声明 byteStride，values 遵循相同矩阵布局。

## 用法

```go
rep, err := glbcheck.CheckFile("/path/to/asset.glb") // 本地文件
rep := glbcheck.CheckBytes(data)                      // 内存字节流
if rep.OK() { /* 通过 */ }
for _, iss := range rep.Issues {
    // iss.Code 机器可读错误码，iss.Path 出错位置，iss.Message 描述
}
```

## 测试

```sh
go test ./...
```
