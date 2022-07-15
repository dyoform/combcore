module combcore

go 1.18

replace libcomb => ./libcomb

require (
	github.com/syndtr/goleveldb v1.0.0
	github.com/vharitonsky/iniflags v0.0.0-20180513140207-a33cd0b5f3de
	libcomb v0.0.0-00010101000000-000000000000
)

require (
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/klauspost/cpuid/v2 v2.0.4 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
)
