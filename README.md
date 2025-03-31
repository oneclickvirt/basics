# basics

[![Hits](https://hits.spiritlhl.net/basics.svg?action=hit&title=Hits&title_bg=%23555555&count_bg=%230eecf8&edge_flat=false)](https://hits.spiritlhl.net)

[![Build and Release](https://github.com/oneclickvirt/basics/actions/workflows/main.yaml/badge.svg)](https://github.com/oneclickvirt/basics/actions/workflows/main.yaml)

系统基础信息查询模块 (System Basic Information Query Module)

Include: https://github.com/oneclickvirt/gostun

## 说明

- [x] 以```-l```指定输出的语言类型，可指定```zh```或```en```，默认不指定时使用中文输出
- [x] 使用```sysctl```获取CPU信息-特化适配freebsd、openbsd系统
- [x] 适配```MacOS```与```Windows```系统的信息查询
- [x] 部分Windows10系统下打勾打叉编码错误显示，已判断是Win时使用Y/N显示而不是勾叉
- [x] 检测GPU相关信息，参考[ghw](https://github.com/jaypipes/ghw)

## TODO

- [ ] 适配MACOS系统的识别

## Usage

下载及安装

```
curl https://raw.githubusercontent.com/oneclickvirt/basics/main/basics_install.sh -sSf | bash
```

或

```
curl https://cdn.spiritlhl.net/https://raw.githubusercontent.com/oneclickvirt/basics/main/basics_install.sh -sSf | bash
```

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
  -log    Enable logging
  -h      Show help information
  -l string
          Set language (en or zh)
  -v      Show version
```

![图片](https://github.com/user-attachments/assets/42b470c8-0a40-474e-98e5-6fe99009b593)

![图片](https://github.com/user-attachments/assets/414f47b3-1708-4a9a-96a7-4e21b30a7b4e)

## 卸载

```
rm -rf /root/basics
rm -rf /usr/bin/basics
```

## 在Golang中使用

```
go get github.com/oneclickvirt/basics@latest
```
