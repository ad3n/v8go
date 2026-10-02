module github.com/ad3n/v8go

go 1.26

require (
	github.com/ad3n/v8go/deps/android_amd64 v0.0.0
	github.com/ad3n/v8go/deps/android_arm64 v0.0.0
	github.com/ad3n/v8go/deps/darwin_amd64 v0.0.0
	github.com/ad3n/v8go/deps/darwin_arm64 v0.0.0
	github.com/ad3n/v8go/deps/linux_amd64 v0.0.0
	github.com/ad3n/v8go/deps/linux_arm64 v0.0.0
)

replace (
	github.com/ad3n/v8go/deps/android_amd64 => ./deps/android_amd64
	github.com/ad3n/v8go/deps/android_arm64 => ./deps/android_arm64
	github.com/ad3n/v8go/deps/darwin_amd64 => ./deps/darwin_amd64
	github.com/ad3n/v8go/deps/darwin_arm64 => ./deps/darwin_arm64
	github.com/ad3n/v8go/deps/linux_amd64 => ./deps/linux_amd64
	github.com/ad3n/v8go/deps/linux_arm64 => ./deps/linux_arm64
)
