## 介绍
用于爬取chiphell论坛二手区板块交易贴

目前仅支持linux(x64)平台

## 使用方法

`wget -O chh-cr https://github.com/langchou/chh-cr/releases/latest/download/chh-cr-linux-amd64`

创建config.yaml文件，并填入对应的内容

``` yaml
cookies: "<chiphell cookies>"
dingtalk_token: "<钉钉机器人token>"
dingtalk_secret: "<钉钉机器人加签密钥>"
# 需要监控的板块
monitored_categories:
  - "显卡"
  - "处理器主板内存"
  - "笔记本/平板"
  - "手机通讯"
  - "影音娱乐"
  - "游戏设备"
  - "网络设备"
  - "外设"
user_keywords:
  "158********":
    - "RTX"
    - "显卡"
  "132********":
    - "iPhone"
    - "手机"
```
给脚本赋予权限
`sudo chmod +x chh-cr`

后台运行
`./chh-cr > output.log 2>&1 &`


## 特性

- 新帖监控
- 钉钉机器人通知
- 监控板块选择
- 老帖过滤（保存一个月）


## TODO

- [x] 关键词监控
- [ ] 热更新cookies
- [x] 钉钉群内通过手机号at对应用户
- [ ] docker支持
- [ ] ...



## 免责声明

本项目仅用于学习和研究目的，禁止用于任何非法或未经授权的爬取操作。使用者应遵守目标网站的 `robots.txt` 文件规定以及相关法律法规。开发者对使用本项目所产生的任何后果概不负责。
