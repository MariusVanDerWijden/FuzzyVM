module github.com/MariusVanDerWijden/FuzzyVM

go 1.15

require (
	github.com/ethereum/go-ethereum v1.10.14-0.20211214103450-fc01a7ce8e4f
	github.com/holiman/goevmlab v0.0.0-20211215113238-06157bc85f7d
	github.com/korovkin/limiter v0.0.0-20190919045942-dac5a6b2a536
	github.com/naoina/toml v0.1.2-0.20170918210437-9fafd6967416
	github.com/pkg/errors v0.9.1
	golang.org/x/crypto v0.0.0-20211209193657-4570a0811e8b
	gopkg.in/urfave/cli.v1 v1.20.0
)

//replace github.com/MariusVanDerWijden/FuzzyVM/filler => ./filler

//replace github.com/holiman/goevmlab => ./../goevmlab
