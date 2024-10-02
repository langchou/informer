## 介绍
用于爬取各个论坛二手区板块交易贴

## 使用方法

### 使用Docker

创建data/config.yaml文件，并填入对应的内容

``` yaml
dingtalk:
  token: ""
  secret: ""

chiphell:
  cookies: ""
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
      - "iphone"
      - "显卡"
    "177********":
      - "iphone"
  wait_time_range:
    min: 2
    max: 5

log:
  file: "data/app.log"
  max_size: 10
  max_backups: 5
  max_age: 30
  compress: true

```

`docker run -d --name informer-v ./data:/app/data jontyding/informer:latest`

### 使用二进制文件
`wget -O informer https://github.com/langchou/informer/releases/latest/download/informer-linux-amd64 && chmod +x informer`

创建data/config.yaml文件，并填入对应的内容

``` yaml
dingtalk:
  token: ""
  secret: ""

chiphell:
  cookies: ""
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
      - "iphone"
      - "显卡"
    "177********":
      - "iphone"
  wait_time_range:
    min: 2
    max: 5

log:
  file: "data/app.log"
  max_size: 10
  max_backups: 5
  max_age: 30
  compress: true

```

后台运行
`./informer &`


## 特性

- 新帖监控
- 钉钉机器人通知
- 监控板块选择
- 老帖过滤（保存一个月）


## TODO

- [x] 关键词监控
- [ ] 热更新cookies
- [x] 钉钉群内通过手机号at对应用户
- [x] docker支持
- [x] 解耦功能
- [ ] 支持多个论坛监控


## 免责声明

本项目仅用于学习和研究目的，禁止用于任何非法或未经授权的爬取操作。使用者应遵守目标网站的 `robots.txt` 文件规定以及相关法律法规。开发者对使用本项目所产生的任何后果概不负责。
