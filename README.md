glog 
====
--- a simple &amp; fast logging package for Golang


<pre>

<b>How to use it</b>
	1. import the package
	2. usage like this:
		glog, err := glog.New()
		glog.Error("Some Content")
		glog.Warn("Some Content")
		.....
		glog.Close() //Don't forget, otherwise the last log may be lost


<b>Feature</b>
	1. use a separate goroutine processing log, no block main-goroutine.
	2. support 5 levels of the log (Error, Warning, Notice, Info, Debug)
	3. each level of the log can be configured to is print and the logfile
	4. supports 4 types of split (Day, Hour, Minute, Size of MB)
	5. supports for custom flush time, auto flush when the writeBufs using 90%
	6. supports for custom the logfile mode, default:0400

<b>benchmark</b>
	on OSX 10.9, Intel Core i7 2.3GHz, 4GB Memory, HDD disk, 
	Write 1 million times takes 4.2 seconds
	use go test -bench, the result like:
	<pre>
		Benchmark_Write	  500000	      3884 ns/op
		ok  	glog	2.046s
	</pre>

<b>Advanced</b>
	more of Advanced usage , please see wiki: https://github.com/syupei/glog/wiki

</pre>


glog
====
(Chinese Version) --- 一个基于Golang语言的、简单、快速的日志包

<pre>

<b>如何使用它?</b>
	1.引入该包
	2.如下面的方法使用
		l, err := glog.New()
		l.Error("Some Content")
		.....
		l.Close()  //不要忘记关闭，否则最后的日志可能会丢掉

<b>特性</b>
	1. 使用独立的goroutine处理日志，不阻塞主线程
	2. 支持五种级别的日志类型 (Error, Warning, Notice, Info, Debug)
	3. 可以为每种级别单独配置是否打印到屏幕、不同的级别记录到不同的日志文件
	4. 支持四种类型的自动切分文件 (按天、按指定的小时数、按指定的分钟数、按指定的文件大小)
	5. 支持自定义刷盘时间，并且当写缓冲区使用到达90%时自动刷盘，以避免阻塞
	6. 支持自定义日志文件的读写属性, 默认为 0400

<b>性能测试</b>
	在我的OSX 10.9, Intel Core i7 2.3GHz, 4GB 内存, HDD 硬盘上
	写入一百万条日志耗时4.2秒
	使用标准的 go test -bench， 返回结果如下:
	<pre>
		Benchmark_Write	  500000	      3884 ns/op
		ok  	glog	2.046s
	</pre>	

<b>高级</b>
	更多的高级特性,请阅读wiki: https://github.com/syupei/glog/wiki

</pre>














