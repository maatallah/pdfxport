module pdfparser

go 1.20

require (
	github.com/ledongthuc/pdf v0.0.0-20250510234604-a6dfec7e9de4
	github.com/xuri/excelize/v2 v2.8.1
)

require (
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/xuri/efp v0.0.0-20231025114914-d1ff6096ae53 // indirect
	github.com/xuri/nfp v0.0.0-20230919160717-d98342af3f05 // indirect
	golang.org/x/crypto v0.19.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace (
	golang.org/x/crypto => golang.org/x/crypto v0.13.0
	golang.org/x/net => golang.org/x/net v0.15.0
	golang.org/x/text => golang.org/x/text v0.13.0
)

replace github.com/ledongthuc/pdf => ./pdf
