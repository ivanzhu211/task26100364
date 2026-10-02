# task26100364 — GLB 资产预检库

Go 实现的 GLB 2.0 资产入库前整文件预检库，供资产入库与本地预览诊断调用。
仅检查容器结构、字节布局与引用合法性，**不**检查材质、节点或几何质量。
结果仅保存在内存，不做任何持久化。

## 用法

```go
import "glbprecheck/glbcheck"

res, err := glbcheck.CheckFile("/path/to/asset.glb") // 本地文件
res := glbcheck.Check(data)                          // 或内存字节
if !res.OK() {
    for _, issue := range res.Issues {
        // issue.Code / issue.Message / issue.Path
    }
}
```

## 检查范围

- 容器：魔数、版本 2、总长度、块长度与 4 字节对齐、JSON 块在前、最多一个 BIN 块、无未知块。
- Buffer：最多一个内嵌 buffer，拒绝外部 URI；BIN 长度仅允许比 `buffer.byteLength` 多 0–3 字节零填充；所有 bufferView 以声明长度为边界。
- 扩展与外部引用：`extensionsUsed`/`extensionsRequired`、image/buffer 的外部 URI 一律拒绝。
- Accessor：分量类型 5120–5126（不含 5124）、形状 SCALAR/VEC2–VEC4/MAT2–MAT4；`count` 为正整数；
  矩阵列绝对起点 4 字节对齐，元素间保留完整列填充，仅末元素末列后可省略填充；
  `byteStride` 缺省为紧密步长，显式值须为 4–252 的 4 倍数且不小于完整布局步长；
  相对/绝对偏移按分量大小对齐，末字节不得越过视图；无 bufferView 时 `byteOffset` 只能缺省或为 0。
- Sparse：覆盖数量不超过 `count`；indices 仅支持 5121/5123/5125，索引严格递增且小于 `count`；
  indices/values 视图不得声明 `byteStride`，偏移、对齐、范围与矩阵布局规则同访问器。

## 测试

```sh
go test ./...
```

48 个用例覆盖交错步长、矩阵列填充、恰好结束与少 1 字节边界、
重复/倒序/越界稀疏索引、容器与引用非法情形。
