set GOOS=linux
set GOARCH=arm
rem set GOARM=7
go build
mkdir linux-arm
copy .\hub linux-arm\

set GOOS=linux
set GOARCH=amd64
go build
mkdir linux-amd64
copy .\hub linux-arm\

set GOOS=windows
set GOARCH=amd64
go build
