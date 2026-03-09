module github.com/gonest-dev/gonest/modules/tester

go 1.25

toolchain go1.25.0

require (
	github.com/gonest-dev/gonest/core/common v0.0.0
	github.com/gonest-dev/gonest/core/di v0.1.1
)

replace (
	github.com/gonest-dev/gonest/core/common => ../../core/common
	github.com/gonest-dev/gonest/core/di => ../../core/di
)
