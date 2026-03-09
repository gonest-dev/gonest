module github.com/gonest-dev/gonest/core/interceptors

go 1.25

toolchain go1.25.0

require github.com/gonest-dev/gonest/core/common v0.1.1

require github.com/gonest-dev/gonest/core/di v0.1.1 // indirect

replace (
	github.com/gonest-dev/gonest/core/common => ../common
	github.com/gonest-dev/gonest/core/di => ../di
)
