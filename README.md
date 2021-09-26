# svg-rasterizer

An software rasterizer using Go and Wasm that is displayed with WebGL in a browser environment.

It is not spec compliant or even close and was done to dive into some basics of how rasterizers work.

It is based on the CMU assignment for [DrawSVG](https://github.com/cmu462/DrawSVG).

### Building

First build the webasm file (run in root of this directory)
```
GOOS=js GOARCH=wasm go build -o o.wasm rasterizer/rasterizer.go
```

Then start an http server that has support for hosting wasm files properly
```
go run main.go
```

Finally go to
```
http://127.0.0.1:8080/
```