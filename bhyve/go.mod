module bhyve

go 1.19

replace github.com/gcla/gowid => ../../gowid
replace tui => ../tui
replace host => ../host

require (
	github.com/gcla/gowid v1.4.1-0.20221101015339-ce29e21d2804
	github.com/gdamore/tcell v1.3.1-0.20200115030318-bff4943f9a29
	github.com/gdamore/tcell/v2 v2.5.0
	github.com/mattn/go-sqlite3 v1.14.12
	github.com/sirupsen/logrus v1.4.2
	github.com/quasilyte/gsignal v0.0.0-20231010082051-3c00e9ebb4e5
	tui v0.0.1
	host v0.0.1
)

require (
	github.com/creack/pty v1.1.15 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	golang.org/x/sys v0.0.0-20220318055525-2edf467146b5 // indirect
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf // indirect
	golang.org/x/text v0.3.7 // indirect
)
