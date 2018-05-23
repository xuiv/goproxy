# goproxy
```
mkdir ~/goproxy
go get -u github.github.com/xuiv/goproxy
cd $GOPATH/src/github.com/xuiv/goproxy
find -iname "*.json" -exec cp {} ~/goproxy \;
env GOOS=windows GOARCH=amd64 go build -a -ldflags "-w -s" -o ~/goproxy/goproxy.exe
```
