module github.com/MariusVanDerWijden/FuzzyVM

go 1.15

require (
	github.com/dvyukov/go-fuzz v0.0.0-20210103155950-6a8e9d1f2415 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.1 // indirect
	github.com/ethereum/go-ethereum v1.9.25
	github.com/google/godepq v0.0.0-20190501212251-2c635fd1e5fe // indirect
	github.com/hhatto/gocloc v0.3.3 // indirect
	github.com/holiman/goevmlab v0.0.0-20200925112252-8249743488ae
	github.com/korovkin/limiter v0.0.0-20190919045942-dac5a6b2a536
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.5.0
	github.com/stephens2424/writerset v1.0.2 // indirect
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/tools v0.0.0-20210114065538-d78b04bdf963 // indirect
	gopkg.in/urfave/cli.v1 v1.20.0
)

replace github.com/MariusVanDerWijden/FuzzyVM/filler => ./filler

replace github.com/ethereum/go-ethereum => /home/matematik/go/src/github.com/ethereum/go-ethereum

replace github.com/holiman/goevmlab => /home/matematik/ethereum/goevmlab
