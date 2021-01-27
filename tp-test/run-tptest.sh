#!/bin/sh

set -euo

export TPBIN='/home/liuzhuomin/tidb_workplace/go-randgen/tp-test/tp-test'
export SCENE='/home/liuzhuomin/tidb_workplace/go-randgen/tp-test/scenarios/clustered_index.yy'
export DSN='root:@tcp(127.0.0.1:4000)/test'

$TPBIN --store $DSN clear
$TPBIN --store $DSN init
$TPBIN --store $DSN gen --test 20
#$TPBIN --store $DSN run --dsn1 'root:@tcp(127.0.0.1:4000)/?time_zone=UTC' --dsn2 'root:@tcp(127.0.0.1:4001)/?time_zone=UTC'

$TPBIN --store $DSN clear
