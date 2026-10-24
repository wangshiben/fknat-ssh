# fknat-SSH

`fknat-ssh` 是专用于 SSH 的 FKNAT 服务端运行程序。它通过 FKNAT 发布一个本地
SSH 后端，并支持生产环境准入、自动发现 Index、持久化节点身份以及逻辑路由 `0`。

```bash
sudo fknat-ssh install --billing-key /secure/billing-key.json
fknat-ssh address
fknat-ssh status
fknat-ssh doctor
```

扣费密钥仅通过路径引用，绝不会被复制或打印。默认后端为 `127.0.0.1:22`；可以使用
`--ssh-target 127.0.0.1:2222` 选择其他本机回环 SSH 端口。程序会拒绝非回环地址，
从而避免这项专用服务变成通用的内网代理。

在 Linux 上，`install` 会写入 `/etc/systemd/system/fknat-ssh.service`，启用该服务
并立即启动。持久化身份、状态和最小配置存放在 `/var/lib/fknat-ssh` 下；配置中只包含
扣费密钥的路径，不包含密钥内容。使用 `--no-service` 可以只写入本地状态而不改动
systemd，之后可通过 `fknat-ssh run --config PATH` 在前台运行。

`status --json` 提供机器可读的状态信息。`address` 会读取已安装配置所选择的身份；
对于非默认安装，请传入 `--config PATH`。`doctor --config PATH` 会检查扣费密钥路径、
身份、SSH banner 以及生产环境的 Index 发现功能。

该运行程序依赖同级 `fknat` 模块中的可复用 API。使用以下命令构建和测试：

```bash
go test ./...
go build ./cmd/fknat-ssh
```

首个版本暂未启用 Windows Service 和 macOS launchd 集成；`doctor` 和服务相关命令会
明确报告这一平台限制。
