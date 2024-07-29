# basics

[![Hits](https://hits.seeyoufarm.com/api/count/incr/badge.svg?url=https%3A%2F%2Fgithub.com%2Foneclickvirt%2Fbasics&count_bg=%232EFFF8&title_bg=%23555555&icon=&icon_color=%23E7E7E7&title=hits&edge_flat=false)](https://hits.seeyoufarm.com) [![Build and Release](https://github.com/oneclickvirt/basics/actions/workflows/main.yaml/badge.svg)](https://github.com/oneclickvirt/basics/actions/workflows/main.yaml)

系统基础信息查询模块 (System Basic Information Query Module)

Include: https://github.com/oneclickvirt/gostun

## 说明

- [x] 以```-l```指定输出的语言类型，可指定```zh```或```en```，默认不指定时使用中文输出
- [x] 使用```sysctl```获取CPU信息-特化适配freebsd、openbsd系统
- [x] 适配```MacOS```与```Windows```系统的信息查询
- [x] 部分Windows10系统下打勾打叉编码错误显示，已判断是Win时使用Y/N显示而不是勾叉
- [x] 检测GPU相关信息，参考[ghw](https://github.com/jaypipes/ghw)

## TODO

- [ ] 特化ASN和归属地查不出来的时候使用备用查询的API进行查询
- [ ] 纯IPV6环境下使用cdn反代获取平台信息

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
  -e    Enable logging
  -h    Show help information
  -l string
        Set language (en or zh)
  -v    Show version
```

![图片](https://github.com/oneclickvirt/basics/assets/103393591/634064de-17a6-485f-b401-dc3a159a18c4)

![图片](https://github.com/oneclickvirt/basics/assets/103393591/49404a18-1717-4875-b50d-26a930238248)

## 卸载

```
rm -rf /root/basics
rm -rf /usr/bin/basics
```

## 在Golang中使用

```
go get github.com/oneclickvirt/basics@latest
```
