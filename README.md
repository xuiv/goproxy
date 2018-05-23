# goproxy
这代码和谷歌IP快被防火墙封得没命了，似乎只能用quic才能连得上，而且没几个IP可以用了。

编译方法:
```
mkdir ~/goproxy
go get -u github.github.com/xuiv/goproxy
cd $GOPATH/src/github.com/xuiv/goproxy
find -iname "*.json" -exec cp {} ~/goproxy \;
cp ./httpproxy/filters/autoproxy/gfwlist.txt ~/goproxy
env GOOS=windows GOARCH=amd64 go build -a -ldflags "-w -s" -o ~/goproxy/goproxy.exe
```
