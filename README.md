# packmgr

`packmgr` 是一个用 Go 编写的轻量包管理工具，既可以维护 `packages.json`，也可以按当前机器的系统平台和 CPU 架构自动选择 release bundle，下载、校验并安装到指定目录。

当前版本只支持 GitHub public release 的 `release` 模式 manifest，不支持 `smoke` 模式。

## 安装

如果你本机已安装 [Go](https://go.dev/dl/)（本仓库要求 **Go 1.23+**），可以直接用 `go install` 从源码安装到 `$(go env GOPATH)/bin`（请确保该目录已在 `PATH` 中）：

```bash
go install github.com/CDRlease/packmgr/cmd/packmgr@latest
```


从 release 下载当前平台对应的 zip，解压后直接执行：

```bash
./packmgr version
```

如果你想自己本地构建：

```bash
make build
./bin/packmgr version
```

如果你想把 `packmgr` 安装到 Go 的 bin 目录，并加入当前用户 shell 的 PATH：

```bash
make install VERSION=v0.1.0
```

这个命令会把二进制安装到 `$(go env GOPATH)/bin`，并把这个目录写入你的 shell 启动文件。
安装完成后，重新打开一个终端，或者执行 `source ~/.zshrc`，就可以直接调用：

```bash
packmgr version
```

## 使用方法

```bash
packmgr install --dir ./vendor
```

默认会读取当前目录下的 `./packages.json`，也可以通过 `--packages` 指定其他路径：

```bash
packmgr install --packages ./examples/packages.json --dir ./vendor
```

### 配置维护命令

```bash
packmgr packages list
packmgr packages get server
packmgr packages add server --repo CDRlease/tgr_server --tag latest --check-release
packmgr packages update server --tag latest --check-release
packmgr packages remove server
```

- 所有 `packmgr packages ...` 命令默认读取 `./packages.json`
- `add` 在文件不存在时会自动创建新的 `packages.json`
- `list` 和 `get` 支持 `--json`
- `add` 和 `update` 支持 `--check-release`，会在写文件前检查 GitHub release 是否存在
- `tag: "latest"` 表示 GitHub 官方 `latest release`，会在安装或 `--check-release` 时动态解析

### 查询输出示例

```bash
packmgr packages list
```

```text
codegen repo=CDRlease/tgr_codegen tag=v0.4.4
config repo=CDRlease/tgr_config tag=v0.1.1
engine repo=CDRlease/tgr_engine tag=v0.1.1
server repo=CDRlease/tgr_server tag=v0.2.2
```

```bash
packmgr packages get server
```

```text
name: server
repo: CDRlease/tgr_server
tag: v0.2.2
```

### `packages.json` 示例

```json
{
  "schemaVersion": 1,
  "components": {
    "server": {
      "repo": "CDRlease/tgr_server",
      "tag": "v0.2.2"
    },
    "engine": {
      "repo": "CDRlease/tgr_engine",
      "tag": "v0.1.1"
    },
    "config": {
      "repo": "CDRlease/tgr_config",
      "tag": "v0.1.1"
    },
    "codegen": {
      "repo": "CDRlease/tgr_codegen",
      "tag": "v0.4.4"
    }
  }
}
```

也支持在配置里直接写 `latest`：

```json
{
  "schemaVersion": 1,
  "components": {
    "server": {
      "repo": "CDRlease/tgr_server",
      "tag": "latest"
    }
  }
}
```

## 安装结果

安装结果是“扁平组件目录”：

- 不保留版本号目录
- 不保留 `os-arch` 目录
- 不保留 zip 的最外层包装目录，例如 `bin/` 或 `codegen-osx-arm64/`
- 组件根目录直接保留 payload 文件，以及上游原始 `manifest.json` 和 `SHA256SUMS.txt`
- 当组件配置为 `tag: "latest"` 时，安装日志会显示原始值 `latest`，以及本次实际解析到的 `resolved tag`

例如：

```text
vendor/
├── server/
│   ├── manifest.json
│   ├── SHA256SUMS.txt
│   ├── run.sh
│   └── mesh/mesh
├── engine/
│   ├── manifest.json
│   ├── SHA256SUMS.txt
│   └── lockstep.engine.dll
├── config/
│   ├── manifest.json
│   ├── SHA256SUMS.txt
│   ├── Luban.dll
│   └── gen.sh
└── codegen/
    ├── manifest.json
    ├── SHA256SUMS.txt
    ├── lockstep.ecs.generator.dll
    ├── Config/HashPrimes.json
    └── scripts/gen.sh
```

## 环境变量

- `PACKMGR_GITHUB_TOKEN`
- `GH_TOKEN`
- `GITHUB_TOKEN`

优先级从上到下。未设置时将匿名访问 GitHub public releases。

## 常见错误

- `schemaVersion must be 1`：`packages.json` 结构不符合 v0.1.0 协议。
- `component <name> already exists`：新增组件时发现重名。
- `component <name> not found`：查询、更新或删除时找不到指定组件。
- `fetch latest release <repo>`：指定了 `latest`，但该仓库没有可用的 GitHub latest release。
- `no compatible bundle found`：上游 release 没有当前平台可用的 bundle，也没有 `any-any` 兜底包。
- `checksum entry not found`：`SHA256SUMS.txt` 缺少选中 zip 或 `manifest.json` 的校验项。
- `unsafe zip entry path`：zip 内包含绝对路径或路径穿越条目，安装被阻止。
