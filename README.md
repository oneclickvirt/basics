# basics

[![Hits](https://hits.spiritlhl.net/basics.svg?action=hit&title=Hits&title_bg=%23555555&count_bg=%230eecf8&edge_flat=false)](https://hits.spiritlhl.net)

[![Build and Release](https://github.com/oneclickvirt/basics/actions/workflows/main.yaml/badge.svg)](https://github.com/oneclickvirt/basics/actions/workflows/main.yaml)

系统基础信息查询模块 (System Basic Information Query Module)

Include: https://github.com/oneclickvirt/gostun

## 说明

- [x] 以```-l```指定输出的语言类型，可指定```zh```或```en```，默认不指定时使用中文输出
- [x] 使用```sysctl```获取CPU信息，特化适配freebsd、openbsd系统
- [x] 适配```MacOS```与```Windows```系统的信息查询
- [x] 检测GPU相关信息，参考[ghw](https://github.com/jaypipes/ghw)
- [x] 支持自动切换为离线模式仅检测系统基础信息，不再检测网络信息
- [x] 提供进程内 DoH/DoT 解析回退组件，供上层和独立命令在在线且系统 DNS 确认不可用时使用；不会改写系统 resolver 文件，短暂超时会保留系统解析
- [x] 检测 CPU/cgroup、主板与 BIOS、PCI/GPU、NUMA/DIMM、HugePages、物理盘与 RAID、TCP 队列和缓冲信息

## 扩展信息说明

- `TCP加速/队列`：斜线前是拥塞控制算法（如 `cubic`、`bbr`），斜线后是队列规则（如 `fq`、`fq_codel`）。
- `TCP接收缓冲`、`TCP发送缓冲`：依次显示最小值、默认值和最大值，用于判断高延迟或高带宽连接是否可能受到缓冲区限制。
- `NUMA/DIMM`：斜线前是 NUMA 节点数，斜线后是检测到的 DIMM 数量；虚拟机未透传 DMI 信息时，DIMM 数量可能为 0。
- `HugePages`：在一行内显示总数、空闲数和单页大小，用于判断大页内存的配置及当前余量。
- `物理盘 N`：同一行显示该磁盘的协议、健康状态和可用时的温度；`unsupported` 表示当前硬件、驱动、权限或虚拟化环境未提供健康数据，不等于磁盘已经故障。

同一实体的紧密属性会合并为一行，独立含义的值仍分别显示。无法从当前系统可靠读取的信息不会推测补全。

## Usage

下载及安装

```
curl https://raw.githubusercontent.com/oneclickvirt/basics/main/basics_install.sh -sSf | bash
```

或

```
curl https://cdn.spiritlhl.net/https://raw.githubusercontent.com/oneclickvirt/basics/main/basics_install.sh -sSf | bash
```

在线但本地 DNS 不可用时，使用支持 DoH 的 curl（例如 curl 7.62+）先拉取安装脚本：

```
curl --doh-url https://cloudflare-dns.com/dns-query --resolve cloudflare-dns.com:443:1.1.1.1,1.0.0.1 -sSfL https://raw.githubusercontent.com/oneclickvirt/basics/main/basics_install.sh | bash
```

安装脚本会保留 `wget` 优先顺序；下载失败且 curl 支持 DoH 时，会自动通过内置 DoH 重试。

使用

```
basics
```

或

```
./basics
```

进行测试

无环境依赖，理论上适配所有系统和主流架构，更多架构请查看 https://github.com/oneclickvirt/basics/releases/tag/output

```
Usage: basics [options]
  -h      Show help information
  -json   Print the structured system report as JSON
  -l string
          Set language (en or zh)
  -dns-mode string
          DNS mode (auto, system, doh, or dot) (default "auto")
  -log    Enable logging
  -structured
          Print the structured system report as JSON
  -text   Print the structured hardware summary as compact text
  -timeout duration
          Structured report timeout (for example 10s)
  -v      Show version
```

`-timeout` 仅用于 `-json`、`-structured` 或 `-text`，传统实时文本模式不接受该参数。

`-dns-mode=auto` 默认保留系统 DNS；只有多个独立探测均确认本地解析器不可用，才会在当前进程选择经固定地址校验、实际 DNS 查询延迟最低的内置 DoH 或 DoT 上游。超时、SERVFAIL 和临时网络错误不会触发切换。`system` 禁用回退，`doh` 与 `dot` 分别强制对应的加密传输。内置地址清单由 `network/resolver/endpoints_sources.json` 定期校验后生成到 `endpoints_embed.json`；固定地址只用于引导这些上游，普通测试域名仍按实时 DNS 回应解析。

## 卸载

```
rm -rf /root/basics
rm -rf /usr/bin/basics
```

## 在Golang中使用

```
go get github.com/oneclickvirt/basics@v0.0.29
```

## 结果展示

![图片](https://github.com/user-attachments/assets/8c241b8a-4403-49a7-a17a-dbddf8783033)

![图片](https://github.com/user-attachments/assets/624d2aaa-ba1c-4bec-a6db-9701c0196c6f)
