mkdir -p dist
if [[ `uname -s` == "Darwin" ]]; then
    echo "system is macos"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-w' -o dist/delay-queue-linux-64
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '-w' -o dist/delay-queue-win-64
    go build -ldflags '-w' -o dist/delay-queue-macos-64
else
    echo "system is linux"
    CGO_ENABLED=0 GOOS=macos GOARCH=amd64 go build -ldflags '-w' -o dist/delay-queue-linux-64
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '-w' -o dist/delay-queue-win-64
    go build -ldflags '-w' -o dist/delay-queue-macos-64
fi
upx -9 dist/delay-queue-linux-64
upx -9 dist/delay-queue-win-64
upx -9 dist/delay-queue-macos-64
