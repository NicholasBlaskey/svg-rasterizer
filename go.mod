module github.com/nicholasblaskey/svg-rasterizer

go 1.17

require (
	github.com/go-gl/mathgl v1.0.0
	github.com/nicholasblaskey/dat-gui-go-wasm v0.0.0-20210919220437-82602fcef295
	github.com/nicholasblaskey/webgl v0.0.0
	github.com/nicholasblaskey/webgl-utils v0.0.0
)

require golang.org/x/image v0.0.0-20190321063152-3fc05d484e9f // indirect

replace github.com/nicholasblaskey/webgl v0.0.0 => ../webgl

replace github.com/nicholasblaskey/webgl-utils v0.0.0 => ../webgl-utils

replace github.com/nicholasblaskey/dat-gui-go-wasm v0.0.0 => ../dat-gui-go-wasm
