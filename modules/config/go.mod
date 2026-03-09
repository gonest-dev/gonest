module github.com/gonest-dev/gonest/modules/config

go 1.25

toolchain go1.25.0

require (
	github.com/gonest-dev/gonest/core/common v0.1.1
	github.com/gonest-dev/gonest/packages/env v0.1.1
)

require github.com/gonest-dev/gonest/core/di v0.1.1 // indirect

replace (
	github.com/gonest-dev/gonest/core/common => ../../core/common
	github.com/gonest-dev/gonest/core/di => ../../core/di
	github.com/gonest-dev/gonest/packages/env => ../../packages/env
	github.com/gonest-dev/gonest/packages/validator => ../../packages/validator
)
