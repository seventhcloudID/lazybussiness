package fontassets

import _ "embed"

//go:embed Inter-Regular.ttf
var interRegularTTF []byte

//go:embed Inter-SemiBold.ttf
var interSemiBoldTTF []byte

func Regular() []byte {
	return interRegularTTF
}

func SemiBold() []byte {
	return interSemiBoldTTF
}
