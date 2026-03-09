module github.com/gonest-dev/gonest/core/pipes

go 1.25.0

require github.com/gonest-dev/gonest/core/common v0.1.1

require (
	github.com/gonest-dev/gonest/core/di v0.1.1 // indirect
	github.com/gonest-dev/gonest/packages/validator v0.1.1
)

replace (
	github.com/gonest-dev/gonest/core/common => ../common
	github.com/gonest-dev/gonest/core/di => ../di
	github.com/gonest-dev/gonest/packages/validator => ../../packages/validator
)
