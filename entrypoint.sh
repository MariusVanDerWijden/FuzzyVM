#!bin/bash

echo "Starting FuzzyVM"
./FuzzyVM run & 
echo "Sleep for a bit"
sleep 100
echo "Starting goevmlab"
./runtest --gethbatch=/gethvm --nethbatch=/nethtest --nimbus=/nimbvm --revme=/revme", "--erigonbatch=/erigon_vm --besubatch=/besu-vm --evmone=/evmone "out/*/*.json"