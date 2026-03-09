module github.com/gonest-dev/gonest/packages/validator

go 1.25.0

require (
	github.com/gonest-dev/gonest/core/common v0.1.1
	github.com/gonest-dev/gonest/core/di v0.1.1 // indirect
	golang.org/x/exp v0.0.0-20260218203240-3dfff04db8fa
)

replace (
	github.com/gonest-dev/gonest/core/common => ../../core/common
	github.com/gonest-dev/gonest/core/di => ../../core/di
)
