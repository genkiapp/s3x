## Minio [![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/minio/minio?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

Minio is a minimal cloud storage server for Micro Services. Written in Golang and licensed under [Apache license v2](./LICENSE). Compatible with Amazon S3 APIs.

## Description

Micro services environment provisions one Minio server per application instance. Scalability is achieved through large number of smaller personalized instances. This version of the Minio binary is built using Filesystem storage backend for magnetic and solid state disks. 

## Minio Client

[Minio Client (mc)](https://github.com/minio/mc#minio-client-mc-) provides a modern alternative to Unix commands like ``ls``, ``cat``, ``cp``, ``sync``, and ``diff``. It supports POSIX compatible filesystems and Amazon S3 compatible cloud storage systems. It is entirely written in Golang.

## Amazon S3 Compatible Client Libraries
- [Golang Library](https://github.com/minio/minio-go)
- [Java Library](https://github.com/minio/minio-java)
- [Nodejs Library](https://github.com/minio/minio-js)
- [Python Library](https://github.com/minio/minio-py)
- [.Net Library](https://github.com/minio/minio-dotnet)

### Install [![Build Status](https://travis-ci.org/minio/minio.svg?branch=master)](https://travis-ci.org/minio/minio)[![Build status](https://ci.appveyor.com/api/projects/status/royh137dni8yevep/branch/master?svg=true)](https://ci.appveyor.com/project/harshavardhana/minio-qxbjq/branch/master)

#### Linux, OS X, Windows

~~~
$ go get -u github.com/minio/minio
~~~

### How to use Minio?

```
NAME:
  minio server - Start Minio cloud storage server.

USAGE:
  minio server [OPTION VALUE] PATH

  OPTION = expiry        VALUE = NN[h|m|s] [DEFAULT=Unlimited]
  OPTION = min-free-disk VALUE = NN% [DEFAULT: 10%]

EXAMPLES:
  1. Start minio server on Linux.
        $ minio server /home/shared

  2. Start minio server on Windows.
        $ minio server C:\MyShare

  3. Start minio server bound to a specific IP:PORT, when you have multiple network interfaces.
        $ minio --address 192.168.1.101:9000 server /home/shared

  4. Start minio server with minimum free disk threshold to 5%
        $ minio server min-free-disk 5% /home/shared/Pictures

  5. Start minio server with minimum free disk threshold to 15% with auto expiration set to 1h
        $ minio server min-free-disk 15% expiry 1h /home/shared/Documents
```

#### Start Minio server.

~~~
$ minio server ~/Photos
AccessKey: G5GJRH51R2HSUWYPGIX5  SecretKey: uxhBC1Yscut3/u81l5L8Yp636ZUk32N4m/gFASuZ

To configure Minio Client.

	$ wget https://dl.minio.io:9000/updates/2015/Oct/linux-amd64/mc
	$ chmod 755 mc
	$ ./mc config host add localhost:9000 G5GJRH51R2HSUWYPGIX5 uxhBC1Yscut3/u81l5L8Yp636ZUk32N4m/gFASuZ
	$ ./mc mb localhost/photobucket
	$ ./mc cp ~/Photos... localhost/photobucket

Starting minio server:
Listening on http://127.0.0.1:9000
Listening on http://172.30.2.17:9000
~~~

#### How to use AWS SDK with Minio?

Please follow the documentation here - [Using aws-sdk-go with Minio server](./AWS-SDK-GO.md)

### Contribute to Minio Project
Please follow Minio [Contributor's Guide](./CONTRIBUTING.md)

