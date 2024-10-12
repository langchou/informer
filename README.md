## 介绍
用于爬取各个论坛二手区板块交易贴

## 使用方法

### 使用Docker

创建data/config.yaml文件，并填入对应的内容

``` yaml
logconfig:
  file: "data/app.log"
  maxSize: 10
  maxBackups: 5
  maxAge: 30
  compress: true

dingtalk:
  token: ""
  secret: ""

proxyPoolAPI: ""   # 代理池api，默认不填写即可

forums:
  chiphell:
    cookies: ""
    userKeyWords:
      "158********":
        - "iphone"
      "177********":
        - "iphone"
    waitTimeRange:
      min: 2
      max: 5
```

`docker run -d --name informer-v ./data:/app/data jontyding/informer:latest`

### 使用二进制文件
`wget -O informer https://github.com/langchou/informer/releases/latest/download/informer-linux-amd64 && chmod +x informer`

创建data/config.yaml文件，并填入对应的内容

``` yaml
logconfig:
  file: "data/app.log"
  maxSize: 10
  maxBackups: 5
  maxAge: 30
  compress: true

dingtalk:
  token: ""
  secret: ""

forums:
  chiphell:
    cookies: ""
    userKeyWords:
      "158********":
        - "iphone"
      "158********":
        - "iphone"
    waitTimeRange:
      min: 2
      max: 5
```

后台运行
`./informer &`


## 特性

- 新帖和关键词监控
- 钉钉机器人通知，通过手机号at对应用户
- 监控板块选择
- 老帖过滤（保存一个月）
- 支持docker部署


## TODO

- [ ] 热更新cookies
- [ ] 支持多个论坛监控


## 二次开发指南


### action二进制编译
1、fork本仓库
2、点击github头像下的Settings，进入到设置页面，选择左侧导航栏下的Developer Settings，再在左侧导航栏下选择Personal access tokens下的Token，创建一个新的Token，
Note填写informer-develop，Expiration选择No expiration，下面Select scopes中将repo相关的所有都选上，然后Generate token，记录ghp_xxxxx这串字符串
3、进入fork的仓库，选择Setting，左侧导航栏选择Secrets and variables下选择Actions，选择Repository secrets中 New repository secrets，创建 key为PERSONAL_ACCESS_TOKEN，value为刚才保存的ghp_xxxxx字符串

### action+docker编译

1、登录docker hub，点击头像选择Account settings，进入Personal access tokens，选择Generate new token，记录生成的这串字符串
2、进入fork的仓库，选择Setting，左侧导航栏选择Secrets and variables下选择Actions，选择Repository secrets中 New repository secrets，创建 key为DOCKER_HUB_ACCESS_TOKEN，value为刚才保存的字符串，再创建一个secrets，key为DOCKER_USERNAME，value为你的docker用户名


## 免责声明

本项目仅用于学习和研究目的，禁止用于任何非法或未经授权的爬取操作。使用者应遵守目标网站的 `robots.txt` 文件规定以及相关法律法规。开发者对使用本项目所产生的任何后果概不负责。
