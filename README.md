# AWS S3 SDK for Go

此 SDK 为 AWS SDK for Go 修改版， 我们添加了客户端负载均衡和保活功能，并删除了除 S3 外的部分代码。

## 快速开始

### 编译

请参考 [使用文档](./doc/README.md)

### 配置SDK

**SDK 当前支持保活模式：**

1.	自动保活：当存储网关提供了用于获取网关列表的API时，SDK 采用API 方式去服务端拉取最新的网关列表，这种方式无需用户手动更新网关列表。

当存储网关无法提供获取网关列表的API时, SDK 将仅使用初始化网关列表（见“配置存储网关列表”）中的存储网关，当网关失效时， 则

将对应的地址加入黑名单，并定时探测存储网关是否恢复。所以如果存储网关不提供获取网关列表的API时，请在配置初始化网关列表时

，列出所有网关地址。

下面将介绍SDK 具体的配置：

#### a. 创建保存配置文件的文件夹：

SDK默认配置文件都保存于用户home目录下名为’.aws’ 的文件夹中。所以在配置SDK前请创建目录 `~/.aws’。

#### b. AK/SK 配置

新建配置文件 `~/.aws/credentials`.

在此文件中写入AK和 SK， 具体格式如下：

```
 [default]
aws_access_key_id = YOUR_AK
aws_secret_access_key = YOUR_SK
```

> 注：请将 “YOUR_AK” 和 “YOUR_SK” 替换为正确的 AK/SK。

#### c. 配置存储网关列表

新建配置文件 `~/.aws/endpoints`

在此文件中写入初始的网关列表。存储网关列表中记录着部分或者全部存储网关的地址，每条地址占一行，具体格式如下：

```
http://10.0.0.1:8080
http://10.0.0.2:8080
http://10.0.0.3:8080
```

> 说明：本地必须要保存一份初始网关列表，SDK初始化时需要通过初始网关列表获取服务端的地址。初始化完成后SDK会去服务
>   端拉取最新的网关列表，并定时保活。但是请注意，SDK并不会更新初始网关列表，所以当初始化网关列表中所有地址均失效时，
>   需要手动更新此列表。

#### d. 开启负载均衡和保活

SDK 通过判断是否传入了初始存储网关列表来决定是否开启负载均衡和保活。 当未配置网关列表时，将以原始的 golang sdk
模式运行，当配置网关列表后将自动开启负载均衡和保活。具体配置如下：

```
config := &aws.Config {
	EndpointsPath: aws.String("/root/.aws/endpoints"), // 指定初始网关列表的路径
	...
	S3ForcePathStyle: aws.Bool(true),
}
```

#### z. 配置说明

ABC Storage S3 CPP Go 新增了3 个配置选项，

MaxNetworkErrorRetries :

描    述:  同一 Endpoint 出现多少次网络错误后，切换endpoint（无需手动配置）

是否必需:  否

默认值  :  0


KeepAliveInterval:

描    述:  探活的周期（秒）

是否必需:  否

默认值  :  60


EndpointsPath:

描    述:  初始网关列表地址

是否必需:  否

默认值  :  空


### 使用SDK

请参考 [AWS SDK for Go 官方文档](https://docs.aws.amazon.com/sdk-for-go/api/service/s3/)

## 测试

```
go test -v $1 -coverprofile=c.out // 请手动替换要测试的文件或者目录
go tool cover -html=c.out -o cover.html
```

## 如何贡献

迎您修改和完善此项目，请直接提交PR 或 issues。

* 提交代码时请保证良好的代码风格。
* 提交 issues 时， 请翻看历史 issues， 尽量不要提交重复的issues。

## 讨论

欢迎提 issues。
