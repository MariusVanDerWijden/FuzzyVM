module github.com/MariusVanDerWijden/FuzzyVM

go 1.15

require (
	github.com/ethereum/go-ethereum v1.10.2
	github.com/holiman/goevmlab v0.0.0-20210406174504-acc14986d1a1
	github.com/korovkin/limiter v0.0.0-20190919045942-dac5a6b2a536
	github.com/naoina/toml v0.1.2-0.20170918210437-9fafd6967416
	github.com/pkg/errors v0.9.1
	gopkg.in/urfave/cli.v1 v1.20.0
)

replace github.com/MariusVanDerWijden/FuzzyVM/filler => ./filler
